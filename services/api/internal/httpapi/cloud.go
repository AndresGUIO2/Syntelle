package httpapi

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syntelle/render-contract"
)

func Cloud() bool { return os.Getenv("SYNTELLE_DEPLOYMENT_MODE") == "cloud" }
func ValidateDeployment() error {
	mode := os.Getenv("SYNTELLE_DEPLOYMENT_MODE")
	if mode != "" && mode != "local" && mode != "cloud" {
		return errors.New("invalid deployment mode")
	}
	if !Cloud() {
		return nil
	}
	if os.Getenv("SYNTELLE_LOCAL_OWNER") != "0" {
		return errors.New("cloud requires SYNTELLE_LOCAL_OWNER=0")
	}
	if _, err := rendercontract.ICEServers(); err != nil {
		return err
	}
	if os.Getenv("SYNTELLE_STREAM_TIME_LIMITS_ENABLED") != "true" {
		return errors.New("cloud requires session time limits")
	}
	if raw := os.Getenv("SYNTELLE_STREAM_SESSION_LIMIT_ENABLED"); raw != "" && raw != "true" {
		return errors.New("cloud requires per-visitor session limits")
	}
	u, e := url.Parse(os.Getenv("SYNTELLE_API_ORIGIN"))
	if e != nil || u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), ".azurecontainerapps.io") || u.Port() != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return errors.New("configure exact HTTPS Azure API origin")
	}
	if os.Getenv("SYNTELLE_APP_ORIGIN") != "https://app.syntelle.online" {
		return errors.New("invalid cloud application origin")
	}
	for _, key := range []string{"SYNTELLE_EDGE_TOKEN", "SYNTELLE_PROCESSOR_TOKEN", "SYNTELLE_RENDER_WORKER_TOKEN", "SYNTELLE_MODAL_LAUNCH_TOKEN"} {
		if len(os.Getenv(key)) < 32 {
			return errors.New("missing cloud service credential: " + key)
		}
	}
	return nil
}
func EdgeRequest(r *http.Request) bool {
	expected := os.Getenv("SYNTELLE_EDGE_TOKEN")
	return len(expected) >= 32 && subtle.ConstantTimeCompare([]byte(expected), []byte(r.Header.Get("X-Syntelle-Edge-Token"))) == 1
}
func CloudRequest(r *http.Request) bool {
	u, e := url.Parse(os.Getenv("SYNTELLE_API_ORIGIN"))
	if e != nil || r.Host != u.Host {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin != "" && origin != "https://app.syntelle.online" {
		return false
	}
	return EdgeRequest(r) || (origin == "" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func DeploymentGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !Cloud() {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/conversation/") || strings.HasPrefix(r.URL.Path, "/api/v1/speech/") {
			http.Error(w, "Unavailable", 404)
			return
		}
		if r.URL.Path != "/healthz" && !CloudRequest(r) {
			http.Error(w, "Service authorization required", 403)
			return
		}
		// Only the authenticated edge can assert the browser IP. Direct integrations
		// retain their actual ingress peer until trusted ingress CIDRs are configured.
		if EdgeRequest(r) {
			if ip := net.ParseIP(r.Header.Get("X-Syntelle-Client-IP")); ip != nil {
				r.RemoteAddr = net.JoinHostPort(ip.String(), "0")
			}
		}
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Forwarded-Host")
		r.Header.Del("X-Forwarded-Proto")
		r.Header.Del("X-Real-IP")
		r.Header.Del("CF-Connecting-IP")
		r.Header.Del("True-Client-IP")
		r.Header.Del("Forwarded")
		next.ServeHTTP(w, r)
	})
}
