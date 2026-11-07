//go:build integration

package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"syntelle/api/internal/accounts"
)

type billingFixture struct {
	t                                              *testing.T
	pool                                           *pgxpool.Pool
	s                                              *Service
	handler                                        http.Handler
	wid, integration, account, token, subscription string
	start, end                                     time.Time
	mu                                             sync.Mutex
	requests                                       []string
	transport                                      func(*http.Request) (*http.Response, error)
}

func newBillingFixture(t *testing.T) *billingFixture {
	t.Helper()
	dsn := os.Getenv("SYNTELLE_BILLING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated billing database not configured")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasPrefix(cfg.ConnConfig.Database, "syntelle_billing_test_") {
		t.Fatal("refusing non-fixture database")
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal("fixture database unavailable")
	}
	t.Cleanup(pool.Close)
	a, err := accounts.Open(context.Background(), pool, accounts.Config{Origin: "http://127.0.0.1:5173", Secret: strings.Repeat("fixture-only-", 4)})
	if err != nil {
		t.Fatal(err)
	}
	f := &billingFixture{t: t, pool: pool, wid: "w_" + randomID(), integration: "i_" + randomID(), account: "a_" + randomID(), token: randomID(), subscription: "sub_" + randomID()}
	var now time.Time
	f.mustQuery(`SELECT clock_timestamp()`).Scan(&now)
	f.start = now.Add(-24 * time.Hour).Truncate(time.Second)
	f.end = now.Add(29 * 24 * time.Hour).Truncate(time.Second)
	f.exec(`INSERT INTO syntelle_accounts.accounts(id,email) VALUES($1,$2)`, f.account, f.account+"@fixture.invalid")
	f.exec(`INSERT INTO syntelle_accounts.sessions(id,token_hash,account_id,expires_at) VALUES($1,$2,$3,clock_timestamp()+interval '1 day')`, randomID(), digest([]byte(f.token)), f.account)
	f.exec(`INSERT INTO syntelle_accounts.workspaces(id,name,created_by) VALUES($1,'Billing fixture',$2)`, f.wid, f.account)
	f.exec(`INSERT INTO syntelle_accounts.memberships(workspace_id,account_id,role) VALUES($1,$2,'owner')`, f.wid, f.account)
	f.exec(`INSERT INTO syntelle_accounts.integrations(id,workspace_id,widget_id,name,draft,published,enabled,created_by) VALUES($1,$2,$3,'Fixture','{}','{}',true,$4)`, f.integration, f.wid, "widget_"+randomID(), f.account)
	c := fixtureConfig()
	c.AuthorizeWorkspace = a.AuthorizeWorkspaceBilling
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		tr := f.transport
		f.mu.Unlock()
		if r.URL.Host != "sandbox-api.paddle.com" {
			return nil, errors.New("unexpected provider host")
		}
		if tr != nil {
			return tr(r)
		}
		if strings.HasPrefix(r.URL.Path, "/prices/") {
			price := strings.TrimPrefix(r.URL.Path, "/prices/")
			amount := "1000"
			cycle := `{"interval":"month","frequency":1}`
			if price == "pri_team" {
				amount = "2000"
			}
			if price == "pri_topup" {
				amount = "500"
				cycle = "null"
			}
			return response(200, fmt.Sprintf(`{"data":{"id":%q,"status":"active","unit_price":{"amount":%q,"currency_code":"USD"},"billing_cycle":%s}}`, price, amount, cycle)), nil
		}
		if r.Method == http.MethodPost && r.URL.Path == "/transactions" {
			return response(200, `{"data":{"id":"txn_`+randomID()+`"}}`), nil
		}
		if strings.HasSuffix(r.URL.Path, "/update-payment-method-transaction") {
			return response(200, `{"data":{"id":"txn_payment_method","subscription_id":"`+f.subscription+`"}}`), nil
		}
		if r.Method == http.MethodPatch {
			return response(200, `{"data":{"id":"`+f.subscription+`"}}`), nil
		}
		if strings.HasSuffix(r.URL.Path, "/cancel") {
			return response(200, fmt.Sprintf(`{"data":{"id":%q,"scheduled_change":{"action":"cancel","effective_at":%q}}}`, f.subscription, f.end.Format(time.RFC3339))), nil
		}
		return nil, errors.New("unexpected fixture provider request")
	})}
	f.s = New(pool, c)
	f.handler = f.s.Handler(http.NotFoundHandler())
	return f
}

