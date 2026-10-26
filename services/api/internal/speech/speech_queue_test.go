package speech

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type queuedSpeechFixture struct {
	gate         synthesisGate
	entered      chan string
	started      chan string
	releaseFirst <-chan struct{}
	calls        atomic.Int32
}

func (p *queuedSpeechFixture) Synthesize(context.Context, Input) (Result, error) {
	return Result{}, errors.New("queued provider bypassed")
}

func (p *queuedSpeechFixture) SynthesizeQueued(ctx context.Context, input Input, timeout time.Duration) (Result, error) {
	p.entered <- input.RequestKey
	release, err := p.gate.acquire(ctx)
	if err != nil {
		return Result{}, err
	}
	defer release()
	call, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	p.calls.Add(1)
	p.started <- input.RequestKey
	if input.RequestKey == "first-key" {
		select {
		case <-p.releaseFirst:
		case <-call.Done():
			return Result{}, call.Err()
		}
	}
	if err := call.Err(); err != nil {
		return Result{}, err
	}
	return Result{Audio: make([]byte, 44), Mime: "audio/wav", DurationMS: 1000, Visemes: []Viseme{{0, 0}, {2, 100}, {0, 900}}}, nil
}

type queuedSpeechLedger struct {
	next     atomic.Int64
	finished chan string
}

func (l *queuedSpeechLedger) Reserve(context.Context, Input, string, int) (int64, *Result, error) {
	return l.next.Add(1), nil, nil
}

func (l *queuedSpeechLedger) Finish(_ context.Context, _ int64, _ *Result, message string, _ bool) error {
	l.finished <- message
	return nil
}

func queuedInput(key string) Input {
	input := validSpeechInput()
	input.RequestKey = key
	return input
}

func TestConfiguredSpeechConcurrency(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
	}{
		{"", 8}, {"1", 1}, {"8", 8}, {"32", 32}, {"0", 0}, {"33", 0}, {"invalid", 0},
	} {
		t.Run(fmt.Sprintf("value_%q", test.value), func(t *testing.T) {
			got, err := configuredSpeechConcurrency(test.value)
			if got != test.want || (err != nil) != (test.want == 0) {
				t.Fatalf("concurrency = %d, error = %v; want %d", got, err, test.want)
			}
		})
	}
}

func waitQueueEvent(t *testing.T, ctx context.Context, events <-chan string, expected string) {
	t.Helper()
	select {
	case got := <-events:
		if got != expected {
			t.Fatalf("queue event = %q, want %q", got, expected)
		}
	case <-ctx.Done():
		t.Fatal("queued speech did not progress")
	}
}

func TestSpeechQueuedProviderGetsFullDeadlineAfterAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	firstRelease := make(chan struct{})
	provider := &queuedSpeechFixture{gate: newSynthesisGate(1), entered: make(chan string, 2), started: make(chan string, 2), releaseFirst: firstRelease}
	ledger := &queuedSpeechLedger{finished: make(chan string, 2)}
	service := NewService(provider, ledger, "canadacentral")
	first := make(chan error, 1)
	go func() {
		_, err := service.synthesizeBounded(ctx, queuedInput("first-key"), 500, time.Second)
		first <- err
	}()
	waitQueueEvent(t, ctx, provider.entered, "first-key")
	waitQueueEvent(t, ctx, provider.started, "first-key")
	second := make(chan error, 1)
	go func() {
		_, err := service.synthesizeBounded(ctx, queuedInput("second-key"), 500, 20*time.Millisecond)
		second <- err
	}()
	waitQueueEvent(t, ctx, provider.entered, "second-key")
	// The second request waits longer than its provider deadline, but must still
	// receive that entire deadline once the first call releases the slot.
	time.Sleep(40 * time.Millisecond)
	close(firstRelease)
	waitQueueEvent(t, ctx, provider.started, "second-key")
	for _, done := range []<-chan error{first, second} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("queued synthesis did not complete")
		}
	}
	if provider.calls.Load() != 2 {
		t.Fatal("queued provider call was lost or repeated")
	}
}

func TestSpeechQueuedProviderCancellationSkipsPaidCall(t *testing.T) {
	provider := &queuedSpeechFixture{gate: newSynthesisGate(1), entered: make(chan string, 1), started: make(chan string, 1)}
	release, err := provider.gate.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ledger := &queuedSpeechLedger{finished: make(chan string, 1)}
	service := NewService(provider, ledger, "canadacentral")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.synthesizeBounded(ctx, queuedInput("second-key"), 500, time.Second)
		done <- err
	}()
	limit, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	waitQueueEvent(t, limit, provider.entered, "second-key")
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled queued speech was accepted")
		}
	case <-limit.Done():
		t.Fatal("cancelled speech remained stuck behind the active call")
	}
	if provider.calls.Load() != 0 {
		t.Fatal("cancelled queued speech reached the provider")
	}
	waitQueueEvent(t, limit, ledger.finished, "Speech request cancelled.")
}

type burstSpeechFixture struct {
	gate    synthesisGate
	started chan struct{}
	release <-chan struct{}
	active  atomic.Int32
	peak    atomic.Int32
	calls   atomic.Int32
}

func (p *burstSpeechFixture) Synthesize(context.Context, Input) (Result, error) {
	return Result{}, errors.New("queued provider bypassed")
}

func (p *burstSpeechFixture) SynthesizeQueued(ctx context.Context, _ Input, timeout time.Duration) (Result, error) {
	release, err := p.gate.acquire(ctx)
	if err != nil {
		return Result{}, err
	}
	defer release()
	call, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	active := p.active.Add(1)
	defer p.active.Add(-1)
	for peak := p.peak.Load(); active > peak; peak = p.peak.Load() {
		if p.peak.CompareAndSwap(peak, active) {
			break
		}
	}
	p.calls.Add(1)
	p.started <- struct{}{}
	select {
	case <-p.release:
	case <-call.Done():
		return Result{}, call.Err()
	}
	return Result{Audio: make([]byte, 44), Mime: "audio/wav", DurationMS: 1000, Visemes: []Viseme{{0, 0}, {2, 100}, {0, 900}}}, nil
}

func TestSpeechQueueServesFiftyActiveUsersWithEightProviderCalls(t *testing.T) {
	const requests, parallel = 50, 8
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	release := make(chan struct{})
	provider := &burstSpeechFixture{gate: newSynthesisGate(parallel), started: make(chan struct{}, requests), release: release}
	ledger := &queuedSpeechLedger{finished: make(chan string, requests)}
	service := NewService(provider, ledger, "canadacentral")
	var workers sync.WaitGroup
	results := make(chan error, requests)
	for i := range requests {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := service.synthesizeBounded(ctx, queuedInput(fmt.Sprintf("burst-%03d", i)), 500, time.Second)
			results <- err
		}()
	}
	for range parallel {
		select {
		case <-provider.started:
		case <-ctx.Done():
			t.Fatal("parallel provider calls did not start")
		}
	}
	close(release)
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("queued requests did not drain")
	}
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if provider.calls.Load() != requests || provider.peak.Load() != parallel || len(ledger.finished) != requests {
		t.Fatalf("burst calls=%d peak=%d archived=%d", provider.calls.Load(), provider.peak.Load(), len(ledger.finished))
	}
}
