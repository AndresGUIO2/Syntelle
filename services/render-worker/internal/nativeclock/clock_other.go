//go:build !linux

package nativeclock

import "time"

var origin = time.Now()

func Now() time.Duration { return time.Since(origin) }
func timerRate() float64 { return 1 }
