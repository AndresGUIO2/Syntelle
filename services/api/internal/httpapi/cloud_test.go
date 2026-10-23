package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudBoundaries(t *testing.T) {
	t.Setenv("SYNTELLE_DEPLOYMENT_MODE", "cloud")
	t.Setenv("SYNTELLE_API_ORIGIN", "https://api.fixture.azurecontainerapps.io")
	t.Setenv("SYNTELLE_EDGE_TOKEN", strings.Repeat("s", 32))
	h := DeploymentGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Peer", r.RemoteAddr); w.WriteHeader(204) }))
	for _, tc := range []struct {
		name, path, host, origin, token, auth string
		want                                  int
	}{
		{"unauthenticated", "/api/v1/auth/me", "api.fixture.azurecontainerapps.io", "", "", "", 403},
		{"forged host", "/api/v1/auth/me", "evil.invalid", "", "", "Bearer key", 403},
		{"foreign origin", "/api/v1/auth/me", "api.fixture.azurecontainerapps.io", "https://evil.invalid", strings.Repeat("s", 32), "", 403},
		{"edge", "/api/v1/auth/me", "api.fixture.azurecontainerapps.io", "https://app.syntelle.online", strings.Repeat("s", 32), "", 204},
		{"integration", "/api/v1/streams", "api.fixture.azurecontainerapps.io", "", "", "Bearer key", 204},
		{"owner", "/api/v1/speech/trial", "api.fixture.azurecontainerapps.io", "", strings.Repeat("s", 32), "", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "https://"+tc.host+tc.path, nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-Syntelle-Edge-Token", tc.token)
			r.Header.Set("Authorization", tc.auth)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatal(w.Code)
			}
		})
	}
}
func TestCloudIPRequiresAuthenticatedEdge(t *testing.T) {
	t.Setenv("SYNTELLE_DEPLOYMENT_MODE", "cloud")
	t.Setenv("SYNTELLE_API_ORIGIN", "https://api.fixture.azurecontainerapps.io")
	t.Setenv("SYNTELLE_EDGE_TOKEN", strings.Repeat("s", 32))
	for _, edge := range []bool{false, true} {
		r := httptest.NewRequest("GET", "https://api.fixture.azurecontainerapps.io/api/v1/streams", nil)
		r.Header.Set("Authorization", "Bearer key")
		r.Header.Set("X-Syntelle-Client-IP", "203.0.113.9")
		r.Header.Set("X-Forwarded-For", "forged")
		if edge {
			r.Header.Set("X-Syntelle-Edge-Token", strings.Repeat("s", 32))
		}
		DeploymentGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.RemoteAddr, "203.0.113.9") != edge {
				t.Fatal("IP spoofing")
			}
			if r.Header.Get("X-Forwarded-For") != "" {
				t.Fatal("forwarded header survived")
			}
		})).ServeHTTP(httptest.NewRecorder(), r)
	}
}

func TestCloudConfigurationRequiresPrivateServicesAndTURN(t *testing.T) {
	t.Setenv("SYNTELLE_DEPLOYMENT_MODE", "cloud")
	t.Setenv("SYNTELLE_API_ORIGIN", "https://api.fixture.azurecontainerapps.io")
	t.Setenv("SYNTELLE_APP_ORIGIN", "https://app.syntelle.online")
	t.Setenv("SYNTELLE_LOCAL_OWNER", "0")
	t.Setenv("SYNTELLE_STREAM_TIME_LIMITS_ENABLED", "true")
	t.Setenv("SYNTELLE_STREAM_SESSION_LIMIT_ENABLED", "true")
	t.Setenv("SYNTELLE_TURN_URLS", "turns:relay.fixture:443?transport=tcp")
	for _, key := range []string{"SYNTELLE_EDGE_TOKEN", "SYNTELLE_PROCESSOR_TOKEN", "SYNTELLE_RENDER_WORKER_TOKEN", "SYNTELLE_MODAL_LAUNCH_TOKEN", "SYNTELLE_TURN_SECRET"} {
		t.Setenv(key, strings.Repeat("s", 32))
	}
	if err := ValidateDeployment(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"SYNTELLE_MODAL_LAUNCH_TOKEN", "SYNTELLE_PROCESSOR_TOKEN"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "")
			if ValidateDeployment() == nil {
				t.Fatal("incomplete cloud configuration accepted")
			}
		})
	}
	t.Run("TURN deferred", func(t *testing.T) {
		t.Setenv("SYNTELLE_TURN_URLS", "")
		t.Setenv("SYNTELLE_TURN_SECRET", "")
		if err := ValidateDeployment(); err != nil {
			t.Fatalf("cloud API should start without TURN: %v", err)
		}
	})
	t.Run("TURN URL without signing secret", func(t *testing.T) {
		t.Setenv("SYNTELLE_TURN_SECRET", "")
		if err := ValidateDeployment(); err == nil {
			t.Fatal("partial TURN configuration accepted")
		}
	})
}
