package streaming

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AdmissionConfig bounds streaming access, independently of provider usage budgets.
type AdmissionConfig struct {
	RequestsPerMinute       int
	IPRequestsPerMinute     int
	ConnectionsPerMinute    int
	ConnectionsPerIP        int
	ConnectionsPerPrincipal int
	SessionsPerPrincipal    int
	DisableSessionLimit     bool
	TrustedProxyCIDRs       []string
}

func AdmissionConfigFromEnv() (AdmissionConfig, error) {
	var c AdmissionConfig
	if raw := os.Getenv("SYNTELLE_STREAM_SESSION_LIMIT_ENABLED"); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return c, errors.New("SYNTELLE_STREAM_SESSION_LIMIT_ENABLED must be true or false")
		}
		c.DisableSessionLimit = !enabled
	}
	for _, item := range []struct {
		name  string
		value *int
	}{
		{"SYNTELLE_STREAM_REQUESTS_PER_MINUTE", &c.RequestsPerMinute},
		{"SYNTELLE_STREAM_IP_REQUESTS_PER_MINUTE", &c.IPRequestsPerMinute},
		{"SYNTELLE_STREAM_CONNECTIONS_PER_MINUTE", &c.ConnectionsPerMinute},
		{"SYNTELLE_STREAM_CONNECTIONS_PER_IP", &c.ConnectionsPerIP},
		{"SYNTELLE_STREAM_CONNECTIONS_PER_PRINCIPAL", &c.ConnectionsPerPrincipal},
		{"SYNTELLE_STREAM_SESSIONS_PER_PRINCIPAL", &c.SessionsPerPrincipal},
	} {
		if raw := os.Getenv(item.name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 1000000 {
				return c, fmt.Errorf("%s must be between 1 and 1000000", item.name)
			}
			*item.value = n
		}
	}
	if raw := os.Getenv("SYNTELLE_STREAM_TRUSTED_PROXY_CIDRS"); raw != "" {
		c.TrustedProxyCIDRs = strings.Split(raw, ",")
	}
	return c, nil
}

type admissionKey struct {
	kind, id string
}
type admissionBucket struct {
	tokens        float64
	updated, seen time.Time
}
type streamAdmission struct {
	mu               sync.Mutex
	config           AdmissionConfig
	trusted          []netip.Prefix
	buckets          map[admissionKey]*admissionBucket
	connections      map[admissionKey]int
	totalConnections int
	lastSweep        time.Time
}

func newStreamAdmission(c AdmissionConfig) (*streamAdmission, error) {
	for _, item := range []struct {
		value    *int
		fallback int
	}{
		{&c.RequestsPerMinute, 600}, {&c.IPRequestsPerMinute, 3000},
		{&c.ConnectionsPerMinute, 30}, {&c.ConnectionsPerIP, 8},
		{&c.ConnectionsPerPrincipal, 2}, {&c.SessionsPerPrincipal, 1},
	} {
		if *item.value < 0 || *item.value > 1000000 {
			return nil, errors.New("invalid streaming admission limit")
		}
		if *item.value == 0 {
			*item.value = item.fallback
		}
	}
	a := &streamAdmission{config: c, buckets: make(map[admissionKey]*admissionBucket), connections: make(map[admissionKey]int)}
	for _, raw := range c.TrustedProxyCIDRs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil || prefix.Bits() == 0 {
			return nil, errors.New("stream trusted proxies must be explicit IP CIDRs")
		}
		a.trusted = append(a.trusted, prefix.Masked())
	}
	return a, nil
}

func (a *streamAdmission) trustedProxy(ip netip.Addr) bool {
	for _, prefix := range a.trusted {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func (a *streamAdmission) clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if value, _, err := net.SplitHostPort(host); err == nil {
		host = value
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	ip = ip.Unmap()
	// The immediate peer must be trusted. Strip trusted hops from the right,
	// never accept a client-supplied leftmost address as identity by itself.
	if !a.trustedProxy(ip) {
		return ip.String()
	}
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" || len(forwarded) > 4096 {
		return ip.String()
	}
	hops := strings.Split(forwarded, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		next, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return ip.String()
		}
		next = next.Unmap()
		if !a.trustedProxy(next) || i == 0 {
			return next.String()
		}
	}
	return ip.String()
}

