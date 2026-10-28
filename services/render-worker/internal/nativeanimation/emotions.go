package nativeanimation

import "syntelle/render-contract"

type emotionDirector struct {
	baseline                                   rendercontract.EmotionBaseline
	current, from, target                      rendercontract.EmotionWeights
	at                                         float64
	session                                    uint64
	initialized                                bool
	suppressed                                 string
	energy, energyFrom, energyTarget, duration float64
}

func emotionWeights(v rendercontract.EmotionTarget) (out rendercontract.EmotionWeights) {
	if v.Valid() {
		if i := rendercontract.EmotionIndex(v.State); i > 0 {
			out[i] = v.Intensity
		}
	}
	return
}

func (d *emotionDirector) sample(ms float64, s SpeechState, settings Settings) rendercontract.EmotionWeights {
	baseline := settings.EmotionBaseline.Normalized()
	base := emotionWeights(rendercontract.EmotionTarget{State: baseline.State, Intensity: baseline.Intensity, Energy: .5})
	duration := 500.
	energy := d.energyTarget
	if !d.initialized {
		d.baseline = baseline
		d.target = base
		d.initialized = true
		d.energy = .5
		d.energyFrom = .5
		d.energyTarget = .5
		d.duration = 500
		energy = .5
		d.at = ms
	}
	target := d.target
	if baseline != d.baseline {
		d.baseline = baseline
		target = base
		energy = .5
		d.suppressed = s.ReplyID
	}
	if !settings.AutomaticExpressions || settings.ReducedMotion || !s.Active && !s.Waiting {
		target = base
		energy = .5
		duration = 900
	} else if s.Active && s.Session != d.session && (d.suppressed == "" || d.suppressed != s.ReplyID) {
		if p := s.plan(); p != nil && p.Performance != nil && p.Performance.Intent != nil && rendercontract.NaturalPerformance(p.Performance.Intent.Version) {
			if v := p.Performance.Intent.Emotion; v != nil && v.Valid() {
				target = emotionWeights(*v)
				energy = v.Energy
			}
		}
	}
	d.session = s.Session
	if target != d.target || energy != d.energyTarget {
		d.energyFrom = d.energy
		d.energyTarget = energy
		d.duration = duration
		d.from = d.current
		d.target = target
		d.at = ms
	}
	t := ease((ms - d.at) / d.duration)
	if settings.ReducedMotion {
		t = 1
	}
	d.energy = blend(d.energyFrom, d.energyTarget, t)
	for i := range d.current {
		d.current[i] = blend(d.from[i], d.target[i], t)
	}
	return d.current
}

func emotionSmile(w rendercontract.EmotionWeights) float64 {
	factors := [10]float64{0, 1, .85, .55, .12, 0, 0, 0, 0, 0}
	value := 0.
	for i, v := range w {
		value += v * factors[i]
	}
	return unit(value)
}
