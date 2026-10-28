package nativeanimation

// Reference laughter and ordinary emotional articulation share the neutral
// endpoint. Cross that endpoint continuously without changing the selected
// emotion/controller target or fading canonical dental material.
type laughterBridge struct {
	active, returning bool
	factor, from, at  float64
}

func (d *laughterBridge) sample(ms float64, closing bool, release float64, hasEmotion, enabled bool) float64 {
	if !enabled {
		*d = laughterBridge{factor: 1}
		return 1
	}
	if closing && (hasEmotion || d.active) {
		d.active, d.returning, d.factor = true, false, unit(release)
		return d.factor
	}
	if d.active {
		if !d.returning {
			d.returning, d.from, d.at = true, d.factor, ms
		}
		phase := ease((ms - d.at) / 320)
		d.factor = blend(d.from, 1, phase)
		if phase == 1 {
			d.active = false
		}
		return d.factor
	}
	d.factor = 1
	return 1
}
