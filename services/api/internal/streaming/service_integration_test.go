//go:build integration

package streaming

import (
	"bytes"
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
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

func streamingTestStore(t *testing.T) *conversation.Store {
	t.Helper()
	dsn := os.Getenv("SYNTELLE_STREAMING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL fixture URL not configured")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("cannot connect to test PostgreSQL")
	}
	name := fmt.Sprintf("syntelle_streaming_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+quoted+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgresql" && u.Scheme != "postgres") {
		t.Fatal("invalid isolated PostgreSQL URL")
	}
	u.Path = "/" + name
	q := u.Query()
	q.Del("dbname")
	u.RawQuery = q.Encode()
	verify, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	var actual string
	err = verify.QueryRow(ctx, "SELECT current_database()").Scan(&actual)
	_ = verify.Close(ctx)
	if err != nil || actual != name {
		t.Fatal("refusing fixture mutation outside isolated database")
	}
	store, err := conversation.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Pool.Close)
	return store
}

type streamModelFixture struct{ calls atomic.Int32 }

func (m *streamModelFixture) Settings() any              { return map[string]bool{"offline_fixture": true} }
func (m *streamModelFixture) Identity() (string, string) { return "fixture-model", "offline-test/1" }
func (m *streamModelFixture) Complete(_ context.Context, _ string, _ []conversation.Message, delta func(string)) (conversation.ModelResult, error) {
	m.calls.Add(1)
	delta("A fixture reply.")
	return conversation.ModelResult{Text: "A fixture reply."}, nil
}

type streamVoiceFixture struct{ calls atomic.Int32 }

func (v *streamVoiceFixture) Prepare(context.Context, string, string, string, string) (any, error) {
	return nil, fmt.Errorf("whole-recording path must not run")
}
func (v *streamVoiceFixture) PrepareParts(_ context.Context, text, _, _, _ string, emit func(conversation.SpeechPart) error) (any, error) {
	v.calls.Add(1)
	err := emit(conversation.SpeechPart{Text: text, Index: 0, Final: true, Speech: map[string]any{"audio_base64": []byte("private-native-audio"), "duration_ms": 100}})
	return map[string]bool{"fixture": true}, err
}

