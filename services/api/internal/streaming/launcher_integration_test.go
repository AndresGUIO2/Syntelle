//go:build integration

package streaming

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type launchTransport func(*http.Request) (*http.Response, error)

func (f launchTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPersistedStartupCoalescesAndSurvivesRestart(t *testing.T) {
	db := streamingTestStore(t)
	records, err := openStore(t.Context(), db.Pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYNTELLE_MODAL_LAUNCH_URL", "https://fixture.modal.run")
	t.Setenv("SYNTELLE_MODAL_LAUNCH_TOKEN", "offline-secret")
	s := &Service{store: records}
	var calls atomic.Int32
	client := &http.Client{Transport: launchTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		var count int
		if err := db.Pool.QueryRow(r.Context(), "SELECT count(*) FROM syntelle_streaming.worker_allocation").Scan(&count); err != nil || count != 1 {
			t.Error("launch preceded durable reservation")
		}
		s.mu.Lock()
		s.worker = &peer{}
		s.mu.Unlock()
		return &http.Response{StatusCode: 202, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	})}
	s.config.LaunchClient = client
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := s.ensureWorker(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate GPU purchase")
	}
	restarted := &Service{store: records, config: Config{LaunchClient: client}}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if restarted.ensureWorker(ctx) == nil {
		t.Fatal("orphan allocation unexpectedly ready")
	}
	if calls.Load() != 1 {
		t.Fatal("API restart repeated GPU purchase")
	}
}
