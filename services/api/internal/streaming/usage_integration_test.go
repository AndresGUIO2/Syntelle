//go:build integration

package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

func usageFixture(t *testing.T) *recordStore {
	t.Helper()
	store, err := openStore(t.Context(), streamingTestStore(t).Pool)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func usageSession(id, principal string) *streamSession {
	return &streamSession{id: id, cid: "conversation_" + id, principal: accounts.Principal{ID: principal, Kind: "account"}, settings: accounts.Defaults()}
}

func TestDemoTimeConcurrentReservationsAreSharedByIdentity(t *testing.T) {
	store := usageFixture(t)
	var accepted, rejected, unexpected atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			deadline, err := store.allocate(t.Context(), usageSession(fmt.Sprint(i), "account:shared"), strings.Repeat("a", 64), false)
			if err == nil {
				accepted.Add(1)
				if remaining := time.Until(deadline); remaining > sessionTimeLimit || remaining < sessionTimeLimit-time.Second {
					unexpected.Add(1)
				}
			} else if errors.Is(err, errDemoTime) {
				rejected.Add(1)
			} else {
				unexpected.Add(1)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 3 || rejected.Load() != 5 || unexpected.Load() != 0 {
		t.Fatal("non-atomic demo reservations", accepted.Load(), rejected.Load(), unexpected.Load())
	}
	var allocated int64
	if err := store.pool.QueryRow(t.Context(), `SELECT sum(used_ms+lease_ms) FROM syntelle_streaming.sessions WHERE principal_id=$1`, "account:shared").Scan(&allocated); err != nil || allocated != visitorTimeLimit.Milliseconds() {
		t.Fatal("visitor allowance exceeded", err, allocated)
	}
	if _, err := store.allocate(t.Context(), usageSession("unrelated", "account:other"), strings.Repeat("a", 64), false); err != nil {
		t.Fatal("unrelated visitor blocked", err)
	}
}

func TestDemoTimePauseRefundsUnusedTimeAndResumeKeepsSessionLimit(t *testing.T) {
	store := usageFixture(t)
	session := usageSession("paused_fixture", "account:one")
	if _, err := store.allocate(t.Context(), session, strings.Repeat("a", 64), false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(t.Context(), `UPDATE syntelle_streaming.sessions SET lease_started_at=clock_timestamp()-interval '40 seconds' WHERE id=$1`, session.id); err != nil {
		t.Fatal(err)
	}
	if err := store.state(t.Context(), session.id, "paused", inactivityReason, true); err != nil {
		t.Fatal(err)
	}
	var used, reserved int64
	if err := store.pool.QueryRow(t.Context(), `SELECT used_ms,lease_ms FROM syntelle_streaming.sessions WHERE id=$1`, session.id).Scan(&used, &reserved); err != nil || used < 40000 || used > 41000 || reserved != 0 {
		t.Fatal("unused time not refunded", err, used, reserved)
	}
	deadline, err := store.allocate(t.Context(), session, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if remaining := time.Until(deadline); remaining > 260*time.Second || remaining < 258*time.Second {
		t.Fatal("pause reset five-minute session", remaining)
	}
	if err = store.state(t.Context(), session.id, "paused", inactivityReason, true); err != nil {
		t.Fatal(err)
	}
	if _, err = store.pool.Exec(t.Context(), `UPDATE syntelle_streaming.sessions SET used_ms=300000 WHERE id=$1`, session.id); err != nil {
		t.Fatal(err)
	}
	if _, err = store.allocate(t.Context(), session, "", true); !errors.Is(err, errSessionTime) {
		t.Fatal("exhausted session resumed", err)
	}
}

func TestDemoTimeExhaustionRejectsBeforeConversationOrWorker(t *testing.T) {
	store := usageFixture(t)
	for i := range 3 {
		if _, err := store.allocate(t.Context(), usageSession(fmt.Sprint(i), "account:one"), strings.Repeat("a", 64), false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.pool.Exec(t.Context(), `UPDATE syntelle_streaming.sessions SET used_ms=lease_ms,lease_ms=0,lease_started_at=NULL,state='ended',closed_at=now()`); err != nil {
		t.Fatal(err)
	}
	worker := &peer{ctx: t.Context(), out: make(chan Envelope, 1)}
	service := &Service{ctx: t.Context(), store: store, admission: testAdmission(t, AdmissionConfig{}), worker: worker, sessions: map[string]*streamSession{}, config: Config{MaximumSessions: 2}, conversation: &conversation.Service{}, resolve: func(http.ResponseWriter, *http.Request, bool) (accounts.Principal, error) {
		return accounts.Principal{ID: "account:one"}, nil
	}}
	request := httptest.NewRequest("POST", "http://localhost/api/v1/streams", strings.NewReader("{}"))
	response := httptest.NewRecorder()
	service.Handler(http.NotFoundHandler()).ServeHTTP(response, request)
	if response.Code != 429 || len(worker.out) != 0 || len(service.sessions) != 0 {
		t.Fatal("exhausted visitor reached downstream work", response.Code)
	}
	// A final partial allowance is bounded by the visitor's remaining time.
	if _, err := store.pool.Exec(t.Context(), `UPDATE syntelle_streaming.sessions SET used_ms=299000 WHERE id='0'`); err != nil {
		t.Fatal(err)
	}
	deadline, err := store.allocate(t.Context(), usageSession("last_second", "account:one"), strings.Repeat("a", 64), false)
	if err != nil || time.Until(deadline) > time.Second {
		t.Fatal("partial allowance became a new five-minute grant", err)
	}
}

func TestDemoTimeRestartDoesNotResetOutstandingAllowance(t *testing.T) {
	store := usageFixture(t)
	session := usageSession("crash_fixture", "account:one")
	if _, err := store.allocate(t.Context(), session, strings.Repeat("a", 64), false); err != nil {
		t.Fatal(err)
	}
	if _, err := openStore(context.Background(), store.pool); err != nil {
		t.Fatal(err)
	}
	var used, reserved int64
	if err := store.pool.QueryRow(t.Context(), `SELECT used_ms,lease_ms FROM syntelle_streaming.sessions WHERE id=$1`, session.id).Scan(&used, &reserved); err != nil || used != 300000 || reserved != 0 {
		t.Fatal("restart reset reserved time", err, used, reserved)
	}
}

func TestLocalTimeExceptionIgnoresCapsAndPreservesProductionEvidence(t *testing.T) {
	store := usageFixture(t)
	for i := range 3 {
		if _, err := store.allocate(t.Context(), usageSession(fmt.Sprint(i), "account:one"), strings.Repeat("a", 64), false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.pool.Exec(t.Context(), `UPDATE syntelle_streaming.sessions SET used_ms=lease_ms,lease_ms=0,lease_started_at=NULL,state='ended',closed_at=now()`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	model, voice := &streamModelFixture{}, &streamVoiceFixture{}
	conversations := conversation.New(&conversation.Store{Pool: store.pool}, model, voice, nil)
	defer func() {
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := conversations.Shutdown(shutdown); err != nil {
			t.Error(err)
		}
	}()
	worker := &peer{ctx: ctx, out: make(chan Envelope, 32)}
	service := &Service{ctx: ctx, store: store, admission: testAdmission(t, AdmissionConfig{}), worker: worker, sessions: map[string]*streamSession{}, config: Config{MaximumSessions: 2, DisableTimeLimits: true}, conversation: conversations, resolve: func(http.ResponseWriter, *http.Request, bool) (accounts.Principal, error) {
		return accounts.Principal{ID: "account:one"}, nil
	}}
	post := func(resume string) (int, SessionResponse) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"resume_from": resume})
		request := httptest.NewRequest("POST", "http://localhost/api/v1/streams", bytes.NewReader(body))
		response := httptest.NewRecorder()
		service.Handler(http.NotFoundHandler()).ServeHTTP(response, request)
		var result SessionResponse
		if response.Code == 201 && json.Unmarshal(response.Body.Bytes(), &result) != nil {
			t.Fatal("invalid response")
		}
		return response.Code, result
	}
	status, result := post("")
	if status != 201 || result.TimeLimitsEnabled || result.RemainingSeconds != 0 {
		t.Fatal("local mode used exhausted production quota", status, result)
	}
	service.mu.Lock()
	session := service.sessions[result.ID]
	future := time.Now().Add(6 * time.Minute)
	session.lastMessage, session.lastSeen = future, future
	noDeadline := session.deadline.IsZero()
	service.mu.Unlock()
	if !noDeadline {
		t.Fatal("local session retained a five-minute deadline")
	}
	service.expireAt(future)
	service.mu.Lock()
	active := service.sessions[result.ID] == session
	service.mu.Unlock()
	if !active {
		t.Fatal("five-minute cap closed local development session")
	}
	service.closeSession(result.ID, inactivityReason)
	status, resumed := post(result.ID)
	if status != 201 || resumed.ID != result.ID || resumed.TimeLimitsEnabled {
		t.Fatal("local pause could not resume", status)
	}
	service.closeSession(result.ID, "fixture complete")
	var allocated int64
	if err := store.pool.QueryRow(t.Context(), `SELECT sum(used_ms+lease_ms) FROM syntelle_streaming.sessions WHERE principal_id='account:one'`).Scan(&allocated); err != nil || allocated != 900000 {
		t.Fatal("local activity reset or charged production evidence", err, allocated)
	}
	service.config.DisableTimeLimits = false
	if status, _ = post(""); status != 429 {
		t.Fatal("production caps did not remain enforceable", status)
	}
	if model.calls.Load() != 0 || voice.calls.Load() != 0 {
		t.Fatal("policy fixture invoked a provider")
	}
}
