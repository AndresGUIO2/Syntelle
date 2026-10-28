package nativeanimation

import (
	"math"
	"syntelle/render-contract"
)

type expression struct{ Smile, Brows float64 }
type expressionCue struct{ time, duration, smile, brows float64 }

func planExpressions(plan *Plan, seed uint32, directed bool, expressive ...bool) []expressionCue {
	cues := []expressionCue{}
	if plan == nil {
		return cues
	}
	if directed {
		if plan.Performance != nil {
			for _, c := range plan.Performance.Resolved {
				if c.Kind == "smile" {
					cues = append(cues, expressionCue{c.Time, c.Duration, .4, 0})
				} else if c.Kind == "emphasis" {
					amount := .3
					if len(expressive) > 0 && expressive[0] {
						amount = .5
					}
					cues = append(cues, expressionCue{c.Time, c.Duration, 0, amount})
				}
			}
		}
		return cues
	}
	r := random{seed ^ 0x46e8}
	for _, cue := range plan.Cues {
		if cue.Kind == "onset" {
			cues = append(cues, expressionCue{cue.Time + .1, 2.1, 0, .22})
		} else if cue.Kind == "emphasis" || cue.Kind == "pause" {
			if r.next() >= .45 {
				continue
			}
			near := false
			for _, other := range cues {
				if math.Abs(other.time-cue.Time) < 3.2 {
					near = true
				}
			}
			if !near {
				cues = append(cues, expressionCue{math.Max(0, cue.Time-.12), 1.5, 0, .35 + .15*cue.Strength})
			}
		}
	}
	return cues
}
func expressionCueAt(cues []expressionCue, position float64) expression {
	out := expression{}
	for _, cue := range cues {
		t := (position - cue.time) / cue.duration
		if t < 0 || t >= 1 {
			continue
		}
		weight := 1.0
		if t < .3 {
			weight = ease(t / .3)
		} else if t >= .6 {
			weight = 1 - ease((t-.6)/.4)
		}
		out.Smile = math.Max(out.Smile, cue.smile*weight)
		out.Brows = math.Max(out.Brows, cue.brows*weight)
	}
	return out
}

type expressionDirector struct {
	naturalBrows                          bool
	browMotion                            browMotion
	v2Seen                                bool
	cachedPlan                            *Plan
	cues                                  []expressionCue
	manualFrom, manualGoal, manualCurrent expression
	manualAt                              float64
	previousAuto, releaseFrom             expression
	releaseAt                             float64
	wasActive                             bool
	previousSession                       uint64
	wasSpeaking                           bool
	completionAt                          float64
	v2Current                             expression
	v2LastMS                              float64
}

func newExpressions() expressionDirector {
	return expressionDirector{previousSession: ^uint64(0), completionAt: math.Inf(-1)}
}
func (d *expressionDirector) sample(ms float64, manual expression, automatic, reduced bool, speech SpeechState, instantManual, mouthBusy bool, c Config) expression {
	plan := speech.plan()
	if !c.Directed && d.wasSpeaking && !speech.Active && plan != nil && speech.Position >= plan.Duration-.15 {
		d.completionAt = ms
	}
	if speech.Active || mouthBusy || !automatic || reduced {
		d.completionAt = math.Inf(-1)
	}
	d.wasSpeaking = speech.Active
	since := ms - d.completionAt
	completionSmile := 0.0
	if automatic && c.SmileAvailable && !reduced && !mouthBusy {
		completionSmile = .4 * ease(since/300) * (1 - ease((since-800)/900))
	}
	goal := expression{Brows: unit(manual.Brows)}
	if c.SmileAvailable && (!mouthBusy || c.SpeakingSmile) {
		goal.Smile = unit(manual.Smile)
		if mouthBusy {
			goal.Smile *= .5
		}
	}
	if goal != d.manualGoal {
		d.manualFrom = d.manualCurrent
		d.manualGoal = goal
		d.manualAt = ms
	}
	t := 1.0
	if !reduced && !instantManual {
		t = ease((ms - d.manualAt) / 240)
	}
	d.manualCurrent = expression{blend(d.manualFrom.Smile, goal.Smile, t), blend(d.manualFrom.Brows, goal.Brows, t)}
	active := automatic && !reduced && speech.Active
	if d.wasActive != active || d.previousSession != speech.Session {
		d.releaseFrom = d.previousAuto
		d.releaseAt = ms
	}
	if d.previousSession != speech.Session || plan != d.cachedPlan {
		d.cues = planExpressions(plan, c.Seed, c.Directed, c.Expressive)
		d.cachedPlan = plan
		d.previousSession = speech.Session
	}
	release := 1 - ease((ms-d.releaseAt)/900)
	target := expression{}
	if active {
		target = expressionCueAt(d.cues, speech.Position)
	}
	d.previousAuto = expression{}
	if !reduced {
		d.previousAuto = expression{d.releaseFrom.Smile*release + target.Smile*(1-release), d.releaseFrom.Brows*release + target.Brows*(1-release)}
	}
	d.wasActive = active
	autoSmile := completionSmile
	if c.Directed && !mouthBusy && c.SmileAvailable {
		autoSmile = d.previousAuto.Smile
	}
	result := expression{1 - (1-d.manualCurrent.Smile)*(1-autoSmile), 1 - (1-d.manualCurrent.Brows)*(1-d.previousAuto.Brows)}
	if plan != nil && plan.Performance != nil && plan.Performance.Intent != nil {
		d.v2Seen = rendercontract.DirectedPerformance(plan.Performance.Intent.Version)
		d.naturalBrows = rendercontract.NaturalPerformance(plan.Performance.Intent.Version)
	}
	if d.v2Seen {
		target := d.manualCurrent
		if active && plan != nil && plan.Performance != nil {
			for _, cue := range plan.Performance.Resolved {
				if cue.Amount == nil || cue.Duration <= 0 {
					continue
				}
				phase := (speech.Position - cue.Time) / cue.Duration
				if phase < 0 || phase >= 1 {
					continue
				}
				weight := ease(phase/.25) * (1 - ease((phase-.65)/.35))
				if d.naturalBrows && (cue.Kind == "brows" || cue.Kind == "emphasis") {
					weight = ease(phase/.28) * (1 - ease((phase-.42)/.58))
				}
				if cue.Kind == "brows" || cue.Kind == "emphasis" {
					if coordinatedSpeech(speech) {
						continue
					}
					amount := *cue.Amount
					if cue.Kind == "emphasis" {
						amount *= .5
					}
					target.Brows = blend(target.Brows, amount, weight)
				}
				if cue.Kind == "smile" && c.SmileAvailable && (!mouthBusy || c.SpeakingSmile) {
					amount := *cue.Amount
					if mouthBusy {
						amount *= .5
					}
					target.Smile = blend(target.Smile, amount, weight)
				}
			}
		}
		dt := math.Max(0, ms-d.v2LastMS)
		d.v2LastMS = ms
		weight := 1 - math.Exp(-dt/120)
		if reduced || instantManual {
			weight = 1
		}
		d.v2Current = expression{blend(d.v2Current.Smile, target.Smile, weight), blend(d.v2Current.Brows, target.Brows, weight)}
		result = d.v2Current
		if d.naturalBrows {
			// An analytic audio-clock envelope already supplies smooth motion;
			// filtering it a second time delays the visible speech accent.
			result.Brows = d.browMotion.sample(ms, target.Brows, active, speech.Session, reduced || instantManual)
			d.v2Current.Brows = result.Brows
		}
	}
	return result
}
