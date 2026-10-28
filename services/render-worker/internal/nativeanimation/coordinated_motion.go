package nativeanimation

import (
	"math"
	"syntelle/render-contract"
)

func coordinatedSpeech(s SpeechState) bool {
	p := s.plan()
	return p != nil && p.Performance != nil && p.Performance.Intent != nil && p.Performance.Intent.Version == rendercontract.CoordinatedPerformanceVersion
}

func semanticFocus(s SpeechState) float64 {
	if !s.Active || !coordinatedSpeech(s) {
		return 0
	}
	weight := 0.
	for _, cue := range s.plan().Performance.Resolved {
		if (!isHeadCue(cue.Kind) && cue.Kind != "brows") || cue.Source == "acoustic" {
			continue
		}
		start := cue.Time - .15
		peak := cue.Time + rendercontract.NaturalGestureLead(cue.Kind, cue.Duration)
		w := ease((s.Position-start)/math.Max(.15, peak-start)) * (1 - ease((s.Position-cue.Time-cue.Duration)/.32))
		weight = math.Max(weight, w)
	}
	return unit(weight)
}

func browAccent(s SpeechState) float64 {
	if !s.Active || !coordinatedSpeech(s) {
		return 0
	}
	result := 0.
	for _, cue := range s.plan().Performance.Resolved {
		if cue.Amount == nil || (cue.Kind != "brows" && cue.Kind != "emphasis") {
			continue
		}
		peak := cue.Time + rendercontract.NaturalGestureLead(cue.Kind, cue.Duration)
		weight := ease((s.Position-peak+.15)/.15) * (1 - ease((s.Position-peak)/.32))
		amount := *cue.Amount
		if cue.Kind == "emphasis" {
			amount *= .5
		}
		result = math.Max(result, unit(amount)*weight)
	}
	return result
}

// Keeps a final accent's release continuous across phrase/idle boundaries.
type accentMotion struct {
	value, from, at float64
	session         uint64
	active          bool
}

func (a *accentMotion) sample(ms, target float64, active bool, session uint64, instant bool) float64 {
	if instant {
		*a = accentMotion{}
		return 0
	}
	if a.active != active || a.session != session {
		a.from, a.at, a.active, a.session = a.value, ms, active, session
	}
	residual := a.from * (1 - ease((ms-a.at)/320))
	if active {
		a.value = unit(target + residual)
	} else {
		a.value = unit(residual)
	}
	return a.value
}
