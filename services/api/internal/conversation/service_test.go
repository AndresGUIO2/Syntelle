package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

func isolatedStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("SYNTELLE_CONVERSATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Set local test database URL for isolated integration checks")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal("Cannot connect to test database")
	}
	name := fmt.Sprintf("syntelle_conversation_test_%d", time.Now().UnixNano())
	if !strings.HasPrefix(name, "syntelle_conversation_test_") {
		t.Fatal("Invalid test name")
	}
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+quoted); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+quoted+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	u, e := url.Parse(dsn)
	if e != nil || u.Scheme != "postgresql" && u.Scheme != "postgres" {
		t.Fatal("Use a PostgreSQL test URL")
	}
	u.Path = "/" + name
	q := u.Query()
	q.Del("dbname")
	u.RawQuery = q.Encode()
	// Verify identity BEFORE migrations or any fixture mutation.
	verify, e := pgx.Connect(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	var actual string
	e = verify.QueryRow(ctx, "SELECT current_database()").Scan(&actual)
	verify.Close(ctx)
	if e != nil || actual != name {
		t.Fatal("Refusing mutation outside isolated database")
	}
	s, e := Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Pool.Close)
	return s
}

type modelFixture struct {
	calls   atomic.Int32
	blocked bool
}

func (m *modelFixture) Settings() any { return map[string]string{"fixture": "not a provider call"} }
func (m *modelFixture) Complete(ctx context.Context, _ string, _ []Message, delta func(string)) (ModelResult, error) {
	m.calls.Add(1)
	delta("Hello.")
	if m.blocked {
		<-ctx.Done()
		return ModelResult{Text: "Hello."}, ctx.Err()
	}
	return ModelResult{Text: "Hello.", Usage: json.RawMessage(`{"output_tokens":2}`)}, nil
}

type speechFixture struct{ calls atomic.Int32 }

func (s *speechFixture) Prepare(ctx context.Context, _, _, _, _ string) (any, error) {
	s.calls.Add(1)
	return map[string]any{"fixture": true}, nil
}

type recognitionFixture struct{}

