package conversation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOutputWaitsForSpaceAndPreservesOrder(t *testing.T) {
	out := make(chan Event, 1)
	out <- Event{Text: "first"}
	done := make(chan error, 1)
	go func() { done <- enqueueOutput(context.Background(), out, Event{Text: "second"}, time.Second) }()
	if event := <-out; event.Text != "first" {
		t.Fatal(event)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if event := <-out; event.Text != "second" {
		t.Fatal(event)
	}
}

func TestOutputCancellationAndCongestionAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cancel bool
		want   error
	}{{"producer cancelled", true, context.Canceled}, {"client congested", false, errOutputCongested}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			out := make(chan Event)
			done := make(chan error, 1)
			go func() { done <- enqueueOutput(ctx, out, Event{}, 30*time.Millisecond) }()
			if tc.cancel {
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Fatalf("got %v, want %v", err, tc.want)
				}
			case <-time.After(time.Second):
				t.Fatal("output wait exceeded its bound")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := make(chan Event, 1)
	if err := enqueueOutput(ctx, out, Event{}, time.Second); !errors.Is(err, context.Canceled) || len(out) != 0 {
		t.Fatal("cancelled producer queued another event", err)
	}
}
