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

func TestNavigationSuspendPreservesConversation(t *testing.T) {
	for _, kind := range []string{"guest", "account"} {
		for _, unmetered := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/metered", true: "/unmetered"}[unmetered], func(t *testing.T) {
				f := newNavigationFixture(t, kind, unmetered)
				status, created := f.create("one", "", accounts.Defaults())
				if status != http.StatusCreated {
					t.Fatal("create failed", status)
				}
				f.service.mu.Lock()
				session := f.service.sessions[created.ID]
				cid := session.cid
				f.service.mu.Unlock()
				// A complete prior turn proves context retention without invoking providers.
				if _, err := f.store.Session(f.ctx, cid, "syntelle", "en-US", "Fixture context"); err != nil {
					t.Fatal(err)
				}
				if _, err := f.store.BeginTurn(f.ctx, cid, "turn_before_navigation", "Remember the blue notebook."); err != nil {
					t.Fatal(err)
				}
				if err := f.store.FinishTurn(f.ctx, "turn_before_navigation", "ready", "The notebook is blue.", "The notebook is blue.", ""); err != nil {
					t.Fatal(err)
				}
				player := f.player(created)
				// Suspension must also work before the renderer reports ready.
				if err := wsjson.Write(f.ctx, player, Envelope{Type: "command", SessionID: created.ID, CommandID: "navigation-suspend-001", Data: json.RawMessage(`{"type":"suspend"}`)}); err != nil {
					t.Fatal(err)
				}
				f.notice(player, "paused")
				var state, reason string
				if err := f.store.Pool.QueryRow(f.ctx, `SELECT state,reason FROM syntelle_streaming.sessions WHERE id=$1 AND closed_at IS NOT NULL`, created.ID).Scan(&state, &reason); err != nil || state != "paused" || reason != "navigation" {
					t.Fatal("acknowledged before committed pause", err, state, reason)
				}
				select {
				case <-session.channel.Done():
				case <-f.ctx.Done():
					t.Fatal("conversation work did not stop")
				}
				f.service.mu.Lock()
				active := f.service.sessions[created.ID] != nil
				f.service.mu.Unlock()
				if active {
					t.Fatal("suspended stream retained its slot")
				}
				stops := 0
				for len(f.worker.out) > 0 {
					if event := <-f.worker.out; event.Type == "stop" && event.SessionID == created.ID {
						stops++
					}
				}
				if stops != 1 {
					t.Fatal("suspension did not release renderer exactly once", stops)
				}
				if err := f.service.store.matchesPausedCharacter(f.ctx, created.ID, kind+":one", "char_"+strings.Repeat("1", 32), strings.Repeat("a", 64)); err == nil {
					t.Fatal("suspended stream accepted a different character")
				}
				if err := f.service.store.matchesPausedCharacter(f.ctx, created.ID, kind+":one", "syntelle", strings.Repeat("b", 64)); err == nil {
					t.Fatal("suspended stream accepted a replaced character package")
				}
				if status, _ = f.create("two", created.ID, accounts.Defaults()); status != 404 {
					t.Fatal("another principal resumed the stream", status)
				}
				spanish := accounts.Defaults()
				spanish.Language = "es-CO"
				if status, _ = f.create("one", created.ID, spanish); status != 409 {
					t.Fatal("resume changed language", status)
				}
				status, resumed := f.create("one", created.ID, accounts.Defaults())
				if status != 201 || resumed.ID != created.ID {
					t.Fatal("resume failed", status, resumed.ID)
				}
				f.service.mu.Lock()
				restored := f.service.sessions[created.ID]
				same := restored.cid == cid
				f.service.mu.Unlock()
				turns, err := f.store.Turns(f.ctx, cid, 10)
				if err != nil || !same || len(turns) != 1 {
					t.Fatal("conversation context changed", err, same, len(turns))
				}
				if _, err = f.service.command(f.ctx, session, Command{ID: "navigation-suspend-001", Type: "suspend"}); err == nil {
					t.Fatal("old session command affected the resumed session")
				}
				if f.model.calls.Load() != 0 || f.voice.calls.Load() != 0 {
					t.Fatal("navigation invoked providers")
				}
			})
		}
	}
}

