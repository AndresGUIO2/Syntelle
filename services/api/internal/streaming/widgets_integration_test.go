//go:build integration

package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

func TestPublicWidgetAdmissionScopeReplayAndRevocation(t *testing.T) {
	db := streamingTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	accountService, err := accounts.Open(ctx, db.Pool, accounts.Config{Origin: "http://127.0.0.1:5173", Secret: strings.Repeat("fixture-secret-", 4), PackageHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	settings := accounts.Defaults()
	settings.Character = "char_" + strings.Repeat("d", 32)
	settings.Instructions = "PRIVATE INTEGRATION INSTRUCTIONS"
	config := accounts.IntegrationConfig{PublicationID: "publication_fixture", AllowedOrigins: []string{"https://customer.example"}, Capabilities: []string{"message", "speak", "interrupt", "suspend"}, Settings: settings, DailyLimitSeconds: 3600}
	raw, _ := json.Marshal(config)
	settingsRaw, _ := json.Marshal(settings)
	_, err = db.Pool.Exec(ctx, `INSERT INTO syntelle_accounts.accounts(id,email) VALUES('owner_fixture','owner-widget@example.com'); INSERT INTO syntelle_accounts.workspaces(id,name,created_by) VALUES('workspace_fixture','Widget team','owner_fixture')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Pool.Exec(ctx, `INSERT INTO syntelle_accounts.publications(id,workspace_id,name,character_id,package_hash,settings,created_by) VALUES('publication_fixture','workspace_fixture','Guide',$3,$1,$2,'owner_fixture')`, strings.Repeat("c", 64), settingsRaw, settings.Character)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Pool.Exec(ctx, `INSERT INTO syntelle_accounts.integrations(id,workspace_id,widget_id,name,draft,published,published_revision,enabled,created_by) VALUES('integration_fixture','workspace_fixture','widget_fixture','Website',$1,$1,1,true,'owner_fixture')`, raw)
	if err != nil {
		t.Fatal(err)
	}
	model := &streamModelFixture{}
	voice := &streamVoiceFixture{}
	conversations := conversation.New(db, model, voice, nil)
	var resolved, verified, reserved, started, released, creates atomic.Int32
	var deny atomic.Bool
	resolve := func(w http.ResponseWriter, r *http.Request, guest bool) (accounts.Principal, error) {
		resolved.Add(1)
		return accountService.Resolve(w, r, guest)
	}
	service, err := Open(ctx, db.Pool, conversations, resolve, Config{PackageHash: strings.Repeat("a", 64), WorkerToken: "fixture-worker-secret", MaximumSessions: 3, DisableTimeLimits: true, ValidatePrincipal: accountService.ValidatePrincipal, ResolveIntegration: accountService.ResolveIntegration, ResolveWidget: accountService.ResolveWidget, TurnstileSiteKey: "fixture-site-key", VerifyWidgetChallenge: func(_ context.Context, token, _, nonce string) error {
		verified.Add(1)
		if token != "fixture-valid" || nonce == "" {
			return errors.New("invalid challenge")
		}
		return nil
	}, ReserveIntegrationUsage: func(_ context.Context, wid, iid, sid string) (time.Time, error) {
		reserved.Add(1)
		if deny.Load() {
			return time.Time{}, errors.New("quota unavailable")
		}
		if wid != "workspace_fixture" || iid != "integration_fixture" || sid == "" {
			t.Error("wrong billing binding")
		}
		return time.Now().Add(time.Minute), nil
	}, StartIntegrationUsage: func(context.Context, string) error { started.Add(1); return nil }, RenewIntegrationUsage: func(context.Context, string) (time.Time, error) { return time.Now().Add(time.Minute), nil }, ReleaseIntegrationUsage: func(context.Context, string) error { released.Add(1); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	accountService.OnRevoked = service.Revoke
	server := httptest.NewServer(service.Handler(http.NotFoundHandler()))
	defer server.Close()
	defer func() {
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = service.Shutdown(shutdown)
		_ = conversations.Shutdown(shutdown)
	}()
	worker, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/stream-worker/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer fixture-worker-secret"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer worker.CloseNow()
	registered := make(chan struct{})
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for {
			_, body, err := worker.Read(ctx)
			if err != nil {
				return
			}
			var event Envelope
			if json.Unmarshal(body, &event) != nil {
				return
			}
			if event.Type == "registered" {
				close(registered)
			}
			if event.Type == "create" {
				creates.Add(1)
				var creation struct {
					PackageHash string            `json:"package_hash"`
					Settings    accounts.Settings `json:"settings"`
				}
				if json.Unmarshal(event.Data, &creation) != nil || creation.PackageHash != strings.Repeat("c", 64) || creation.Settings.Character != settings.Character || creation.Settings.Instructions != settings.Instructions {
					t.Error("published custom package was not bound to native create", string(event.Data))
					return
				}
				data, _ := json.Marshal(envelope("ready", event.SessionID, map[string]string{"package_hash": creation.PackageHash}))
				if worker.Write(ctx, websocket.MessageText, data) != nil {
					return
				}
			}
		}
	}()
	select {
	case <-registered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	request := func(method, path string, input any, capability string) (int, []byte) {
		t.Helper()
		body, _ := json.Marshal(input)
		r, _ := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(body))
		r.Header.Set("Origin", "http://127.0.0.1:5173")
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: "syntelle_session", Value: "not-a-widget-credential"})
		if capability != "" {
			r.Header.Set("Authorization", "Bearer "+capability)
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		out, _ := io.ReadAll(response.Body)
		if len(response.Cookies()) != 0 {
			t.Fatal("widget created account cookie")
		}
		return response.StatusCode, out
	}
	prefix := "/api/v1/widgets/widget_fixture"
	status, body := request("GET", prefix, nil, "")
	if status != 200 || bytes.Contains(body, []byte("PRIVATE")) {
		t.Fatal(status, string(body))
	}
	status, _ = request("POST", prefix+"/bootstrap", map[string]any{"parent_origin": "https://evil.example"}, "")
	if status != 403 {
		t.Fatal("unlisted parent", status)
	}
	bootstrap := func() map[string]any {
		t.Helper()
		status, body := request("POST", prefix+"/bootstrap", map[string]any{"parent_origin": "https://customer.example"}, "")
		if status != 201 {
			t.Fatal(status, string(body))
		}
		var b map[string]any
		_ = json.Unmarshal(body, &b)
		return map[string]any{"bootstrap_id": b["bootstrap_id"], "nonce": b["nonce"], "parent_origin": "https://customer.example", "turnstile_token": "fixture-valid", "request_id": "fixture_request"}
	}
	input := bootstrap()
	input["turnstile_token"] = "bad"
	status, _ = request("POST", prefix+"/sessions", input, "")
	if status != 403 || reserved.Load() != 0 || creates.Load() != 0 {
		t.Fatal("challenge failure allocated work", status)
	}
	input["turnstile_token"] = "fixture-valid"
	status, body = request("POST", prefix+"/sessions", input, "")
	if status != 201 {
		t.Fatal(status, string(body))
	}
	var session SessionResponse
	if json.Unmarshal(body, &session) != nil || session.Capability == "" || session.Settings.Instructions != "" || !session.TimeLimitsEnabled {
		t.Fatal("unsafe public response", string(body))
	}
	if resolved.Load() != 0 {
		t.Fatal("widget relied on account cookies")
	}
	status, _ = request("POST", prefix+"/sessions", input, "")
	if status != 409 || reserved.Load() != 1 {
		t.Fatal("replayed challenge allocated another session", status, reserved.Load())
	}
	deadline := time.Now().Add(time.Second)
	for started.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if started.Load() != 1 {
		t.Fatal("ready did not start bounded billing")
	}
	player, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/streams/"+session.ID+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer player.CloseNow()
	auth, _ := json.Marshal(envelope("authenticate", session.ID, map[string]string{"ticket": session.Ticket}))
	if err = player.Write(ctx, websocket.MessageText, auth); err != nil {
		t.Fatal(err)
	}
	_, body, err = player.Read(ctx)
	if err != nil || bytes.Contains(body, []byte("PRIVATE")) || bytes.Contains(body, []byte(session.Capability)) {
		t.Fatal("websocket leaked private configuration", err, string(body))
	}
	status, _ = request("GET", "/api/v1/streams/"+session.ID, nil, session.Capability)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = request("GET", "/api/v1/streams/stream_other", nil, session.Capability)
	if status != 404 {
		t.Fatal("capability crossed session", status)
	}
	status, _ = request("POST", "/api/v1/streams", map[string]any{}, session.Capability)
	if status != 403 {
		t.Fatal("capability created another stream", status)
	}
	status, _ = request("POST", "/api/v1/streams/"+session.ID+"/commands", map[string]any{"id": "fixture_command", "type": "configure", "settings": settings}, session.Capability)
	if status != 409 || model.calls.Load() != 0 || voice.calls.Load() != 0 {
		t.Fatal("visitor changed configuration", status)
	}
	deny.Store(true)
	input = bootstrap()
	status, body = request("POST", prefix+"/sessions", input, "")
	if status != 429 || creates.Load() != 1 {
		t.Fatal("paid quota bypassed with development limits disabled", status, string(body), creates.Load())
	}
	service.Revoke("integration", "integration_fixture")
	status, _ = request("GET", "/api/v1/streams/"+session.ID, nil, session.Capability)
	if status != 401 {
		t.Fatal("revoked capability survived", status)
	}
	if released.Load() != 2 {
		t.Fatal("usage was not released on denial and revocation", released.Load())
	}
	var count int
	if err = db.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_streaming.saved_conversations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("widget leaked into personal history", count, err)
	}
	_ = worker.CloseNow()
	select {
	case <-workerDone:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
