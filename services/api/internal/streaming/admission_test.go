package streaming

import (
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

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

func testAdmission(t *testing.T, config AdmissionConfig) *streamAdmission {
	t.Helper()
	a, err := newStreamAdmission(config)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAdmissionConfiguration(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		a := testAdmission(t, AdmissionConfig{})
		if a.config.SessionsPerPrincipal != 1 || a.config.ConnectionsPerIP != 8 {
			t.Fatal("unsafe admission defaults")
		}
	})
	t.Run("invalid proxy", func(t *testing.T) {
		if _, err := newStreamAdmission(AdmissionConfig{TrustedProxyCIDRs: []string{"proxy.example"}}); err == nil {
			t.Fatal("accepted an unbounded proxy identity")
		}
	})
	t.Run("universal proxy trust", func(t *testing.T) {
		for _, cidr := range []string{"0.0.0.0/0", "::/0"} {
			if _, err := newStreamAdmission(AdmissionConfig{TrustedProxyCIDRs: []string{cidr}}); err == nil {
				t.Fatal("accepted universal forwarded-header trust")
			}
		}
	})
	t.Run("invalid environment limit", func(t *testing.T) {
		t.Setenv("SYNTELLE_STREAM_REQUESTS_PER_MINUTE", "0")
		if _, err := AdmissionConfigFromEnv(); err == nil {
			t.Fatal("silently disabled admission")
		}
	})
	t.Run("explicit local session exception", func(t *testing.T) {
		t.Setenv("SYNTELLE_STREAM_SESSION_LIMIT_ENABLED", "false")
		config, err := AdmissionConfigFromEnv()
		if err != nil || !config.DisableSessionLimit {
			t.Fatal("local session exception was not applied", err)
		}
	})
	t.Run("invalid session switch", func(t *testing.T) {
		t.Setenv("SYNTELLE_STREAM_SESSION_LIMIT_ENABLED", "maybe")
		if _, err := AdmissionConfigFromEnv(); err == nil {
			t.Fatal("invalid switch silently changed access policy")
		}
	})
}

func TestAdmissionLocalExceptionPreservesGlobalCapacity(t *testing.T) {
	a := testAdmission(t, AdmissionConfig{DisableSessionLimit: true})
	service := &Service{admission: a, worker: &peer{}, sessions: make(map[string]*streamSession), config: Config{MaximumSessions: 2}}
	principal := accounts.Principal{ID: "account:one"}
	for _, id := range []string{"first", "second"} {
		if err := service.reserveSession(&streamSession{id: id, principal: principal}); err != nil {
			t.Fatal("local visitor could not open another tab", err)
		}
	}
	if err := service.reserveSession(&streamSession{id: "third", principal: principal}); !errors.Is(err, errRenderCapacity) {
		t.Fatal("local exception disabled global renderer capacity", err)
	}
}

