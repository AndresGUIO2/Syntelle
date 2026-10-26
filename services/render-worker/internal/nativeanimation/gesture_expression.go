package nativeanimation

type browMotion struct {
	initialized, active bool
	session             uint64
	at, from, value     float64
}

// Gate and phrase boundaries ease from the currently visible value. Between
// boundaries the prepared envelope is sampled directly on emitted audio time.
func (b *browMotion) sample(ms, target float64, active bool, session uint64, instant bool) float64 {
	if !b.initialized || instant {
		*b = browMotion{initialized: true, active: active, session: session, at: ms, from: target, value: target}
		return target
	}
	if b.active != active || b.session != session {
		b.from, b.at, b.active, b.session = b.value, ms, active, session
	}
	duration := 120.
	if !active {
		duration = 240
	}
	b.value = blend(b.from, target, ease((ms-b.at)/duration))
	return unit(b.value)
}
