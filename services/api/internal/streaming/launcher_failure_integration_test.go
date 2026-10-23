//go:build integration

package streaming

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestUncertainLaunchDoesNotRetry(t *testing.T) {
	db := streamingTestStore(t)
	records, err := openStore(t.Context(), db.Pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYNTELLE_MODAL_LAUNCH_URL", "https://fixture.modal.run")
	var calls atomic.Int32
	s := &Service{store: records, config: Config{LaunchClient: &http.Client{Transport: launchTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("offline ambiguous result")
	})}}}
	if s.ensureWorker(t.Context()) == nil {
		t.Fatal("ambiguous failure accepted")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if s.ensureWorker(ctx) == nil {
		t.Fatal("unknown worker accepted")
	}
	if calls.Load() != 1 {
		t.Fatal("uncertain allocation automatically retried")
	}
}