// allow returns the minimum whole seconds to retry. State is bounded and idle
// buckets expire without a background goroutine or an external cache.
func (a *streamAdmission) allow(kind, id string, perMinute int, now time.Time) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if now.Sub(a.lastSweep) >= time.Minute {
		for key, bucket := range a.buckets {
			if now.Sub(bucket.seen) >= 2*time.Minute {
				delete(a.buckets, key)
			}
		}
		a.lastSweep = now
	}
	key := admissionKey{kind, id}
	burst := float64(min(perMinute, 60))
	if strings.Contains(kind, "connect") {
		burst = float64(min(perMinute, 5))
	}
	bucket := a.buckets[key]
	if bucket == nil {
		if len(a.buckets) >= 8192 {
			return 60
		}
		bucket = &admissionBucket{tokens: burst, updated: now}
		a.buckets[key] = bucket
	}
	rate := float64(perMinute) / 60
	if now.Before(bucket.updated) {
		now = bucket.updated
	}
	bucket.tokens = math.Min(burst, bucket.tokens+math.Max(0, now.Sub(bucket.updated).Seconds())*rate)
	bucket.updated, bucket.seen = now, now
	if bucket.tokens < 1 {
		return max(1, int(math.Ceil((1-bucket.tokens)/rate)))
	}
	bucket.tokens--
	return 0
}

func (a *streamAdmission) request(ip, principal string, now time.Time) int {
	if retry := a.allow("ip-request", ip, a.config.IPRequestsPerMinute, now); retry != 0 {
		return retry
	}
	if principal != "" {
		return a.allow("principal-request", principal, a.config.RequestsPerMinute, now)
	}
	return 0
}

func (a *streamAdmission) acquireConnection(kind, id string, maximum int) (func(), bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := admissionKey{kind, id}
	// Includes unauthenticated sockets waiting for their ticket.
	if a.connections[key] >= maximum || kind == "ip" && a.totalConnections >= 64 {
		return nil, false
	}
	a.connections[key]++
	if kind == "ip" {
		a.totalConnections++
	}
	return sync.OnceFunc(func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.connections[key]--
		if a.connections[key] == 0 {
			delete(a.connections, key)
		}
		if kind == "ip" {
			a.totalConnections--
		}
	}), true
}

func (s *Service) allowStreamRequest(w http.ResponseWriter, r *http.Request, principal string) bool {
	if s.admission == nil {
		return true
	}
	var retry int
	if principal == "" {
		retry = s.admission.request(s.admission.clientIP(r), "", time.Now())
	} else {
		retry = s.admission.allow("principal-request", principal, s.admission.config.RequestsPerMinute, time.Now())
	}
	if retry != 0 {
		limited(w, retry, "Too many streaming requests. Try again shortly.")
		return false
	}
	return true
}

func limited(w http.ResponseWriter, retry int, message string) {
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	writeError(w, http.StatusTooManyRequests, message)
}

var errPrincipalSessionLimit = errors.New("Close your active stream before opening another one.")
var errRenderCapacity = errors.New("Rendering capacity is busy. Close another session and try again.")
var errRendererUnavailable = errors.New("The private renderer is not running.")

func (s *Service) reserveSession(session *streamSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing || s.worker == nil {
		return errRendererUnavailable
	}
	if s.sessions[session.id] != nil {
		return errRenderCapacity
	}
	if s.admission != nil && !s.admission.config.DisableSessionLimit {
		count := 0
		for _, existing := range s.sessions {
			if existing.principal.ID == session.principal.ID {
				count++
			}
		}
		if count >= s.admission.config.SessionsPerPrincipal {
			return errPrincipalSessionLimit
		}
	}
	if len(s.sessions) >= s.config.MaximumSessions {
		return errRenderCapacity
	}
	s.sessions[session.id] = session
	return nil
}