func TestSessionOwnershipCommandsAndPrivateDelivery(t *testing.T) {
	store := streamingTestStore(t)
	model := &streamModelFixture{}
	voice := &streamVoiceFixture{}
	conversations := conversation.New(store, model, voice, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	resolve := func(_ http.ResponseWriter, r *http.Request, _ bool) (accounts.Principal, error) {
		id := r.Header.Get("X-Test-Account")
		if id == "" {
			id = "one"
		}
		return accounts.Principal{ID: "account:" + id, Kind: "account", AccountID: id, SessionID: "browser-" + id}, nil
	}
	var expired atomic.Bool
	validate := func(context.Context, accounts.Principal) error {
		if expired.Load() {
			return fmt.Errorf("fixture authorization expired")
		}
		return nil
	}
	service, err := Open(ctx, store.Pool, conversations, resolve, Config{WorkerToken: "fixture-worker-secret", MaximumSessions: 2, PackageHash: strings.Repeat("a", 64), ValidatePrincipal: validate})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler(http.NotFoundHandler()))
	defer server.Close()
	defer func() {
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := service.Shutdown(shutdown); err != nil {
			t.Error(err)
		}
		if err := conversations.Shutdown(shutdown); err != nil {
			t.Error(err)
		}
	}()
	worker, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/stream-worker/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer fixture-worker-secret"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer worker.CloseNow()
	workerReady := make(chan struct{})
	workerDone := make(chan error, 1)
	privateParts := atomic.Int32{}
	go func() {
		for {
			_, b, err := worker.Read(ctx)
			if err != nil {
				workerDone <- err
				return
			}
			var event Envelope
			if err = json.Unmarshal(b, &event); err != nil {
				workerDone <- err
				return
			}
			send := func(e Envelope) error {
				data, _ := json.Marshal(e)
				return worker.Write(ctx, websocket.MessageText, data)
			}
			switch event.Type {
			case "registered":
				close(workerReady)
			case "create":
				if err = send(envelope("ready", event.SessionID, nil)); err != nil {
					workerDone <- err
					return
				}
			case "conversation":
				var e conversation.Event
				if err = json.Unmarshal(event.Data, &e); err != nil {
					workerDone <- err
					return
				}
				if e.Type == "speech_part" {
					privateParts.Add(1)
					if err = send(envelope("playback", event.SessionID, map[string]string{"event": "playback_started", "turn_id": e.TurnID})); err != nil {
						workerDone <- err
						return
					}
				}
				if e.Type == "speech_complete" {
					if err = send(envelope("playback", event.SessionID, map[string]string{"event": "playback_ended", "turn_id": e.TurnID})); err != nil {
						workerDone <- err
						return
					}
				}
			}
		}
	}()
	select {
	case <-workerReady:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	request := func(method, path, account string, value any) (int, []byte) {
		t.Helper()
		var body bytes.Buffer
		if value != nil {
			_ = json.NewEncoder(&body).Encode(value)
		}
		r, err := http.NewRequestWithContext(ctx, method, server.URL+path, &body)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-Test-Account", account)
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var result bytes.Buffer
		_, _ = result.ReadFrom(response.Body)
		return response.StatusCode, result.Bytes()
	}
	status, b := request("POST", "/api/v1/streams", "one", map[string]any{"settings": accounts.Defaults()})
	if status != 201 {
		t.Fatalf("create %d %s", status, b)
	}
	var created SessionResponse
	if json.Unmarshal(b, &created) != nil {
		t.Fatal("invalid create response")
	}
	status, _ = request("POST", "/api/v1/streams", "one", map[string]any{"settings": accounts.Defaults()})
	if status != http.StatusTooManyRequests {
		t.Fatal("same visitor opened a second render session", status)
	}
	var activeStreams int
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_streaming.sessions WHERE closed_at IS NULL`).Scan(&activeStreams); err != nil || activeStreams != 1 {
		t.Fatal("rejected session reached persistence or renderer", err, activeStreams)
	}
	player, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/streams/"+created.ID+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer player.CloseNow()
	auth, _ := json.Marshal(envelope("authenticate", created.ID, map[string]string{"ticket": created.Ticket}))
	if err = player.Write(ctx, websocket.MessageText, auth); err != nil {
		t.Fatal(err)
	}
	readIdle := func(requireReply bool) {
		t.Helper()
		reply := !requireReply
		for {
			_, b, err := player.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(b, []byte("audio_base64")) || bytes.Contains(b, []byte("private-native-audio")) {
				t.Fatal("native audio leaked to player")
			}
			var event Envelope
			_ = json.Unmarshal(b, &event)
			if event.Type == "conversation" {
				var e conversation.Event
				_ = json.Unmarshal(event.Data, &e)
				if e.Type == "reply_complete" {
					reply = true
				}
				if e.Type == "error" {
					t.Fatal(e.Text)
				}
			}
			if event.Type == "state" {
				var state struct{ State string }
				_ = json.Unmarshal(event.Data, &state)
				if state.State == "idle" && reply {
					return
				}
			}
		}
	}
	readIdle(false)
	message := Command{ID: "message-fixture-0001", Type: "message", Text: "Tell me something."}
	status, b = request("POST", "/api/v1/streams/"+created.ID+"/commands", "one", message)
	if status != 202 {
		t.Fatalf("command %d %s", status, b)
	}
	readIdle(true)
	status, b = request("POST", "/api/v1/streams/"+created.ID+"/commands", "one", message)
	var duplicate commandResult
	_ = json.Unmarshal(b, &duplicate)
	if status != 202 || !duplicate.Duplicate || model.calls.Load() != 1 || voice.calls.Load() != 1 {
		t.Fatalf("duplicate caused work: %d %s", status, b)
	}
	literal := Command{ID: "literal-fixture-0001", Type: "speak", Text: "Speak exactly this supplied text."}
	status, b = request("POST", "/api/v1/streams/"+created.ID+"/commands", "one", literal)
	if status != 202 {
		t.Fatalf("literal %d %s", status, b)
	}
	readIdle(true)
	if model.calls.Load() != 1 || voice.calls.Load() != 2 || privateParts.Load() != 2 {
		t.Fatal("literal invoked model or native delivery was duplicated")
	}
	status, _ = request("GET", "/api/v1/streams/"+created.ID, "two", nil)
	if status != 404 {
		t.Fatal("another account read a stream")
	}
	service.Revoke("session", "browser-one")
	var state string
	if err = store.Pool.QueryRow(ctx, `SELECT state FROM syntelle_streaming.sessions WHERE id=$1`, created.ID).Scan(&state); err != nil || state != "closed" {
		t.Fatal("revocation did not persist closure", err, state)
	}
	var commands int
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_streaming.commands WHERE session_id=$1`, created.ID).Scan(&commands); err != nil || commands != 2 {
		t.Fatal("command persistence/idempotency failed", err, commands)
	}
	status, b = request("POST", "/api/v1/streams", "one", map[string]any{"settings": accounts.Defaults()})
	if status != http.StatusCreated {
		t.Fatal("closing the previous session did not release the visitor's slot", status)
	}
	if json.Unmarshal(b, &created) != nil {
		t.Fatal("invalid replacement session")
	}
	replacement, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/streams/"+created.ID+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.CloseNow()
	if err = wsjson.Write(ctx, replacement, envelope("authenticate", created.ID, map[string]string{"ticket": created.Ticket})); err != nil {
		t.Fatal(err)
	}
	var connected Envelope
	if err = wsjson.Read(ctx, replacement, &connected); err != nil || connected.Type != "session" {
		t.Fatal("replacement did not authenticate", err)
	}
	expired.Store(true)
	time.Sleep(1100 * time.Millisecond)
	// Exhaust the request allowance: throttling must not skip authorization.
	for range 61 {
		service.admission.allow("principal-request", "account:one", 600, time.Now())
	}
	if err = wsjson.Write(ctx, replacement, envelope("signal", created.ID, nil)); err != nil {
		t.Fatal(err)
	}
	for err == nil {
		err = wsjson.Read(ctx, replacement, &connected)
	}
	if ctx.Err() != nil {
		t.Fatal("non-heartbeat traffic kept expired authorization alive")
	}
	// Socket closure precedes its bounded persistence write.
	for deadline := time.Now().Add(time.Second); ; {
		err = store.Pool.QueryRow(ctx, `SELECT state FROM syntelle_streaming.sessions WHERE id=$1`, created.ID).Scan(&state)
		if err == nil && state == "closed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired authorization did not release the session", err, state)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