func (recognitionFixture) Run(ctx context.Context, _ string, pcm <-chan []byte, emit func(RecognitionEvent)) error {
	select {
	case <-pcm:
	case <-ctx.Done():
		return ctx.Err()
	}
	emit(RecognitionEvent{Type: "partial", ID: "native-result-0001", Text: "Hello"})
	event := RecognitionEvent{Type: "final", ID: "native-result-0001", Text: "Hello from microphone."}
	emit(event)
	emit(event)
	<-ctx.Done()
	return ctx.Err()
}
func TestRecognitionFinalDedupAndPreparedReply(t *testing.T) {
	s := isolatedStore(t)
	model := &modelFixture{}
	voice := &speechFixture{}
	service := New(s, model, voice, recognitionFixture{})
	server := httptest.NewServer(service.Handler(http.NotFoundHandler()))
	defer server.Close()
	ctx, stop := context.WithTimeout(context.Background(), 8*time.Second)
	defer stop()
	c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/conversation/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://127.0.0.1:5173"}}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	send := func(v Event) {
		b, _ := json.Marshal(v)
		if e = c.Write(ctx, websocket.MessageText, b); e != nil {
			t.Fatal(e)
		}
	}
	read := func(kind string) Event {
		for {
			_, b, e := c.Read(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var v Event
			_ = json.Unmarshal(b, &v)
			if v.Type == kind {
				return v
			}
		}
	}
	cid := "session-speech-01"
	send(Event{Type: "hello", ConversationID: cid, TurnID: "hello-speech-001", Character: "cafe", Language: "es-CO", Voice: "en-US-Andrew:DragonHDLatestNeural"})
	read("ready")
	send(Event{Type: "mic_start", ConversationID: cid, TurnID: "mic-speech-0001"})
	read("listening")
	send(Event{Type: "audio", ConversationID: cid, TurnID: "mic-speech-0001", Audio: make([]byte, 3200)})
	reply := read("prepared_speech")
	if reply.TurnID != "native-result-0001" || reply.Text != "Hello." {
		t.Fatal(reply)
	}
	send(Event{Type: "playback_ended", ConversationID: cid, TurnID: reply.TurnID})
	shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if e = service.Shutdown(shutdown); e != nil {
		t.Fatal(e)
	}
	if model.calls.Load() != 1 || voice.calls.Load() != 1 {
		t.Fatal("Recognition duplicate reached a provider")
	}
	var count int
	_ = s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM syntelle_conversation.recognition_results`).Scan(&count)
	if count != 1 {
		t.Fatal("Recognition result not deduplicated in PostgreSQL", count)
	}
}

func TestPostgresJSONBHistoryIdempotencyAndRecovery(t *testing.T) {
	s := isolatedStore(t)
	ctx := context.Background()
	cid := "session-test-001"
	if _, e := s.Session(ctx, cid, "syntelle", "en-US", "A quote: \"hello\" and ñ"); e != nil {
		t.Fatal(e)
	}
	fresh, e := s.BeginTurn(ctx, cid, "turn-test-0001", "A question?")
	if e != nil || !fresh {
		t.Fatal(e)
	}
	if fresh, e = s.BeginTurn(ctx, cid, "turn-test-0001", "A question?"); e != nil || fresh {
		t.Fatal("Duplicate turn accepted")
	}
	if _, e = s.BeginTurn(ctx, cid, "turn-test-0001", "different"); e == nil {
		t.Fatal("Changed idempotent input accepted")
	}
	if e = s.FinishTurn(ctx, "turn-test-0001", "ready", "Original ñ", "Spoken ñ", ""); e != nil {
		t.Fatal(e)
	}
	restored, e := s.Session(ctx, cid, "syntelle", "en-US", "Updated instructions")
	if e != nil || len(restored) != 1 || restored[0].Reply != "Original ñ" {
		t.Fatal(restored, e)
	}
	op, e := s.Reserve(ctx, "dragon", "request-0001", "binding", cid, "turn-test-0001", map[string]any{"text": "hi"}, true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Reserve(ctx, "dragon", "request-0001", "binding", cid, "turn-test-0001", nil, true); e == nil {
		t.Fatal("Active operation retried")
	}
	if _, e = s.Reserve(ctx, "dragon", "different-request-key", "binding", cid, "turn-test-0001", nil, true); e == nil {
		t.Fatal("Identical in-flight synthesis was duplicated")
	}
	if e = s.Finish(ctx, op.ID, "received", map[string]any{"native_visemes": []int{0, 2}}, map[string]int{"characters": 2}, nil, []byte{1, 2, 3}); e != nil {
		t.Fatal(e)
	}
	cached, e := s.Reserve(ctx, "dragon", "request-0002", "binding", cid, "turn-test-0001", nil, true)
	if e != nil || string(cached.Audio) != "\x01\x02\x03" {
		t.Fatal("BYTEA cache did not restore", e)
	}
	uncertain, e := s.Reserve(ctx, "luna", "request-crash", "", cid, "turn-test-0001", map[string]string{"text": "saved before call"}, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	var status string
	_ = s.Pool.QueryRow(ctx, `SELECT status FROM syntelle_conversation.operations WHERE id=$1`, uncertain.ID).Scan(&status)
	if status != "uncertain" {
		t.Fatal(status)
	}
}
func TestWebSocketTurnDedupCancellationAndShutdown(t *testing.T) {
	s := isolatedStore(t)
	model := &modelFixture{blocked: true}
	voice := &speechFixture{}
	service := New(s, model, voice, nil)
	server := httptest.NewServer(service.Handler(http.NotFoundHandler()))
	defer server.Close()
	ctx, stop := context.WithTimeout(context.Background(), 8*time.Second)
	defer stop()
	c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/conversation/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://127.0.0.1:5173"}}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	send := func(v Event) {
		b, _ := json.Marshal(v)
		if e = c.Write(ctx, websocket.MessageText, b); e != nil {
			t.Fatal(e)
		}
	}
	read := func(kind string) Event {
		for {
			_, b, e := c.Read(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var v Event
			_ = json.Unmarshal(b, &v)
			if v.Type == kind {
				return v
			}
		}
	}
	cid, id := "session-socket-01", "turn-socket-0001"
	send(Event{Type: "hello", ConversationID: cid, TurnID: "hello-socket-01", Character: "syntelle", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural"})
	read("ready")
	event := Event{Type: "submit", ConversationID: cid, TurnID: id, Text: "Hello"}
	send(event)
	read("reply_delta")
	send(event)
	read("duplicate")
	// A cancellation for an obsolete turn must not cancel the current request.
	send(Event{Type: "cancel", ConversationID: cid, TurnID: "obsolete-turn-01"})
	send(Event{Type: "cancel", ConversationID: cid, TurnID: id, Reason: "test interruption"})
	read("cancelled")
	shutdown, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	if e = service.Shutdown(shutdown); e != nil {
		t.Fatal(e)
	}
	if model.calls.Load() != 1 || voice.calls.Load() != 0 {
		t.Fatal("Duplicate or cancelled work reached provider")
	}
	turns, e := s.Turns(context.Background(), cid, 0)
	if e != nil || len(turns) != 1 || turns[0].Status != "interrupted" {
		t.Fatal(turns, e)
	}
}
func TestOwnerOrigins(t *testing.T) {
	handler := httpapi.LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		host, origin string
		want         int
	}{{"127.0.0.1:8080", "http://127.0.0.1:5173", 204}, {"127.0.0.1:8080", "https://evil.example", 403}, {"evil.example", "http://127.0.0.1:5173", 403}, {"127.0.0.1:8080", "", 403}} {
		r := httptest.NewRequest("POST", "http://"+tc.host+"/api/v1/speech/synthesize", nil)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(tc, w.Code)
		}
	}
}
