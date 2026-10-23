//go:build integration

package streaming

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

func TestAccountHistoryOwnershipContinuationAndPurge(t *testing.T) {
	store := streamingTestStore(t)
	ctx := context.Background()
	_, err := accounts.Open(ctx, store.Pool, accounts.Config{Secret: strings.Repeat("fixture-", 5), Origin: "http://127.0.0.1:5173", PackageHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_accounts.accounts(id,email) VALUES('owner-one','one@example.invalid'),('owner-two','two@example.invalid')`)
	if err != nil {
		t.Fatal(err)
	}
	conversations := conversation.New(store, nil, nil, nil)
	s, err := Open(ctx, store.Pool, conversations, func(_ http.ResponseWriter, r *http.Request, _ bool) (accounts.Principal, error) {
		owner := r.Header.Get("X-Test-Owner")
		return accounts.Principal{Kind: "account", AccountID: owner, ID: "account:" + owner, SessionID: "browser-" + owner}, nil
	}, Config{PackageHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Shutdown(stop)
	})
	cid := "convo_history_fixture"
	if _, err = store.Session(ctx, cid, "syntelle", "en-US", "Fixture instructions"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.BeginTurn(ctx, cid, "turn_history_fixture", "First question"); err != nil {
		t.Fatal(err)
	}
	if err = store.FinishTurn(ctx, "turn_history_fixture", "ready", "First reply", "First reply", ""); err != nil {
		t.Fatal(err)
	}
	settings, _ := json.Marshal(accounts.Defaults())
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_streaming.sessions(id,conversation_id,principal_id,principal_kind,settings,package_hash,state,closed_at) VALUES('stream_history_fixture',$1,'account:owner-one','account',$2,$3,'closed',now())`, cid, settings, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.saveTurn(ctx, cid, "owner-one"); err != nil {
		t.Fatal(err)
	}
	var operationID int64
	err = store.Pool.QueryRow(ctx, `INSERT INTO syntelle_conversation.operations(conversation_id,turn_id,kind,request_key,status,request,result,audio) VALUES($1,'turn_history_fixture','speech','history-audio-fixture','received','{"text":"private request"}','{"text":"private result"}',decode('aabb','hex')) RETURNING id`, cid).Scan(&operationID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_conversation.recognition_results(operation_id,result_id,result) VALUES($1,'recognized-fixture','{"text":"private recognition"}')`, operationID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_streaming.commands(session_id,id,turn_id,kind,request,status) VALUES('stream_history_fixture','command-history','turn_history_fixture','message','{"text":"private command"}','complete'); INSERT INTO syntelle_streaming.events(session_id,kind,data) VALUES('stream_history_fixture','fixture','{"text":"private event"}'); UPDATE syntelle_streaming.sessions SET used_ms=1000 WHERE id='stream_history_fixture'`)
	if err != nil {
		t.Fatal(err)
	}
	request := func(owner, path, method string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, nil)
		r.Header.Set("X-Test-Owner", owner)
		r.Header.Set("Origin", "http://127.0.0.1:5173")
		w := httptest.NewRecorder()
		s.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
		return w
	}
	if got := request("owner-one", "/api/v1/conversations", http.MethodGet); got.Code != 200 || !strings.Contains(got.Body.String(), "First question") {
		t.Fatalf("owner history: %d %s", got.Code, got.Body.String())
	}
	if got := request("owner-two", "/api/v1/conversations/"+cid, http.MethodGet); got.Code != 404 {
		t.Fatalf("cross-account history: %d", got.Code)
	}
	if got := request("owner-one", "/api/v1/conversations/"+cid, http.MethodGet); got.Code != 200 || !strings.Contains(got.Body.String(), "First reply") {
		t.Fatalf("transcript: %d %s", got.Code, got.Body.String())
	}
	if _, err = s.store.continuation(ctx, cid, "owner-one"); err != nil {
		t.Fatal("owner cannot continue", err)
	}
	if _, err = s.store.continuation(ctx, cid, "owner-two"); err == nil {
		t.Fatal("another account continued")
	}
	if got := request("owner-one", "/api/v1/conversations/"+cid, http.MethodDelete); got.Code != 204 {
		t.Fatalf("delete: %d %s", got.Code, got.Body.String())
	}
	var turns, ops int
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_conversation.turns WHERE conversation_id=$1`, cid).Scan(&turns); err != nil || turns != 0 {
		t.Fatal("turns retained", err, turns)
	}
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_conversation.operations WHERE conversation_id=$1`, cid).Scan(&ops); err != nil || ops != 0 {
		t.Fatal("operations retained", err, ops)
	}
	var recognition, commands, events, used int
	var persistedSettings string
	err = store.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM syntelle_conversation.recognition_results WHERE operation_id=$1),(SELECT count(*) FROM syntelle_streaming.commands WHERE session_id='stream_history_fixture'),(SELECT count(*) FROM syntelle_streaming.events WHERE session_id='stream_history_fixture'),used_ms,settings::text FROM syntelle_streaming.sessions WHERE id='stream_history_fixture'`, operationID).Scan(&recognition, &commands, &events, &used, &persistedSettings)
	if err != nil || recognition != 0 || commands != 0 || events != 0 || used != 1000 || persistedSettings != "{}" {
		t.Fatal("private content or usage purge mismatch", err, recognition, commands, events, used, persistedSettings)
	}
	if got := request("owner-one", "/api/v1/conversations/"+cid, http.MethodGet); got.Code != 404 {
		t.Fatalf("deleted transcript visible: %d", got.Code)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_accounts.sessions(id,token_hash,expires_at,account_id,guest_session_id) VALUES('guest-history','hash-guest',now()+interval '24 hours',NULL,NULL),('browser-owner-one','hash-owner',now()+interval '7 days','owner-one','guest-history')`)
	if err != nil {
		t.Fatal(err)
	}
	guestCID := "convo_guest_fixture"
	if _, err = store.Session(ctx, guestCID, "syntelle", "en-US", "Guest instructions"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.BeginTurn(ctx, guestCID, "turn_guest_fixture", "Guest question"); err != nil {
		t.Fatal(err)
	}
	if err = store.FinishTurn(ctx, "turn_guest_fixture", "ready", "Guest reply", "Guest reply", ""); err != nil {
		t.Fatal(err)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_streaming.sessions(id,conversation_id,principal_id,principal_kind,browser_session_id,settings,package_hash,state,closed_at) VALUES('stream_guest_fixture',$1,'guest:guest-history','guest','guest-history',$2,$3,'closed',now())`, guestCID, settings, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if got := request("owner-one", "/api/v1/conversations/claimable", http.MethodGet); got.Code != 200 || !strings.Contains(got.Body.String(), "Guest question") {
		t.Fatalf("guest offer: %d %s", got.Code, got.Body.String())
	}
	if got := request("owner-one", "/api/v1/conversations/"+guestCID+"/claim", http.MethodPost); got.Code != 200 {
		t.Fatalf("guest claim: %d %s", got.Code, got.Body.String())
	}
	if got := request("owner-two", "/api/v1/conversations/"+guestCID, http.MethodGet); got.Code != 404 {
		t.Fatalf("claimed cross-account read: %d", got.Code)
	}
	_, err = store.Pool.Exec(ctx, `UPDATE syntelle_streaming.saved_conversations SET last_activity_at=now()-interval '91 days' WHERE conversation_id=$1`, guestCID)
	if err != nil {
		t.Fatal(err)
	}
	if got := request("owner-one", "/api/v1/conversations/"+guestCID, http.MethodGet); got.Code != 404 {
		t.Fatalf("expired chat remained readable: %d", got.Code)
	}
	s.purgeOldHistory(ctx)
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_conversation.turns WHERE conversation_id=$1`, guestCID).Scan(&turns); err != nil || turns != 0 {
		t.Fatal("expired content retained", err, turns)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_accounts.sessions(id,token_hash,expires_at) VALUES('expired-guest-history','hash-expired',now()-interval '1 minute')`)
	if err != nil {
		t.Fatal(err)
	}
	const unclaimedCID = "convo_unclaimed_fixture"
	if _, err = store.Session(ctx, unclaimedCID, "syntelle", "en-US", "Guest only"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.BeginTurn(ctx, unclaimedCID, "turn_unclaimed_fixture", "Unclaimed text"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO syntelle_streaming.sessions(id,conversation_id,principal_id,principal_kind,settings,package_hash,state,closed_at) VALUES('stream_unclaimed_fixture',$1,'guest:expired-guest-history','guest',$2,$3,'closed',now())`, unclaimedCID, settings, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	s.purgeOldHistory(ctx)
	if err = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_conversation.turns WHERE conversation_id=$1`, unclaimedCID).Scan(&turns); err != nil || turns != 0 {
		t.Fatal("expired guest content retained", err, turns)
	}
}
