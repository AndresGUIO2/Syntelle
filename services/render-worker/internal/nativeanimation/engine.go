package nativeanimation

import "math"

// Engine stores only per-session motion history. Its owner serializes Sample
// calls and supplies monotonic, pause-aware animation time. Audio position is
// independently supplied from emitted PCM samples; no timer advances speech.
type Engine struct {
	hadEmotionControl bool

	config      Config
	attention   attentionDirector
	blink       blinkClock
	expressions expressionDirector
	emotions    emotionDirector
	last        Frame
	lastMS      float64
	initialized bool
	lastSmile   float64
}

func NewEngine(c Config) *Engine {
	return &Engine{config: c, attention: newAttention(c.Seed), blink: newBlinkClock(c), expressions: newExpressions(), lastSmile: math.Inf(-1)}
}
func (e *Engine) Sample(animationMS float64, speech SpeechState, settings Settings) Frame {
	if settings.Paused && e.initialized {
		return e.last
	}
	// A new explicit clock after reset starts a new deterministic session.
	if e.initialized && animationMS < e.lastMS {
		replacement := NewEngine(e.config)
		replacement.lastSmile = e.lastSmile
		*e = *replacement
	}
	ms := math.Max(0, animationMS)
	c := e.config
	mouth := Mouth{"Rest", "A", 0}
	if speech.Active && speech.Speech != nil {
		mouth = MouthAt(speech.Speech.Timeline, speech.Position)
		if mouth.From == mouth.To {
			mouth.Amount = 0
			if mouth.From == "Rest" {
				mouth.To = "A"
			} else {
				mouth.To = "Rest"
			}
		}
	}
	moving := settings.Movement && !settings.ReducedMotion
	a := attention{}
	if moving {
		a = e.attention.sample(ms, speech, c)
		if c.MotionScale > 1 {
			a.Head.Yaw *= c.MotionScale
			a.Head.Pitch *= c.MotionScale
			a.Head.Roll *= c.MotionScale
		}
	}
	gaze := [2][2]float64{}
	if moving {
		gaze = gazeOffsets(c.Eyes, a.Head, a.Glance)
	}
	closure := e.blink.sample(ms, speech, settings.Blinking && !settings.ReducedMotion)
	if settings.EmotionControl != nil && settings.EmotionControl.Negative(settings.EmotionTime) {
		// Gate only automatic positive facial cues. Manual values are still
		// composed by the original expression path, including its speech limit.
		if plan := speech.plan(); plan != nil && plan.Performance != nil {
			copySpeech := *speech.Speech
			copyPlan := *plan
			copyPerformance := *plan.Performance
			copyPerformance.Resolved = nil
			for _, cue := range plan.Performance.Resolved {
				if cue.Kind != "smile" && cue.Kind != "laugh" {
					copyPerformance.Resolved = append(copyPerformance.Resolved, cue)
				}
			}
			copyPlan.Performance = &copyPerformance
			copySpeech.Plan = &copyPlan
			speech.Speech = &copySpeech
		}
	}
	busy := speech.Waiting && !c.SpeakingSmile || speech.Active && (speech.Speech == nil || speech.Position < speech.Speech.Duration)
	if !busy {
		closed := func(s string) bool { return s == "Rest" || s == "MBP" }
		busy = (mouth.Amount < 1 && !closed(mouth.From)) || (mouth.Amount > 0 && !closed(mouth.To))
	}
	ex := expression{}
	if c.Expressions {
		ex = e.expressions.sample(ms, expression{settings.Smile, settings.Brows}, settings.AutomaticExpressions && !settings.ReducedMotion, settings.ReducedMotion, speech, settings.Paused, busy, c)
	}
	if busy && !c.SpeakingSmile {
		ex.Smile = 0
	}
	laugh := 0.
	if c.LaughAvailable && speech.Active && speech.plan() != nil && speech.plan().Performance != nil && settings.AutomaticExpressions && !settings.ReducedMotion {
		for _, cue := range speech.plan().Performance.Resolved {
			if cue.Kind != "laugh" || cue.Amount == nil {
				continue
			}
			phase := (speech.Position - cue.Time) / cue.Duration
			if phase < 0 || phase >= 1 {
				continue
			}
			laugh = unit(*cue.Amount) * ease(phase/.15) * (1 - ease((phase-.7)/.3))
			if !c.ReferenceLaugh {
				ex.Smile = math.Max(ex.Smile, laugh)
			}

			// Laughter is a boundary action. Speech visemes cannot overwrite its
			// smile; the accepted audio drives small jaw releases instead.
			mouth = Mouth{"Rest", "A", laugh * (.015 + .045*speech.Speech.EnergyAt(speech.Position))}
			if c.ReferenceLaugh {
				mouth.Amount = speech.Speech.EnergyAt(speech.Position)
				ex.Smile = settings.Smile
			}
		}
	}
	breath := 0.0
	if moving {
		if c.Breathing != nil {
			breath = breathingAt(ms, c.Seed, c.Breathing)
		} else if c.BreathPeriod > 0 {
			breath = math.Sin(ms / c.BreathPeriod * 2 * math.Pi)
		}
		breath *= c.FaceHeight * c.BreathFraction
	}
	out := Frame{Head: a.Head, Breath: breath, Mouth: mouth, EyeClosure: closure, Gaze: gaze, Smile: ex.Smile, Brows: ex.Brows}
	out.Laugh = laugh
	if e.hadEmotionControl && settings.EmotionControl == nil {
		base := baseTarget(settings.EmotionBaseline)
		w := emotionWeights(base)
		e.emotions = emotionDirector{initialized: true, baseline: settings.EmotionBaseline.Normalized(), current: w, from: w, target: w, energy: .5, energyFrom: .5, energyTarget: .5, duration: 500, at: ms}
	}
	e.hadEmotionControl = settings.EmotionControl != nil
	if c.Emotions {
		if settings.EmotionControl != nil {
			out.Emotion, e.emotions.energy = settings.EmotionControl.Sample(settings.EmotionTime)
		} else {
			out.Emotion = e.emotions.sample(ms, speech, settings)
		}
		emotionStrength := 0.
		for _, w := range out.Emotion {
			emotionStrength += w
		}
		out.Brows *= 1 - .75*emotionStrength
		if emotionStrength > 0 {
			out.Smile = 1 - (1-out.Smile)*(1-emotionSmile(out.Emotion))
		}
		negative := 0.
		for _, index := range []int{5, 6, 7, 8, 9} {
			negative += out.Emotion[index]
		}
		out.Smile = math.Min(out.Smile, 1-.75*negative)
		if !speech.Active && !speech.Waiting && out.Laugh == 0 {
			opening := .4*out.Emotion[8] + .14*out.Emotion[7]
			if c.EmotionRestOpening != nil {
				opening = 0
				for i, weight := range out.Emotion {
					opening += weight * c.EmotionRestOpening[i]
				}
			}
			if opening > 0 {
				out.Mouth = Mouth{"Rest", "A", opening}
			}
		}
		if moving {
			strength := 0.
			for _, w := range out.Emotion {
				strength += w
			}
			gain := 1 + .3*(e.emotions.energy-.5)*strength
			out.Head.Yaw = math.Max(-c.Bounds.Yaw, math.Min(c.Bounds.Yaw, out.Head.Yaw*gain))
			out.Head.Pitch = math.Max(-c.Bounds.Pitch, math.Min(c.Bounds.Pitch, out.Head.Pitch*gain))
			out.Head.Roll = math.Max(-c.Bounds.Roll, math.Min(c.Bounds.Roll, out.Head.Roll*gain))
		}
	}
	e.last = out
	e.lastMS = ms
	e.initialized = true
	return out
}
