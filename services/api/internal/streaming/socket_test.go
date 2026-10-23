package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"syntelle/api/internal/conversation"
)

func TestPlayerSocketPreservesQueuedSignalsAcrossReads(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	session := &streamSession{id: "fixture_signal_session", state: "idle"}
	ticket, err := issueTicket(session)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the real worker queue until the player has sent both signals and a
	// heartbeat. The handler must retain each signal's original bytes meanwhile.
	worker := &peer{ctx: ctx, out: make(chan Envelope, 8)}
	service := &Service{ctx: ctx, sessions: map[string]*streamSession{session.id: session}, worker: worker, conversation: &conversation.Service{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service.playerSocket(w, r, session.id)
	}))
	defer server.Close()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"http://127.0.0.1:5173"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = connection.CloseNow()
		cancel()
		service.wg.Wait()
	}()
	write := func(event Envelope) {
		t.Helper()
		if err := wsjson.Write(ctx, connection, event); err != nil {
			t.Fatal(err)
		}
	}
	write(envelope("authenticate", session.id, map[string]string{"ticket": ticket}))
	for range 2 {
		var event Envelope
		if err := wsjson.Read(ctx, connection, &event); err != nil {
			t.Fatal(err)
		}
	}
	want := []Envelope{
		envelope("signal", session.id, map[string]any{"description": map[string]string{"type": "answer", "sdp": strings.Repeat("fixture/sdp;", 100)}}),
		envelope("signal", session.id, map[string]any{"candidate": map[string]string{"candidate": "fixture-candidate"}}),
	}
	for _, event := range want {
		write(event)
	}
	write(envelope("heartbeat", session.id, nil))
	var heartbeat Envelope
	if err := wsjson.Read(ctx, connection, &heartbeat); err != nil || heartbeat.Type != "heartbeat" {
		t.Fatalf("player did not acknowledge the reads: %v", err)
	}
	for index, expected := range want {
		select {
		case got := <-worker.out:
			if !bytes.Equal(got.Data, expected.Data) {
				t.Fatalf("queued signal %d changed after the following player read", index)
			}
			if _, err := json.Marshal(got); err != nil {
				t.Fatalf("queued signal %d is no longer marshalable: %v", index, err)
			}
		case <-ctx.Done():
			t.Fatal("player did not queue its signals")
		}
	}
}
