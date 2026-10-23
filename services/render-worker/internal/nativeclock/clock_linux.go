//go:build linux

package nativeclock

import (
	"golang.org/x/sys/unix"
	"time"
)

var fallbackStart = time.Now()
var rawOrigin = func() time.Duration {
	var ts unix.Timespec
	if unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts) != nil {
		return 0
	}
	return time.Duration(ts.Nano())
}()

// Now is unaffected by NTP/adjtimex frequency slewing. Speech sample rate,
// frame scheduling and interpolation must all measure the same real second.
func Now() time.Duration {
	var ts unix.Timespec
	if rawOrigin != 0 && unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts) == nil {
		return time.Duration(ts.Nano())
	}
	return rawOrigin + time.Since(fallbackStart)
}

// Go timers use the adjusted monotonic clock. Translate raw deadlines to that
// clock's current rate; do not change kernel time or increase the media FPS.
func timerRate() float64 {
	if rawOrigin == 0 {
		return 1
	}
	var tx unix.Timex
	if _, err := unix.Adjtimex(&tx); err != nil {
		return 1
	}
	// Linux ADJ_TICK is microseconds per USER_HZ=100 tick on this platform.
	rate := float64(tx.Tick)/10000 + float64(tx.Freq)/(65536*1e6)
	if rate < .8 || rate > 1.2 {
		return 1
	}
	return rate
}