func TestNavigationSuspendPersistenceFailureDoesNotAcknowledgePause(t *testing.T) {
	f := newNavigationFixture(t, "account", true)
	status, created := f.create("one", "", accounts.Defaults())
	if status != 201 {
		t.Fatal(status)
	}
	_, err := f.store.Pool.Exec(f.ctx, `CREATE FUNCTION syntelle_streaming.reject_navigation_pause() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.reason='navigation' THEN RAISE EXCEPTION 'fixture pause persistence failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_navigation_pause BEFORE UPDATE ON syntelle_streaming.sessions FOR EACH ROW EXECUTE FUNCTION syntelle_streaming.reject_navigation_pause()`)
	if err != nil {
		t.Fatal(err)
	}
	player := f.player(created)
	if err = wsjson.Write(f.ctx, player, Envelope{Type: "command", SessionID: created.ID, CommandID: "navigation-suspend-fail-001", Data: json.RawMessage(`{"type":"suspend"}`)}); err != nil {
		t.Fatal(err)
	}
	event := f.notice(player, "error")
	var failure struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(event.Data, &failure) != nil || failure.Code != "suspension_failed" {
		t.Fatal("missing actionable suspension failure", string(event.Data))
	}
	var next Envelope
	if err = wsjson.Read(f.ctx, player, &next); err == nil {
		t.Fatal("failure was followed by a successful acknowledgement", next.Type)
	}
	if status, _ = f.create("one", created.ID, accounts.Defaults()); status != 404 {
		t.Fatal("uncommitted pause resumed", status)
	}
	f.service.mu.Lock()
	active := f.service.sessions[created.ID] != nil
	f.service.mu.Unlock()
	if active {
		t.Fatal("failed suspension retained live work")
	}
}

type navigationFixture struct {
	t       *testing.T
	ctx     context.Context
	store   *conversation.Store
	service *Service
	server  *httptest.Server
	model   *streamModelFixture
	voice   *streamVoiceFixture
	worker  *peer
}

func newNavigationFixture(t *testing.T, kind string, unmetered bool) *navigationFixture {
	t.Helper()
	store := streamingTestStore(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	model, voice := &streamModelFixture{}, &streamVoiceFixture{}
	conversations := conversation.New(store, model, voice, nil)
	resolve := func(_ http.ResponseWriter, r *http.Request, _ bool) (accounts.Principal, error) {
		owner := r.Header.Get("X-Test-Owner")
		if owner == "" {
			owner = "one"
		}
		return accounts.Principal{ID: kind + ":" + owner, Kind: kind, SessionID: "browser-" + owner}, nil
	}
	service, err := Open(ctx, store.Pool, conversations, resolve, Config{MaximumSessions: 2, PackageHash: strings.Repeat("a", 64), DisableTimeLimits: unmetered})
	if err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	worker := &peer{ctx: ctx, out: make(chan Envelope, 256), cancel: func() {}}
	service.worker = worker
	service.mu.Unlock()
	server := httptest.NewServer(service.Handler(http.NotFoundHandler()))
	f := &navigationFixture{t: t, ctx: ctx, store: store, service: service, server: server, model: model, voice: voice, worker: worker}
	t.Cleanup(func() {
		service.mu.Lock()
		service.worker = nil
		service.mu.Unlock()
		stop, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		if err := service.Shutdown(stop); err != nil {
			t.Error(err)
		}
		if err := conversations.Shutdown(stop); err != nil {
			t.Error(err)
		}
		server.Close()
		cancel()
	})
	return f
}

func (f *navigationFixture) create(owner, resume string, settings accounts.Settings) (int, SessionResponse) {
	f.t.Helper()
	body, _ := json.Marshal(map[string]any{"settings": settings, "resume_from": resume})
	req, _ := http.NewRequestWithContext(f.ctx, http.MethodPost, f.server.URL+"/api/v1/streams", bytes.NewReader(body))
	req.Header.Set("X-Test-Owner", owner)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer response.Body.Close()
	var result SessionResponse
	if response.StatusCode == 201 {
		if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
			f.t.Fatal(err)
		}
	} else {
		_, _ = io.Copy(io.Discard, response.Body)
	}
	return response.StatusCode, result
}

func (f *navigationFixture) player(created SessionResponse) *websocket.Conn {
	f.t.Helper()
	player, _, err := websocket.Dial(f.ctx, "ws"+strings.TrimPrefix(f.server.URL, "http")+"/api/v1/streams/"+created.ID+"/ws", nil)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { _ = player.CloseNow() })
	if err = wsjson.Write(f.ctx, player, envelope("authenticate", created.ID, map[string]string{"ticket": created.Ticket})); err != nil {
		f.t.Fatal(err)
	}
	f.notice(player, "session")
	return player
}

func (f *navigationFixture) notice(player *websocket.Conn, expected string) Envelope {
	f.t.Helper()
	for {
		var event Envelope
		if err := wsjson.Read(f.ctx, player, &event); err != nil {
			f.t.Fatal("missing "+expected, err)
		}
		if event.Type == expected {
			return event
		}
		if event.Type == "paused" || event.Type == "error" {
			f.t.Fatal("unexpected terminal event", event.Type, string(event.Data))
		}
	}
}
