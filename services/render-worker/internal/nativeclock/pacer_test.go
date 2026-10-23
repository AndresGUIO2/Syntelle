package nativeclock

import (
	"context"
	"testing"
	"time"
)

func TestPacerRealDeadlinesAndCancellation(t *testing.T) {
	p := NewPacer(2 * time.Millisecond)
	defer p.Close()
	start := Now()
	var previous time.Duration
	for range 6 {
		at, ok := p.Next(context.Background())
		if !ok || at <= previous || Now() < at {
			t.Fatal("deadline ordering violated")
		}
		previous = at
	}
	if Now()-start < 12*time.Millisecond {
		t.Fatal("frames ran before their raw deadlines")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p.WaitUntil(ctx, Now()+time.Hour) {
		t.Fatal("cancelled wait continued")
	}
}