type scanner interface{ Scan(...any) error }

func (f *billingFixture) mustQuery(sql string, args ...any) scanner {
	return f.pool.QueryRow(context.Background(), sql, args...)
}
func (f *billingFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatal(err)
	}
}
func (f *billingFixture) request(method, path string, body any, key string) *httptest.ResponseRecorder {
	f.t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://127.0.0.1:5173/api/v1/workspaces/"+f.wid+path, strings.NewReader(string(raw)))
	r.AddCookie(&http.Cookie{Name: "syntelle_session", Value: f.token})
	r.Header.Set("Origin", "http://127.0.0.1:5173")
	r.Header.Set("X-Syntelle-Request", "1")
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func (f *billingFixture) action(kind, target string) (string, string) {
	f.t.Helper()
	input := map[string]string{}
	if kind == "topup" {
		input["topup"] = target
	} else if target != "" {
		input["plan"] = target
	}
	key := randomID()
	w := f.request("POST", "/billing/"+kind, input, key)
	if w.Code != 200 {
		f.t.Fatalf("%s: %d %s", kind, w.Code, w.Body.String())
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	var id string
	if err := f.mustQuery(`SELECT id FROM syntelle_billing.actions WHERE workspace_id=$1 AND idempotency_key=$2`, f.wid, key).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	txn, _ := out["transaction_id"].(string)
	return id, txn
}
func (f *billingFixture) deliver(e event) {
	f.t.Helper()
	raw, _ := json.Marshal(e)
	r := httptest.NewRequest("POST", "http://127.0.0.1:5173/api/v1/billing/paddle/webhook", strings.NewReader(string(raw)))
	r.Header.Set("Paddle-Signature", signed(raw, time.Now(), f.s.cfg.WebhookSecret))
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		f.t.Fatalf("webhook: %d %s", w.Code, w.Body.String())
	}
	if err := f.s.Reconcile(context.Background()); err != nil {
		f.t.Fatal(err)
	}
}
func (f *billingFixture) paid(txn, price, action string) event {
	data := map[string]any{"id": txn, "status": "completed", "currency_code": "USD", "customer_id": "ctm_" + f.wid, "subscription_id": f.subscription, "custom_data": map[string]string{"workspace_id": f.wid, "action_id": action}, "items": []any{map[string]any{"quantity": 1, "price": map[string]string{"id": price}}}, "billing_period": period{f.start, f.end}, "details": map[string]any{"totals": map[string]string{"total": "1000"}}}
	raw, _ := json.Marshal(data)
	e := event{ID: "evt_" + randomID(), Type: "transaction.completed", Occurred: time.Now().UTC()}
	json.Unmarshal(raw, &e.Data)
	return e
}
func (f *billingFixture) activate(plan string) {
	f.t.Helper()
	action, txn := f.action("checkout", plan)
	f.deliver(f.paid(txn, f.s.cfg.price(plan), action))
}
func (f *billingFixture) usage() Usage {
	f.t.Helper()
	u, err := f.s.Usage(context.Background(), f.wid)
	if err != nil {
		f.t.Fatal(err)
	}
	return u
}

func TestBillingRolesAndUnconfiguredBoundary(t *testing.T) {
	f := newBillingFixture(t)
	for _, role := range []string{"owner", "billing", "administrator", "editor"} {
		f.exec(`UPDATE syntelle_accounts.memberships SET role=$3 WHERE workspace_id=$1 AND account_id=$2`, f.wid, f.account, role)
		w := f.request("GET", "/billing", nil, "")
		want := 403
		if role == "owner" || role == "billing" {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("%s billing=%d body=%s", role, w.Code, w.Body.String())
		}
		if u := f.request("GET", "/usage", nil, ""); u.Code != 200 {
			t.Fatalf("%s usage=%d", role, u.Code)
		}
	}
	f.exec(`UPDATE syntelle_accounts.memberships SET role='owner' WHERE workspace_id=$1 AND account_id=$2`, f.wid, f.account)
	f.s.cfg.APIKey = ""
	w := f.request("GET", "/billing", nil, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("unconfigured=%s", w.Body.String())
	}
	if w = f.request("POST", "/billing/checkout", map[string]string{"plan": "test_starter"}, randomID()); w.Code != 503 {
		t.Fatalf("unconfigured action=%d", w.Code)
	}
	if _, err := f.s.ReserveIntegrationUsage(context.Background(), f.wid, f.integration, "session"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unconfigured reserve=%v", err)
	}
	if len(f.requests) != 0 {
		t.Fatal("unconfigured boundary called provider")
	}
}

func TestCheckoutIdempotencyPaidOnlyAndWebhookDedup(t *testing.T) {
	f := newBillingFixture(t)
	key := randomID()
	input := map[string]string{"plan": "test_starter"}
	w := f.request("POST", "/billing/checkout", input, key)
	if w.Code != 200 {
		t.Fatalf("checkout %d %s", w.Code, w.Body.String())
	}
	first := w.Body.String()
	if got := f.usage().Included; got != 0 {
		t.Fatalf("unpaid credit=%d", got)
	}
	if w = f.request("POST", "/billing/checkout", input, key); w.Code != 200 || w.Body.String() != first || len(f.requests) != 2 {
		t.Fatalf("duplicate checkout %d %s requests=%d", w.Code, w.Body.String(), len(f.requests))
	}
	if w = f.request("POST", "/billing/checkout", map[string]string{"plan": "test_team"}, key); w.Code != 409 {
		t.Fatalf("conflicting key=%d", w.Code)
	}
	var action, txn string
	var request []byte
	if err := f.mustQuery(`SELECT id,provider_id,request FROM syntelle_billing.actions WHERE workspace_id=$1`, f.wid).Scan(&action, &txn, &request); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(request), "test_starter") {
		t.Fatal("input not retained")
	}
	e := f.paid(txn, "pri_starter", action)
	f.deliver(e)
	f.deliver(e)
	e.ID = "evt_" + randomID()
	f.deliver(e)
	if u := f.usage(); u.Included != 3600 || u.Concurrency != 1 {
		t.Fatalf("paid usage=%+v", u)
	}
	var count int
	f.mustQuery(`SELECT count(*) FROM syntelle_billing.fulfillments WHERE workspace_id=$1`, f.wid).Scan(&count)
	if count != 1 {
		t.Fatalf("fulfillments=%d", count)
	}
	f.mustQuery(`SELECT count(*) FROM syntelle_billing.provider_requests WHERE action_id=$1 AND response IS NOT NULL`, action).Scan(&count)
	if count != 2 {
		t.Fatalf("provider audit=%d", count)
	}
}

