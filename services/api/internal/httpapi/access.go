package httpapi

import (
	"net"
	"net/http"
	"strings"
)

func AllowedOrigin(r *http.Request) bool {
	// This is an owner-only, loopback development service. Origin is required
	// for browser writes, including HTTP Read aloud; wildcard CORS is prohibited.
	switch r.Header.Get("Origin") {
	case "http://localhost:5173", "http://127.0.0.1:5173":
		return true
	}
	return false
}
func LocalOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/conversation/") || strings.HasPrefix(r.URL.Path, "/api/v1/speech/") {
			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			if host != "localhost" && host != "127.0.0.1" && host != "::1" {
				http.Error(w, "Loopback host required.", 403)
				return
			}
			if r.Method != http.MethodGet && !AllowedOrigin(r) {
				http.Error(w, "Local owner browser origin required.", http.StatusForbidden)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && !AllowedOrigin(r) {
				http.Error(w, "Unrelated origin rejected.", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
