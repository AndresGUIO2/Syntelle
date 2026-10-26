//go:build integration

package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloudWorkersBindTwoSessionsPerDurableAllocation(t *testing.T) {
	db := streamingTestStore(t)
	records, err := openStore(t.Context(), db.Pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYNTELLE_MODAL_LAUNCH_URL", "https://fixture.modal.run")
	t.Setenv("SYNTELLE_MODAL_LAUNCH_TOKEN", "offline-secret")
	s := &Service{store: records, sessions: map[string]*streamSession{}, workers: map[string]*cloudWorker{}}
	var launches atomic.Int32
	s.config.LaunchClient = &http.Client{Transport: launchTransport(func(r *http.Request) (*http.Response, error) {
		launches.Add(1)
		var request struct {
			AllocationID string `json:"allocation_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		var reserved bool
		if err := db.Pool.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM syntelle_streaming.worker_allocations WHERE id=$1)", request.AllocationID).Scan(&reserved); err != nil || !reserved {
			t.Error("launch preceded the durable allocation")
		}
		s.mu.Lock()
		s.workers[request.AllocationID] = &cloudWorker{peer: &peer{ctx: context.Background(), out: make(chan Envelope, 4)}, started: time.Now()}
		s.mu.Unlock()
		return &http.Response{StatusCode: 202, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	})}
	assigned := []string{}
	for _, id := range []string{"one", "two", "three"} {
		session := &streamSession{id: id}
		s.sessions[id] = session
		ctx, stop := context.WithTimeout(t.Context(), 2*time.Second)
		err := s.ensureCloudWorker(ctx, session)
		stop()
		if err != nil {
			t.Fatal(err)
		}
		assigned = append(assigned, session.workerID)
	}
	if launches.Load() != 2 || assigned[0] != assigned[1] || assigned[2] == assigned[0] {
		t.Fatalf("unexpected allocation grouping: launches=%d assignments=%v", launches.Load(), assigned)
	}
}

func TestUncertainCloudLaunchKeepsItsLease(t *testing.T) {
	db := streamingTestStore(t)
	records, err := openStore(t.Context(), db.Pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYNTELLE_MODAL_LAUNCH_URL", "https://fixture.modal.run")
	s := &Service{store: records, sessions: map[string]*streamSession{}, workers: map[string]*cloudWorker{}}
	var launches atomic.Int32
	s.config.LaunchClient = &http.Client{Transport: launchTransport(func(*http.Request) (*http.Response, error) {
		launches.Add(1)
		return nil, errors.New("offline uncertain result")
	})}
	first := &streamSession{id: "first"}
	s.sessions[first.id] = first
	if s.ensureCloudWorker(t.Context(), first) == nil {
		t.Fatal("uncertain launch accepted")
	}
	second := &streamSession{id: "second"}
	s.sessions[second.id] = second
	ctx, stop := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer stop()
	if s.ensureCloudWorker(ctx, second) == nil {
		t.Fatal("unconfirmed lease accepted")
	}
	if launches.Load() != 1 {
		t.Fatal("uncertain GPU invocation retried")
	}
}