func TestConcurrentAdmissionAndServerClockSettlement(t *testing.T) {
	f := newBillingFixture(t)
	f.activate("test_starter")
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := randomID()
			_, err := f.s.ReserveIntegrationUsage(ctx, f.wid, f.integration, id)
			errs <- err
			if err == nil {
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrConcurrency) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("admitted %d", success)
	}
	id := <-ids
	if u := f.usage(); u.Included != 3300 || u.Reserved != 300 || u.Active != 1 {
		t.Fatalf("reserved=%+v", u)
	}
	if err := f.s.ReleaseIntegrationUsage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if u := f.usage(); u.Included != 3600 || u.Used != 0 {
		t.Fatalf("startup charged=%+v", u)
	}
	id = randomID()
	if _, err := f.s.ReserveIntegrationUsage(ctx, f.wid, f.integration, id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.StartIntegrationUsage(ctx, id); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE syntelle_billing.usage_sessions SET started_at=clock_timestamp()-interval '10 seconds' WHERE session_id=$1`, id)
	if _, err := f.s.RenewIntegrationUsage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ReleaseIntegrationUsage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ReleaseIntegrationUsage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if u := f.usage(); u.Used < 10 || u.Used > 11 || u.Included+u.Used != 3600 || u.Active != 0 {
		t.Fatalf("settled=%+v", u)
	}
	if _, err := f.s.ReserveIntegrationUsage(ctx, f.wid, f.integration, id); !errors.Is(err, ErrExpired) {
		t.Fatalf("released identity reused=%v", err)
	}
}

func TestDailyCapExpiryAndCreditPriority(t *testing.T) {
	f := newBillingFixture(t)
	f.activate("test_team")
	action, txn := f.action("topup", "test_topup")
	f.deliver(f.paid(txn, "pri_topup", action))
	f.exec(`UPDATE syntelle_accounts.integrations SET daily_limit_seconds=60 WHERE id=$1`, f.integration)
	f.exec(`UPDATE syntelle_billing.grants SET available_seconds=20 WHERE workspace_id=$1 AND kind='monthly'`, f.wid)
	id := randomID()
	if _, err := f.s.ReserveIntegrationUsage(context.Background(), f.wid, f.integration, id); err != nil {
		t.Fatal(err)
	}
	if u := f.usage(); u.Included != 0 || u.Topup != 1760 || u.Reserved != 60 {
		t.Fatalf("priority=%+v", u)
	}
	if _, err := f.s.ReserveIntegrationUsage(context.Background(), f.wid, f.integration, randomID()); !errors.Is(err, ErrQuota) {
		t.Fatalf("daily cap=%v", err)
	}
	if err := f.s.StartIntegrationUsage(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE syntelle_billing.usage_sessions SET started_at=clock_timestamp()-interval '12 seconds',lease_expires_at=clock_timestamp()-interval '2 seconds' WHERE session_id=$1`, id)
	if err := f.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if u := f.usage(); u.Used < 9 || u.Used > 11 || u.Active != 0 || u.Topup != 1800 || u.Included+u.Used != 20 {
		t.Fatalf("expired release=%+v", u)
	}
	f.exec(`UPDATE syntelle_billing.grants SET expires_at=clock_timestamp()-interval '1 second' WHERE workspace_id=$1 AND kind='monthly'`, f.wid)
	if u := f.usage(); u.Included != 0 || u.Topup != 1800 {
		t.Fatalf("monthly expiry=%+v", u)
	}
}

