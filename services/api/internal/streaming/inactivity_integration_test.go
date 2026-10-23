//go:build integration

package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

func TestInactivityPauseReleasesAndExplicitlyResumesConversation(t *testing.T) {
	store := streamingTestStore(t)
	model, voice := &streamModelFixture{}, &streamVoiceFixture{}
	conversations := conversation.New(store, model, voice, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	resolve := func(_ http.ResponseWriter, r *http.Request, _ bool) (accounts.Principal, error) {
		id := r.Header.Get("X-Test-Account")
		if id == "" {
			id = "one"
		}
		return accounts.Principal{ID: "account:" + id, Kind: "account", SessionID: "browser-" + id}, nil
	}
	service, err := Open(ctx, store.Pool, conversations, resolve, Config{MaximumSessions: 2, PackageHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	worker := &peer{ctx: ctx, out: make(chan Envelope, 256), cancel: cancel}
	service.mu.Lock()
	service.worker = worker
	service.mu.Unlock()
	server := httptest.NewServer(service.Handler(http.NotFoundHandler()))
	defer server.Close()
	defer func() {
		service.mu.Lock()
		service.worker = nil // fixture peer has no network connection
		service.mu.Unlock()
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := service.Shutdown(shutdown); err != nil {
			t.Error(err)
		}
		if err := conversations.Shutdown(shutdown); err != nil {
			t.Error(err)
		}
	}()
	post := func(account, resume string) (int, SessionResponse) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"settings": accounts.Defaults(), "resume_from": resume})
		r, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/v1/streams", bytes.NewReader(body))
		r.Header.Set("X-Test-Account", account)
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var result SessionResponse
		if response.StatusCode == 201 {
			if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
		} else {
			_, _ = io.Copy(io.Discard, response.Body)
		}
		return response.StatusCode, result
	}
	status, created := post("one", "")
	if status != 201 {
		t.Fatal("create failed", status)
	}
	service.mu.Lock()
	session := service.sessions[created.ID]
	cid := session.cid
	service.mu.Unlock()
	player, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/streams/"+created.ID+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer player.CloseNow()
	if err = wsjson.Write(ctx, player, envelope("authenticate", created.ID, map[string]string{"ticket": created.Ticket})); err != nil {
		t.Fatal(err)
	}
	var event Envelope
	if err = wsjson.Read(ctx, player, &event); err != nil || event.Type != "session" {
		t.Fatal("player did not authenticate", err)
	}
	service.mu.Lock()
	session.lastSeen = session.created.Add(2 * time.Minute) // keep transport healthy
	service.mu.Unlock()
	service.expireAt(session.created.Add(2*time.Minute - time.Nanosecond))
	service.mu.Lock()
	active := service.sessions[created.ID] == session
	service.mu.Unlock()
	if !active {
		t.Fatal("paused before initial two minutes")
	}
	service.expireAt(session.created.Add(2 * time.Minute))
	for event.Type != "paused" {
		if err = wsjson.Read(ctx, player, &event); err != nil {
			t.Fatal("pause notice missing", err)
		}
	}
	select {
	case <-session.channel.Done():
	case <-ctx.Done():
		t.Fatal("pause did not cancel conversation work")
	}
	var state, reason string
	if err = store.Pool.QueryRow(ctx, `SELECT state,reason FROM syntelle_streaming.sessions WHERE id=$1 AND closed_at IS NOT NULL`, created.ID).Scan(&state, &reason); err != nil || state != "paused" || reason != inactivityReason {
		t.Fatal("pause was not persisted", err, state, reason)
	}
	if model.calls.Load() != 0 || voice.calls.Load() != 0 {
		t.Fatal("idle pause invoked a provider")
	}
	if status, _ = post("two", created.ID); status != 404 {
		t.Fatal("another visitor resumed private history", status)
	}
	status, resumed := post("one", created.ID)
	if status != 201 || resumed.ID != created.ID {
		t.Fatal("explicit resume failed", status)
	}
	service.mu.Lock()
	session = service.sessions[created.ID]
	session.state = "idle"
	sameConversation := session.cid == cid && session.lastMessage.IsZero()
	service.mu.Unlock()
	if !sameConversation {
		t.Fatal("empty conversation did not resume with the initial policy")
	}
	command := Command{ID: "idle-message-fixture-001", Type: "message", Text: "Keep this conversation."}
	if _, err = service.command(ctx, session, command); err != nil {
		t.Fatal(err)
	}
	// The accepted event establishes that the persisted user turn is ready.
	for {
		select {
		case event = <-worker.out:
			if event.Type != "conversation" {
				continue
			}
			var message conversation.Event
			if json.Unmarshal(event.Data, &message) != nil || message.Type != "accepted" {
				continue
			}
		case <-ctx.Done():
			t.Fatal("message was not accepted")
		}
		break
	}
	service.mu.Lock()
	last := session.lastMessage
	session.lastSeen = last.Add(2 * time.Minute) // keep transport healthy
	service.mu.Unlock()
	if last.IsZero() {
		t.Fatal("submitted message did not extend inactivity")
	}
	service.expireAt(last.Add(2*time.Minute - time.Nanosecond))
	service.mu.Lock()
	active = service.sessions[created.ID] == session
	service.mu.Unlock()
	if !active {
		t.Fatal("active conversation paused before two minutes")
	}
	service.expireAt(last.Add(2 * time.Minute))
	status, resumed = post("one", created.ID)
	if status != 201 || resumed.ID != created.ID {
		t.Fatal("message-bearing conversation did not resume", status)
	}
	service.mu.Lock()
	restored := service.sessions[created.ID]
	preserved := restored.cid == cid && restored.lastMessage.Equal(restored.created)
	service.mu.Unlock()
	if !preserved {
		t.Fatal("resume lost history or reverted to the short timeout")
	}
	var sessions, turns int
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_streaming.sessions WHERE conversation_id=$1`, cid).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatal("resume duplicated a session", err, sessions)
	}
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_conversation.turns WHERE conversation_id=$1`, cid).Scan(&turns); err != nil || turns != 1 {
		t.Fatal("resume lost or replayed a turn", err, turns)
	}
	stops := 0
	for len(worker.out) > 0 {
		if (<-worker.out).Type == "stop" {
			stops++
		}
	}
	// One stop was consumed while waiting for acceptance; the later pause must
	// independently send its own worker stop and release the visitor slot.
	if stops != 1 {
		t.Fatal("two-minute pause did not stop renderer exactly once", stops)
	}
	service.mu.Lock()
	limit := restored.deadline
	restored.lastMessage, restored.lastSeen = limit, limit
	service.mu.Unlock()
	service.expireAt(limit.Add(-time.Nanosecond))
	service.mu.Lock()
	active = service.sessions[created.ID] == restored
	service.mu.Unlock()
	if !active {
		t.Fatal("five-minute ceiling fired early")
	}
	service.expireAt(limit)
	if err = store.Pool.QueryRow(ctx, `SELECT state,reason FROM syntelle_streaming.sessions WHERE id=$1`, created.ID).Scan(&state, &reason); err != nil || state != "ended" || reason != sessionLimitReason {
		t.Fatal("messages bypassed the fixed session ceiling", err, state, reason)
	}
	if status, _ = post("one", created.ID); status != 404 {
		t.Fatal("exhausted session resumed as an inactivity pause", status)
	}
}
