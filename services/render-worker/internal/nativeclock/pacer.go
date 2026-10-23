// Package nativeclock owns bounded, cancellable media deadlines without an
// additional goroutine or pending frame queue.
package nativeclock

import (
	"context"
	"time"
)

type Pacer struct {
	interval, next time.Duration
	timer          *time.Timer
	rate           float64
	refresh        time.Duration
}

func NewPacer(interval time.Duration) *Pacer {
	t := time.NewTimer(time.Hour)
	t.Stop()
	now := Now()
	return &Pacer{interval: interval, next: now + interval, timer: t, rate: timerRate(), refresh: now + time.Second}
}

func (p *Pacer) Close() { p.timer.Stop() }

func (p *Pacer) WaitUntil(ctx context.Context, deadline time.Duration) bool {
	for {
		if ctx.Err() != nil {
			return false
		}
		now := Now()
		remaining := deadline - now
		if remaining <= 0 {
			return true
		}
		if now >= p.refresh {
			p.rate = timerRate()
			p.refresh = now + time.Second
		}
		delay := max(50*time.Microsecond, time.Duration(float64(remaining)*p.rate))
		p.timer.Reset(delay)
		select {
		case <-ctx.Done():
			return false
		case <-p.timer.C:
		}
	}
}

func (p *Pacer) Next(ctx context.Context) (time.Duration, bool) {
	at := p.next
	if now := Now(); now-at >= p.interval {
		at = now
	} // Skip obsolete deadlines, never queue.
	p.next = at + p.interval
	return at, p.WaitUntil(ctx, at)
}