func TestUpgradeDowngradeCancellationAndPaymentMethod(t *testing.T) {
	f := newBillingFixture(t)
	f.activate("test_starter")
	action, _ := f.action("subscription", "test_team")
	if u := f.usage(); u.Concurrency != 1 || u.Included != 3600 {
		t.Fatalf("unpaid upgrade=%+v", u)
	}
	e := f.paid("txn_"+randomID(), "pri_team", action)
	f.deliver(e)
	if u := f.usage(); u.Concurrency != 2 || u.Included <= 3600 || u.Included >= 10800 {
		t.Fatalf("prorated upgrade=%+v", u)
	}
	f.action("subscription", "test_starter")
	if u := f.usage(); u.Concurrency != 2 {
		t.Fatal("downgrade removed current rights")
	}
	f.action("payment-method", "")
	f.action("cancel", "")
	var cancel bool
	var pending string
	if err := f.mustQuery(`SELECT cancel_at_period_end,pending_plan FROM syntelle_billing.workspaces WHERE workspace_id=$1`, f.wid).Scan(&cancel, &pending); err != nil || !cancel || pending != "test_starter" {
		t.Fatalf("cancel=%v pending=%s err=%v", cancel, pending, err)
	}
	// A new paid period resets monthly credit; retained top-ups remain available.
	action, txn := f.action("topup", "test_topup")
	f.deliver(f.paid(txn, "pri_topup", action))
	f.exec(`UPDATE syntelle_billing.grants SET expires_at=clock_timestamp()-interval '1 second' WHERE workspace_id=$1 AND kind<>'topup'`, f.wid)
	if err := f.mustQuery(`SELECT date_trunc('second',clock_timestamp())-interval '1 minute'`).Scan(&f.start); err != nil {
		t.Fatal(err)
	}
	f.end = f.start.Add(30 * 24 * time.Hour)
	f.exec(`UPDATE syntelle_billing.workspaces SET period_start=$2,period_end=$3 WHERE workspace_id=$1`, f.wid, f.start.Add(-30*24*time.Hour), f.start)
	renew := f.paid("txn_"+randomID(), "pri_starter", "")
	f.deliver(renew)
	if u := f.usage(); u.Included != 3600 || u.Topup != 1800 || u.Concurrency != 1 {
		t.Fatalf("renewal=%+v", u)
	}
}

