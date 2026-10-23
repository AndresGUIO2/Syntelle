package main

import "time"

// This state is owned by the single control-read loop; the timer only cancels
// its context. No timer callback reads or changes session state.
type cloudLifecycle struct {
	active map[string]bool
	timer  *time.Timer
	idle   time.Duration
}

func newCloudLifecycle(enabled bool, cancel func(), grace, idle time.Duration) *cloudLifecycle {
	l := &cloudLifecycle{active: map[string]bool{}, idle: idle}
	if enabled {
		l.timer = time.AfterFunc(grace, cancel)
	}
	return l
}
func (l *cloudLifecycle) observe(kind, id string) {
	if l.timer == nil {
		return
	}
	if kind == "create" {
		l.active[id] = true
		l.timer.Stop()
	}
	if kind == "stop" {
		delete(l.active, id)
		if len(l.active) == 0 {
			l.timer.Reset(l.idle)
		}
	}
}
func (l *cloudLifecycle) close() {
	if l.timer != nil {
		l.timer.Stop()
	}
}
