//go:build integration

package streaming

import (
	"net/http"
	"net/http/httptest"
	"syntelle/api/internal/httpapi"
	"testing"
)

func TestIsolatedFixtureOriginIsExactAndTestOnly(t *testing.T) {
	for _, tc := range []struct {
		name, configured, incoming, forwarded string
		status                                int
	}{
		{"isolated browser", "http://127.0.0.1:5174", "http://127.0.0.1:5174", "http://127.0.0.1:5173", 204},
		{"native worker", "http://127.0.0.1:5174", "", "", 204},
		{"other local page", "http://127.0.0.1:5174", "http://127.0.0.1:5173", "", 403},
		{"lookalike hostname", "http://127.0.0.1:5174", "http://localhost:5174", "", 403},
		{"foreign origin", "http://127.0.0.1:5174", "https://example.invalid", "", 403},
		{"unapproved configuration", "https://example.invalid", "https://example.invalid", "", 403},
		{"unchanged default fixture", "http://127.0.0.1:5173", "http://localhost:5173", "http://localhost:5173", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8083/api/v1/streams/session-fixture/ws", nil)
			r.Header.Set("Origin", tc.incoming)
			r.Header.Set("Upgrade", "websocket")
			called := false
			h := fixtureStreamOrigin(tc.configured, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Header.Get("Origin") != tc.forwarded || r.Header.Get("Upgrade") != "websocket" {
					t.Fatal("incorrect forwarded socket headers")
				}
				if tc.forwarded != "" && !httpapi.AllowedOrigin(r) {
					t.Fatal("canonical streaming check failed")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || called != (tc.status == 204) {
				t.Fatal(w.Code, called)
			}
			if r.Header.Get("Origin") != tc.incoming {
				t.Fatal("original authentication request was mutated")
			}
		})
	}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8083/", nil)
	r.Header.Set("Origin", "http://127.0.0.1:5174")
	if httpapi.AllowedOrigin(r) {
		t.Fatal("production origin allowance was broadened")
	}
}