func TestOutOfOrderRefundFreezesGrantAndStaleStatusCannotReopen(t *testing.T) {
	f := newBillingFixture(t)
	action, txn := f.action("checkout", "test_starter")
	refund := event{ID: "evt_" + randomID(), Type: "adjustment.updated", Occurred: time.Now().UTC()}
	refund.Data.TransactionID = txn
	refund.Data.Status = "approved"
	f.deliver(refund)
	f.deliver(f.paid(txn, "pri_starter", action))
	if err := f.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if u := f.usage(); u.Included != 0 {
		t.Fatalf("frozen credit=%+v", u)
	}
	if _, err := f.s.ReserveIntegrationUsage(context.Background(), f.wid, f.integration, randomID()); !errors.Is(err, ErrQuota) {
		t.Fatalf("review admission=%v", err)
	}
	canceled := event{ID: "evt_" + randomID(), Type: "subscription.canceled", Occurred: time.Now().UTC()}
	canceled.Data.ID = f.subscription
	canceled.Data.Status = "canceled"
	f.deliver(canceled)
	stale := canceled
	stale.ID = "evt_" + randomID()
	stale.Type = "subscription.activated"
	stale.Data.Status = "active"
	stale.Occurred = stale.Occurred.Add(-time.Minute)
	f.deliver(stale)
	var status string
	f.mustQuery(`SELECT status FROM syntelle_billing.workspaces WHERE workspace_id=$1`, f.wid).Scan(&status)
	if status != "canceled" {
		t.Fatalf("stale status=%s", status)
	}
}

func TestRejectedCatalogAndAmbiguousMutationNeverRetry(t *testing.T) {
	f := newBillingFixture(t)
	f.transport = func(r *http.Request) (*http.Response, error) {
		return response(200, `{"data":{"id":"pri_starter","status":"active","unit_price":{"amount":"9900","currency_code":"USD"}}}`), nil
	}
	key := randomID()
	w := f.request("POST", "/billing/checkout", map[string]string{"plan": "test_starter"}, key)
	if w.Code != 502 {
		t.Fatalf("wrong catalog=%d", w.Code)
	}
	var state string
	f.mustQuery(`SELECT state FROM syntelle_billing.actions WHERE workspace_id=$1`, f.wid).Scan(&state)
	if state != "failed" {
		t.Fatalf("known preflight failure=%s", state)
	}
	f.transport = func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			return response(200, `{"data":{"id":"pri_starter","status":"active","unit_price":{"amount":"1000","currency_code":"USD"},"billing_cycle":{"interval":"month","frequency":1}}}`), nil
		}
		io.ReadAll(r.Body)
		return nil, errors.New("fixture connection lost after mutation")
	}
	key = randomID()
	w = f.request("POST", "/billing/checkout", map[string]string{"plan": "test_starter"}, key)
	if w.Code != 502 {
		t.Fatalf("ambiguous action=%d", w.Code)
	}
	n := len(f.requests)
	if w = f.request("POST", "/billing/checkout", map[string]string{"plan": "test_starter"}, key); w.Code != 409 || len(f.requests) != n {
		t.Fatalf("ambiguous action retried=%d", w.Code)
	}
	if w = f.request("POST", "/billing/checkout", map[string]string{"plan": "test_starter"}, randomID()); w.Code != 409 || len(f.requests) != n {
		t.Fatalf("new key bypassed ambiguity=%d", w.Code)
	}
}

func TestBillingAuthRejectsCrossTenantBearerAndCrossOrigin(t *testing.T) {
	f := newBillingFixture(t)
	for _, tc := range []struct{ name, origin, bearer, token, wid string }{
		{"missing cookie", "http://127.0.0.1:5173", "", "", f.wid},
		{"bearer cannot pay", "http://127.0.0.1:5173", "Bearer sk_fixture", f.token, f.wid},
		{"wrong origin", "https://untrusted.invalid", "", f.token, f.wid},
		{"other tenant", "http://127.0.0.1:5173", "", f.token, "w_unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://127.0.0.1:5173/api/v1/workspaces/"+tc.wid+"/billing/checkout", strings.NewReader(`{"plan":"test_starter"}`))
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-Syntelle-Request", "1")
			r.Header.Set("Authorization", tc.bearer)
			r.Header.Set("Idempotency-Key", randomID())
			if tc.token != "" {
				r.AddCookie(&http.Cookie{Name: "syntelle_session", Value: tc.token})
			}
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			if w.Code < 400 || w.Code >= 500 {
				t.Fatalf("boundary status=%d %s", w.Code, w.Body.String())
			}
		})
	}
	if len(f.requests) != 0 {
		t.Fatal("rejected authority called provider")
	}
}