func TestAdmissionClientIP(t *testing.T) {
	for _, tc := range []struct {
		name, peer, forwarded, want string
		trusted                     []string
	}{
		{"direct", "203.0.113.7:443", "", "203.0.113.7", nil},
		{"forged header", "203.0.113.7:443", "198.51.100.8", "203.0.113.7", nil},
		{"trusted proxy", "192.0.2.10:443", "198.51.100.8", "198.51.100.8", []string{"192.0.2.10/32"}},
		{"forged leftmost hop", "192.0.2.10:443", "198.51.100.8, 203.0.113.7", "203.0.113.7", []string{"192.0.2.10/32"}},
		{"trusted chain", "192.0.2.10:443", "203.0.113.7, 192.0.2.11", "203.0.113.7", []string{"192.0.2.0/24"}},
		{"malformed chain", "192.0.2.10:443", "not-an-ip", "192.0.2.10", []string{"192.0.2.10/32"}},
		{"mapped IPv4", "[::ffff:203.0.113.7]:443", "", "203.0.113.7", nil},
		{"IPv6", "[2001:db8::1]:443", "", "2001:db8::1", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testAdmission(t, AdmissionConfig{TrustedProxyCIDRs: tc.trusted})
			r := httptest.NewRequest("GET", "http://localhost/api/v1/streams/status", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			if got := a.clientIP(r); got != tc.want {
				t.Fatalf("client IP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAdmissionSharedIdentityAndIPLimits(t *testing.T) {
	now := time.Now()
	t.Run("identity across IPs", func(t *testing.T) {
		a := testAdmission(t, AdmissionConfig{RequestsPerMinute: 2})
		for i := range 2 {
			if a.request(fmt.Sprint(i), "account:one", now) != 0 {
				t.Fatal("normal request rejected")
			}
		}
		if a.request("new-ip", "account:one", now) == 0 {
			t.Fatal("changing IP bypassed account limit")
		}
		if a.request("new-ip", "account:two", now) != 0 {
			t.Fatal("unrelated account was blocked")
		}
		if a.request("new-ip", "account:one", now.Add(31*time.Second)) != 0 {
			t.Fatal("rate allowance did not recover")
		}
	})
	t.Run("IP across identities", func(t *testing.T) {
		a := testAdmission(t, AdmissionConfig{IPRequestsPerMinute: 2})
		for i := range 2 {
			if a.request("shared-ip", fmt.Sprint(i), now) != 0 {
				t.Fatal("normal request rejected")
			}
		}
		if a.request("shared-ip", "new-guest", now) == 0 {
			t.Fatal("new identity bypassed IP limit")
		}
	})
	t.Run("simultaneous requests", func(t *testing.T) {
		a := testAdmission(t, AdmissionConfig{RequestsPerMinute: 4})
		var accepted atomic.Int32
		var wg sync.WaitGroup
		for range 32 {
			wg.Go(func() {
				if a.request("shared-ip", "same-account", now) == 0 {
					accepted.Add(1)
				}
			})
		}
		wg.Wait()
		if accepted.Load() != 4 {
			t.Fatalf("accepted %d simultaneous requests", accepted.Load())
		}
	})
}

func TestAdmissionStateIsBoundedAndExpires(t *testing.T) {
	a := testAdmission(t, AdmissionConfig{})
	now := time.Now()
	for i := range 8192 {
		if a.allow("ip-request", fmt.Sprint(i), 60, now) != 0 {
			t.Fatal("early capacity rejection")
		}
	}
	if a.allow("ip-request", "overflow", 60, now) == 0 {
		t.Fatal("unbounded identity allocation")
	}
	if a.allow("ip-request", "new-client", 60, now.Add(3*time.Minute)) != 0 {
		t.Fatal("expired entries block new clients")
	}
}

func TestAdmissionConnectionReservationsReleaseExactlyOnce(t *testing.T) {
	a := testAdmission(t, AdmissionConfig{})
	release, ok := a.acquireConnection("ip", "one", 1)
	if !ok {
		t.Fatal("first connection rejected")
	}
	if _, ok = a.acquireConnection("ip", "one", 1); ok {
		t.Fatal("connection cap bypassed")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(release)
	}
	wg.Wait()
	release, ok = a.acquireConnection("ip", "one", 1)
	if !ok {
		t.Fatal("closed connection leaked admission slot")
	}
	release()
	if len(a.connections) != 0 || a.totalConnections != 0 {
		t.Fatal("connection counters leaked")
	}
}

func TestAdmissionHTTPRejectionPrecedesIdentityAndProviderWork(t *testing.T) {
	a := testAdmission(t, AdmissionConfig{IPRequestsPerMinute: 1})
	service := &Service{admission: a}
	var reached atomic.Int32
	service.resolve = func(http.ResponseWriter, *http.Request, bool) (accounts.Principal, error) {
		reached.Add(1)
		return accounts.Principal{}, errors.New("fixture unauthorized")
	}
	handler := service.Handler(http.NotFoundHandler())
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://localhost/api/v1/streams", strings.NewReader("{}"))
		r.RemoteAddr = "203.0.113.7:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if request().Code != 401 {
		t.Fatal("fixture boundary not reached")
	}
	blocked := request()
	if blocked.Code != 429 || blocked.Header().Get("Retry-After") == "" || reached.Load() != 1 {
		t.Fatal("request reached identity/provider work after throttle")
	}
}

func TestAdmissionCapsUnauthenticatedSocketsAndReleasesOnDisconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	a := testAdmission(t, AdmissionConfig{ConnectionsPerIP: 1})
	service := &Service{admission: a}
	done := make(chan struct{}, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { done <- struct{}{} }()
		service.playerSocket(w, r, "fixture_session")
	}))
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http")
	first, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.CloseNow()
	_, response, err := websocket.Dial(ctx, endpoint, nil)
	if err == nil || response == nil || response.StatusCode != 429 {
		t.Fatal("unauthenticated connection cap not enforced")
	}
	<-done // rejected second handshake
	_ = first.CloseNow()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("unauthenticated connection did not release")
	}
	third, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatal("released IP cannot reconnect:", err)
	}
	_ = third.CloseNow()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("replacement connection did not release")
	}
}

