//go:build integration

package streaming

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

func TestCloudStartupFailureAndCancellationReleaseAdmission(t *testing.T) {
	for _, stopEarly := range []bool{false, true} {
		t.Run(map[bool]string{false: "launch rejected", true: "visitor cancelled"}[stopEarly], func(t *testing.T) {
			db := streamingTestStore(t)
			t.Setenv("SYNTELLE_DEPLOYMENT_MODE", "cloud")
			t.Setenv("SYNTELLE_MODAL_LAUNCH_URL", "https://fixture.modal.run")
			t.Setenv("SYNTELLE_TURN_SECRET", strings.Repeat("x", 32))
			t.Setenv("SYNTELLE_TURN_URLS", "turns:fixture:443")
			called, outcome := make(chan struct{}), make(chan struct{})
			client := &http.Client{Transport: launchTransport(func(r *http.Request) (*http.Response, error) {
				close(called)
				select {
				case <-r.Context().Done():
					return nil, r.Context().Err()
				case <-outcome:
					return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("offline failure"))}, nil
				}
			})}
			conversations := conversation.New(db, nil, nil, nil)
			service, err := Open(t.Context(), db.Pool, conversations, func(http.ResponseWriter, *http.Request, bool) (accounts.Principal, error) {
				return accounts.Principal{}, nil
			}, Config{MaximumSessions: 2, PackageHash: strings.Repeat("a", 64), LaunchClient: client})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if err := service.Shutdown(ctx); err != nil {
					t.Error(err)
				}
				if err := conversations.Shutdown(ctx); err != nil {
					t.Error(err)
				}
			})
			principal := accounts.Principal{ID: "guest:deployment", Kind: "guest", SessionID: "fixture-browser"}
			response := httptest.NewRecorder()
			service.create(response, httptest.NewRequest("POST", "/api/v1/streams", strings.NewReader("{}")), principal)
			var session SessionResponse
			if response.Code != 201 || json.Unmarshal(response.Body.Bytes(), &session) != nil || session.State != "starting" {
				t.Fatalf("expected admitted starting stream: %d %s", response.Code, response.Body)
			}
			select {
			case <-called:
			case <-time.After(2 * time.Second):
				t.Fatal("launcher was not called")
			}
			duplicate := httptest.NewRecorder()
			service.create(duplicate, httptest.NewRequest("POST", "/api/v1/streams", strings.NewReader("{}")), principal)
			if duplicate.Code != 429 {
				t.Fatal("same visitor acquired simultaneous admission")
			}
			if stopEarly {
				service.closeSession(session.ID, "stopped")
			} else {
				close(outcome)
			}
			deadline := time.Now().Add(2 * time.Second)
			for {
				var closed bool
				if err := db.Pool.QueryRow(t.Context(), "SELECT closed_at IS NOT NULL AND lease_ms=0 FROM syntelle_streaming.sessions WHERE id=$1", session.ID).Scan(&closed); err != nil {
					t.Fatal(err)
				}
				service.mu.Lock()
				_, active := service.sessions[session.ID]
				service.mu.Unlock()
				if closed && !active {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("startup did not release durable and in-memory admission")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