func TestWebhookRawEvidenceAndCanonicalReconciliation(t *testing.T) {
	f := newBillingFixture(t)
	action, txn := f.action("checkout", "test_starter")
	e := f.paid(txn, "pri_starter", action)
	e.Data.BillingPeriod = nil
	raw, _ := json.Marshal(e)
	r := httptest.NewRequest("POST", "http://127.0.0.1:5173/api/v1/billing/paddle/webhook", strings.NewReader(string(raw)))
	r.Header.Set("Paddle-Signature", signed(raw, time.Now().Add(-time.Minute), f.s.cfg.WebhookSecret))
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("stale signature=%d", w.Code)
	}
	f.deliver(e)
	var stored []byte
	var state, reason string
	if err := f.mustQuery(`SELECT payload,state,reason FROM syntelle_billing.webhook_inbox WHERE event_id=$1`, e.ID).Scan(&stored, &state, &reason); err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(raw) || state != "pending" || reason != "awaiting_paid_period" {
		t.Fatalf("evidence mismatch state=%s reason=%s", state, reason)
	}
	complete := f.paid(txn, "pri_starter", action)
	complete.Data.UpdatedAt = time.Now().UTC()
	f.transport = func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Path != "/transactions/"+txn {
			return nil, errors.New("unexpected reconciliation request")
		}
		b, _ := json.Marshal(map[string]any{"data": complete.Data})
		return response(200, string(b)), nil
	}
	f.action("reconcile", "")
	if u := f.usage(); u.Included != 3600 {
		t.Fatalf("canonical paid credit=%+v", u)
	}
	// An ID can never be reused to overwrite the signed immutable event.
	e.Data.Status = "canceled"
	changed, _ := json.Marshal(e)
	r = httptest.NewRequest("POST", "http://127.0.0.1:5173/api/v1/billing/paddle/webhook", strings.NewReader(string(changed)))
	r.Header.Set("Paddle-Signature", signed(changed, time.Now(), f.s.cfg.WebhookSecret))
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("conflicting event bytes=%d", w.Code)
	}
	if err := f.mustQuery(`SELECT payload FROM syntelle_billing.webhook_inbox WHERE event_id=$1`, e.ID).Scan(&stored); err != nil || string(stored) != string(raw) {
		t.Fatal("signed event was overwritten")
	}
}

func TestRenewalRevocationReleasesUnusedReservation(t *testing.T) {
	f := newBillingFixture(t)
	f.activate("test_starter")
	id := randomID()
	if _, err := f.s.ReserveIntegrationUsage(context.Background(), f.wid, f.integration, id); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE syntelle_accounts.integrations SET enabled=false WHERE id=$1`, f.integration)
	if _, err := f.s.RenewIntegrationUsage(context.Background(), id); !errors.Is(err, ErrQuota) {
		t.Fatalf("revoked integration renewal=%v", err)
	}
	if u := f.usage(); u.Active != 0 || u.Reserved != 0 || u.Used != 0 || u.Included != 3600 {
		t.Fatalf("revoked startup settlement=%+v", u)
	}
}

func TestBillingRechecksMembershipBeforeReservation(t *testing.T) {
	f := newBillingFixture(t)
	authorize := f.s.cfg.AuthorizeWorkspace
	f.s.cfg.AuthorizeWorkspace = func(w http.ResponseWriter, r *http.Request, wid string) (string, string, bool, bool) {
		actor, role, suspended, ok := authorize(w, r, wid)
		if ok {
			f.exec(`DELETE FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND account_id=$2`, wid, actor)
		}
		return actor, role, suspended, ok
	}
	w := f.request("POST", "/billing/checkout", map[string]string{"plan": "test_starter"}, randomID())
	if w.Code != 403 || len(f.requests) != 0 {
		t.Fatalf("revoked role admitted: status=%d requests=%d", w.Code, len(f.requests))
	}
	var count int
	if err := f.mustQuery(`SELECT count(*) FROM syntelle_billing.actions WHERE workspace_id=$1`, f.wid).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unauthorized reservation=%d err=%v", count, err)
	}
}
