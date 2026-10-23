package conversation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type burstModelFixture struct{ calls atomic.Int32 }

func (m *burstModelFixture) Settings() any { return map[string]string{"fixture": "offline burst"} }
func (m *burstModelFixture) Complete(_ context.Context, _ string, _ []Message, delta func(string)) (ModelResult, error) {
	m.calls.Add(1)
	var text strings.Builder
	for i := range 200 {
		fragment := string(rune('a' + i%26))
		text.WriteString(fragment)
		delta(fragment)
	}
	return ModelResult{Text: text.String()}, nil
}

type socketFixture struct {
	t   *testing.T
	ctx context.Context
	c   *websocket.Conn
}

func openSocketFixture(t *testing.T, service *Service, cid string) socketFixture {
	t.Helper()
	server := httptest.NewServer(service.Handler(http.NotFoundHandler()))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	t.Cleanup(cancel)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/conversation/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://127.0.0.1:5173"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.CloseNow()
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := service.Shutdown(shutdown); err != nil {
			t.Error(err)
		}
	})
	f := socketFixture{t, ctx, c}
	f.send(Event{Type: "hello", ConversationID: cid, TurnID: "fixture-hello-01", Character: "syntelle", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural"})
	if event := f.read(); event.Type != "ready" {
		t.Fatal(event)
	}
	return f
}

func (f socketFixture) send(event Event) {
	f.t.Helper()
	b, err := json.Marshal(event)
	if err != nil {
		f.t.Fatal(err)
	}
	if err = f.c.Write(f.ctx, websocket.MessageText, b); err != nil {
		f.t.Fatal(err)
	}
}

func (f socketFixture) read() Event {
	f.t.Helper()
	_, b, err := f.c.Read(f.ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	var event Event
	if err = json.Unmarshal(b, &event); err != nil {
		f.t.Fatal(err)
	}
	return event
}

type inputModelFixture struct {
	modelFixture
	input chan string
}

func (m *inputModelFixture) Complete(ctx context.Context, instructions string, messages []Message, delta func(string)) (ModelResult, error) {
	m.input <- messages[len(messages)-1].Content
	return m.modelFixture.Complete(ctx, instructions, messages, delta)
}

func TestLongUserMessageReachesModelWithoutTruncation(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 1)}
	voice := &speechFixture{}
	f := openSocketFixture(t, New(store, model, voice, nil), "long-input-session-01")
	text := strings.Repeat("Explain this in detail. Gracias. 日本語 también.\n", 100)
	f.send(Event{Type: "submit", ConversationID: "long-input-session-01", TurnID: "long-input-turn-01", Text: text})
	if event := f.read(); event.Type != "accepted" || event.Text != text {
		t.Fatal("Long user message was rejected or changed", event.Type)
	}
	for {
		event := f.read()
		if event.Type == "error" {
			t.Fatal(event.Text)
		}
		if event.Type == "prepared_speech" {
			break
		}
	}
	if received := <-model.input; received != text || model.calls.Load() != 1 || voice.calls.Load() != 1 {
		t.Fatal("Long user message was truncated or repeated")
	}
}

func TestBurstCompletesWithoutDisconnectOrDuplicateProviders(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	store := isolatedStore(t)
	model, voice := &burstModelFixture{}, &speechFixture{}
	service := New(store, model, voice, nil)
	cid, id := "burst-session-01", "burst-turn-0001"
	f := openSocketFixture(t, service, cid)
	f.send(Event{Type: "submit", ConversationID: cid, TurnID: id, Text: "Reply briefly."})
	var received strings.Builder
	fragments := 0
	for {
		event := f.read()
		switch event.Type {
		case "reply_delta":
			received.WriteString(event.Text)
			fragments++
		case "error":
			t.Fatal(event.Text)
		case "prepared_speech":
			if fragments != 200 || received.String() != event.Text || model.calls.Load() != 1 || voice.calls.Load() != 1 {
				t.Fatalf("lost/reordered text or repeated work: fragments=%d model=%d speech=%d", fragments, model.calls.Load(), voice.calls.Load())
			}
			return
		}
	}
}

func TestInterruptionPersistenceWarningFollowsCancellation(t *testing.T) {
	store := isolatedStore(t)
	model, voice := &modelFixture{}, &speechFixture{}
	service := New(store, model, voice, nil)
	cid, id := "archive-session-01", "archive-turn-0001"
	f := openSocketFixture(t, service, cid)
	f.send(Event{Type: "submit", ConversationID: cid, TurnID: id, Text: "Hello."})
	for f.read().Type != "prepared_speech" {
	}
	// Fault injection affects only the identity-verified disposable database.
	if _, err := store.Pool.Exec(f.ctx, "ALTER TABLE syntelle_conversation.turns ADD CONSTRAINT fixture_reject_interruption CHECK (status <> 'interrupted')"); err != nil {
		t.Fatal(err)
	}
	f.send(Event{Type: "cancel", ConversationID: cid, TurnID: id, Reason: "owner stopped playback"})
	for _, kind := range []string{"cancelled", "persistence_warning"} {
		event := f.read()
		if event.Type != kind || event.ConversationID != cid || event.TurnID != id {
			t.Fatalf("expected %s for stopped turn, got %+v", kind, event)
		}
	}
	turns, err := store.Turns(f.ctx, cid, 1)
	if err != nil || len(turns) != 1 || turns[0].Status != "ready" {
		t.Fatal("failed write was incorrectly presented as persisted", turns, err)
	}
	if model.calls.Load() != 1 || voice.calls.Load() != 1 {
		t.Fatal("interruption repeated provider work")
	}
}