func TestAdmissionSessionLimitRejectsHTTPBeforePersistence(t *testing.T) {
	a := testAdmission(t, AdmissionConfig{})
	principal := accounts.Principal{ID: "account:one", Kind: "account", SessionID: "browser-one"}
	service := &Service{
		ctx: t.Context(), admission: a, worker: &peer{}, config: Config{MaximumSessions: 2},
		sessions: map[string]*streamSession{"existing": {id: "existing", principal: principal}},
		resolve:  func(http.ResponseWriter, *http.Request, bool) (accounts.Principal, error) { return principal, nil },
	}
	r := httptest.NewRequest("POST", "http://localhost/api/v1/streams", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	service.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || len(service.sessions) != 1 {
		t.Fatal("duplicate visitor reached persistence/provider code", w.Code)
	}
}

func TestAdmissionWebSocketControlsShareRequestLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	a := testAdmission(t, AdmissionConfig{RequestsPerMinute: 1})
	session := &streamSession{id: "fixture_session", principal: accounts.Principal{ID: "account:one"}, state: "idle"}
	ticket, err := issueTicket(session)
	if err != nil {
		t.Fatal(err)
	}
	worker := &peer{ctx: ctx, out: make(chan Envelope, 8)}
	service := &Service{ctx: ctx, admission: a, sessions: map[string]*streamSession{session.id: session}, worker: worker, conversation: &conversation.Service{}}
	var authorizationChecks atomic.Int32
	service.config.ValidatePrincipal = func(context.Context, accounts.Principal) error {
		authorizationChecks.Add(1)
		return nil
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { service.playerSocket(w, r, session.id) }))
	defer server.Close()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow(); cancel(); service.wg.Wait() }()
	if err = wsjson.Write(ctx, c, envelope("authenticate", session.id, map[string]string{"ticket": ticket})); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		var event Envelope
		if err = wsjson.Read(ctx, c, &event); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"signal", "command"} {
		if err = wsjson.Write(ctx, c, envelope(kind, session.id, map[string]string{"candidate": "offline-fixture"})); err != nil {
			t.Fatal(err)
		}
	}
	var event Envelope
	if err = wsjson.Read(ctx, c, &event); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Retry int `json:"retry_after"`
	}
	if event.Type != "error" || json.Unmarshal(event.Data, &payload) != nil || payload.Retry < 1 {
		t.Fatal("socket did not report throttling")
	}
	if len(worker.out) != 1 {
		t.Fatal("throttled signal reached worker")
	}
	if authorizationChecks.Load() != 1 {
		t.Fatal("throttled command caused an extra authorization query")
	}
}

func TestAdmissionConcurrentSessionsShareAccountAcrossBrowserAndKey(t *testing.T) {
	a := testAdmission(t, AdmissionConfig{})
	service := &Service{admission: a, worker: &peer{}, sessions: make(map[string]*streamSession), config: Config{MaximumSessions: 50}}
	var accepted atomic.Int32
	var unexpected atomic.Int32
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			principal := accounts.Principal{ID: "account:one", Kind: "account", SessionID: fmt.Sprint(i)}
			if i%2 == 0 {
				principal.Kind, principal.KeyID = "key", fmt.Sprint(i)
			}
			err := service.reserveSession(&streamSession{id: fmt.Sprint(i), principal: principal})
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, errPrincipalSessionLimit) {
				unexpected.Add(1)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 1 || unexpected.Load() != 0 {
		t.Fatalf("session admissions: accepted=%d unexpected=%d", accepted.Load(), unexpected.Load())
	}
	if err := service.reserveSession(&streamSession{id: "other", principal: accounts.Principal{ID: "account:two"}}); err != nil {
		t.Fatal("unrelated visitor was blocked:", err)
	}
}
