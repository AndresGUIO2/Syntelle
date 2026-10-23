//go:build integration

package accounts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type capturedMail struct {
	mu    sync.Mutex
	codes map[string]string
	calls int
	fail  bool
}

func (m *capturedMail) SendCode(_ context.Context, email, code, id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.codes[id] = code
	return "fixture-delivery", nil
}
func (m *capturedMail) code(id string) string { m.mu.Lock(); defer m.mu.Unlock(); return m.codes[id] }

type accountFixture struct {
	s       *Service
	pool    *pgxpool.Pool
	mail    *capturedMail
	now     time.Time
	t       *testing.T
	handler http.Handler
}

func newAccountFixture(t *testing.T) *accountFixture {
	t.Helper()
	dsn := os.Getenv("SYNTELLE_ACCOUNTS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated account database not configured")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid fixture database")
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "syntelle_accounts_test_") {
		t.Fatal("refusing non-fixture database")
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal("fixture database unavailable")
	}
	t.Cleanup(pool.Close)
	f := &accountFixture{pool: pool, mail: &capturedMail{codes: map[string]string{}}, now: time.Now().UTC(), t: t}
	s, err := Open(context.Background(), pool, Config{Secret: strings.Repeat("fixture-only-key-", 3), Origin: "http://127.0.0.1:5173", Mailer: f.mail, PackageHash: strings.Repeat("a", 64), Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	f.s = s
	f.handler = s.Handler(http.NotFoundHandler())
	return f
}
func (f *accountFixture) request(method, path string, body any, cookie *http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	f.t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://127.0.0.1:5173"+path, strings.NewReader(string(b)))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://127.0.0.1:5173")
	r.Header.Set("X-Syntelle-Request", "1")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func fixtureJSON(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("expected status %d, got %d (%s)", status, w.Code, w.Body.String())
	}
	var b map[string]any
	if json.Unmarshal(w.Body.Bytes(), &b) != nil {
		t.Fatal("invalid JSON response")
	}
	return b
}
func (f *accountFixture) guest() *http.Cookie {
	w := f.request("GET", "/api/v1/auth/me", nil, nil, nil)
	fixtureJSON(f.t, w, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		f.t.Fatal("guest cookie missing")
	}
	return cookies[0]
}
func (f *accountFixture) start(cookie *http.Cookie, email string, extra map[string]any) (map[string]any, string) {
	input := map[string]any{"email": email}
	for k, v := range extra {
		input[k] = v
	}
	w := f.request("POST", "/api/v1/auth/email/start", input, cookie, nil)
	b := fixtureJSON(f.t, w, 202)
	return b, f.mail.code(b["challenge_id"].(string))
}
func (f *accountFixture) signin(cookie *http.Cookie, email string) (*http.Cookie, map[string]any) {
	f.now = f.now.Add(61 * time.Second)
	b, code := f.start(cookie, email, nil)
	w := f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": b["challenge_id"], "code": code}, cookie, nil)
	out := fixtureJSON(f.t, w, 200)
	return w.Result().Cookies()[0], out["account"].(map[string]any)
}

func TestAccountLifecycleAndOwnership(t *testing.T) {
	f := newAccountFixture(t)
	ctx := context.Background()
	cookie := f.guest()
	guestConfig := fixtureJSON(t, f.request("POST", "/api/v1/configurations", map[string]any{"name": "Guest setup", "settings": Defaults()}, cookie, nil), 201)
	challenge, code := f.start(cookie, "lifecycle-"+randomID()+"@example.com", nil)
	wrongBrowser := f.guest()
	fixtureJSON(t, f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": challenge["challenge_id"], "code": code}, wrongBrowser, nil), 400)
	w := f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": challenge["challenge_id"], "code": code}, cookie, nil)
	login := fixtureJSON(t, w, 200)
	signed := w.Result().Cookies()[0]
	if signed.Value == cookie.Value {
		t.Fatal("session fixation: cookie did not rotate")
	}
	account := login["account"].(map[string]any)
	// A guest configuration must not be silently imported on sign-in.
	fixtureJSON(t, f.request("GET", "/api/v1/configurations/"+guestConfig["id"].(string), nil, signed, nil), 404)
	fixtureJSON(t, f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": challenge["challenge_id"], "code": code}, signed, nil), 400)
	var stored, mailState string
	if err := f.pool.QueryRow(ctx, `SELECT verifier,delivery_status FROM syntelle_accounts.email_challenges WHERE id=$1`, challenge["challenge_id"]).Scan(&stored, &mailState); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, code) || mailState != "sent" {
		t.Fatal("code stored unsafely or delivery not recorded")
	}
	config := fixtureJSON(t, f.request("POST", "/api/v1/configurations", map[string]any{"name": "Saved setup", "settings": Defaults()}, signed, nil), 201)
	if config["package_hash"] != strings.Repeat("a", 64) {
		t.Fatal("server package identity lost")
	}
	created := fixtureJSON(t, f.request("POST", "/api/v1/keys", map[string]any{"name": "Integration"}, signed, nil), 201)
	key := created["secret"].(string)
	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	p, err := f.s.Resolve(httptest.NewRecorder(), req, false)
	if err != nil || p.AccountID != account["id"] || p.Kind != "key" {
		t.Fatalf("valid key failed: %v", err)
	}
	if _, err = f.s.Configuration(ctx, p, config["id"].(string)); err != nil {
		t.Fatal("key cannot select owned configuration")
	}
	fixtureJSON(t, f.request("GET", "/api/v1/keys", nil, nil, map[string]string{"Authorization": "Bearer " + key}), 401)
	other, _ := f.signin(f.guest(), "other-"+randomID()+"@example.com")
	fixtureJSON(t, f.request("GET", "/api/v1/configurations/"+config["id"].(string), nil, other, nil), 404)
	fixtureJSON(t, f.request("DELETE", "/api/v1/keys/"+created["id"].(string), nil, other, nil), 404)
	var revoked string
	f.s.OnRevoked = func(kind, id string) { revoked = kind + ":" + id }
	fixtureJSON(t, f.request("DELETE", "/api/v1/keys/"+created["id"].(string), nil, signed, nil), 200)
	if revoked != "key:"+created["id"].(string) {
		t.Fatal("key sessions not notified")
	}
	if _, err = f.s.Resolve(httptest.NewRecorder(), req, false); err == nil {
		t.Fatal("revoked key accepted")
	}
	fixtureJSON(t, f.request("POST", "/api/v1/auth/logout", nil, signed, nil), 200)
	if !strings.HasPrefix(revoked, "session:") {
		t.Fatal("logout sessions not notified")
	}
	if _, err = f.s.Resolve(httptest.NewRecorder(), httptestRequestWithCookie(signed), false); err == nil {
		t.Fatal("logged-out session accepted")
	}
}

func httptestRequestWithCookie(cookie *http.Cookie) *http.Request {
	r := httptest.NewRequest("GET", "http://127.0.0.1:5173/", nil)
	r.AddCookie(cookie)
	return r
}

func TestEmailLimitsExpiryAndIdempotency(t *testing.T) {
	f := newAccountFixture(t)
	cookie := f.guest()
	email := "limits-" + randomID() + "@example.com"
	requestKey := randomID()
	headers := map[string]string{"Idempotency-Key": requestKey}
	w := f.request("POST", "/api/v1/auth/email/start", map[string]any{"email": email}, cookie, headers)
	first := fixtureJSON(t, w, 202)
	calls := f.mail.calls
	second := fixtureJSON(t, f.request("POST", "/api/v1/auth/email/start", map[string]any{"email": email}, cookie, headers), 202)
	if first["challenge_id"] != second["challenge_id"] || f.mail.calls != calls {
		t.Fatal("duplicate delivery")
	}
	fixtureJSON(t, f.request("POST", "/api/v1/auth/email/start", map[string]any{"email": email}, cookie, nil), 429)
	for i := 0; i < 5; i++ {
		fixtureJSON(t, f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": first["challenge_id"], "code": "not-the-code"}, cookie, nil), 400)
	}
	fixtureJSON(t, f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": first["challenge_id"], "code": f.mail.code(first["challenge_id"].(string))}, cookie, nil), 400)
	f.now = f.now.Add(61 * time.Second)
	fresh, code := f.start(cookie, email, nil)
	f.now = f.now.Add(11 * time.Minute)
	fixtureJSON(t, f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": fresh["challenge_id"], "code": code}, cookie, nil), 400)
	for i := 0; i < 3; i++ {
		f.now = f.now.Add(61 * time.Second)
		f.start(cookie, email, nil)
	}
	f.now = f.now.Add(61 * time.Second)
	fixtureJSON(t, f.request("POST", "/api/v1/auth/email/start", map[string]any{"email": email}, cookie, nil), 429)
}

func TestGitHubCodeFlowAndExplicitAccountLink(t *testing.T) {
	f := newAccountFixture(t)
	email := "social-" + randomID() + "@example.com"
	guest := f.guest()
	signed, account := f.signin(guest, email)
	_ = signed
	f.s.cfg.GitHubClientID = "fixture-client"
	f.s.cfg.GitHubClientSecret = "fixture-secret"
	tokenCalls := 0
	f.s.cfg.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "github.com/login/oauth/access_token":
			tokenCalls++
			b, _ := io.ReadAll(r.Body)
			values, _ := url.ParseQuery(string(b))
			if values.Get("code_verifier") == "" {
				t.Fatal("PKCE verifier missing")
			}
			return jsonResponse(map[string]any{"access_token": "fixture-token", "token_type": "bearer"}, 200), nil
		case "api.github.com/user":
			return jsonResponse(map[string]any{"id": 918273, "login": "fixture", "name": "Fixture User"}, 200), nil
		case "api.github.com/user/emails":
			return jsonResponse([]any{map[string]any{"email": email, "verified": true, "primary": true}}, 200), nil
		default:
			t.Fatal("unexpected external auth request")
			return nil, nil
		}
	})}
	newBrowser := f.guest()
	start := f.request("GET", "/api/v1/auth/github/start?return_to=https://foreign.invalid", nil, newBrowser, nil)
	if start.Code != 303 {
		t.Fatal("provider did not redirect")
	}
	authorization, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if authorization.Query().Get("code_challenge_method") != "S256" || authorization.Query().Get("state") == "" {
		t.Fatal("missing state/PKCE")
	}
	callbackPath := "/api/v1/auth/github/callback?state=" + url.QueryEscape(authorization.Query().Get("state")) + "&code=fixture-code"
	callback := f.request("GET", callbackPath, nil, newBrowser, nil)
	if callback.Code != 303 {
		t.Fatal("provider callback failed")
	}
	location, _ := url.Parse(callback.Header().Get("Location"))
	flow := location.Query().Get("flow_id")
	if flow == "" || location.Query().Get("auth") != "verify-email" {
		t.Fatal("existing account was merged without proof")
	}
	var identities int
	if err = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM syntelle_accounts.identities WHERE provider='github' AND subject='918273'`).Scan(&identities); err != nil || identities != 0 {
		t.Fatal("identity attached prematurely")
	}
	f.now = f.now.Add(61 * time.Second)
	challenge, code := f.start(newBrowser, email, map[string]any{"flow_id": flow})
	fixtureJSON(t, f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": challenge["challenge_id"], "code": code}, newBrowser, nil), 400)
	linked := fixtureJSON(t, f.request("POST", "/api/v1/auth/email/verify", map[string]any{"challenge_id": challenge["challenge_id"], "code": code, "confirm_link": true}, newBrowser, nil), 200)
	if linked["account"].(map[string]any)["id"] != account["id"] {
		t.Fatal("duplicate account created")
	}
	f.request("GET", callbackPath, nil, newBrowser, nil)
	if tokenCalls != 1 {
		t.Fatal("consumed callback repeated provider exchange")
	}
}
