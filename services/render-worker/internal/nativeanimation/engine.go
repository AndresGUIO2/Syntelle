package nativeanimation

import (
	"math"
	"syntelle/render-contract"
)

// Engine stores only per-session motion history. Its owner serializes Sample
// calls and supplies monotonic, pause-aware animation time. Audio position is
// independently supplied from emitted PCM samples; no timer advances speech.
type Engine struct {
	laughter                        laughterPresentation
	jawSession                      uint64
	jawHandoffAt, jawHandoffFrom    float64
	jawHandoff                      bool
	jawWasActive                    bool
	jawReleaseStart, jawReleaseFrom float64
	accent                          accentMotion
	motionReply                     string
	motionWindow                    rendercontract.GestureWindow
	recoveryStart                   float64
	recoveryFrom                    Frame
	recovering                      bool
	laughBridge                     laughterBridge
	recoveryFaceFrom                float64
	hadEmotionControl               bool

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
	return &Engine{config: c, attention: newAttention(c.Seed), blink: newBlinkClock(c), expressions: newExpressions(), lastSmile: math.Inf(-1), laughBridge: laughterBridge{factor: 1}}
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
		a = e.attention.sample(ms, speech, c, settings)
		if settings.Activity == "listening" {
			a.Glance = [2]float64{}
			if c.PresenceVersion != PresenceVersion {
				a.Head.Yaw *= .65
				a.Head.Pitch *= .65
				a.Head.Roll *= .65
			}
		}
		if settings.Activity == "preparing" {
			a.Glance[0] *= .35
			a.Glance[1] *= .35
		}
		if c.MotionScale > 1 && c.PresenceVersion != PresenceVersion {
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
	if settings.EmotionControl != nil && settings.EmotionControl.Negative(settings.EmotionTime) && !(c.LaughterV2 && settings.ManualLaughter) {
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
	laugh := 0.
	laughing := false
	laughClosing, faceRelease := false, 1.
	if c.LaughAvailable && speech.Active && speech.plan() != nil && speech.plan().Performance != nil && (settings.AutomaticExpressions || c.LaughterV2 && settings.ManualLaughter) && !settings.ReducedMotion {
		for _, cue := range speech.plan().Performance.Resolved {
			if cue.Kind != "laugh" || cue.Amount == nil {
				continue
			}
			phase := (speech.Position - cue.Time) / cue.Duration
			if phase < 0 || phase >= 1 {
				continue
			}
			laughing = true
			faceRelease = 1 - ease((phase-.7)/.3)
			laughClosing = phase >= .7
			laugh = unit(*cue.Amount) * ease(phase/.15) * faceRelease
			if channels, ok := speech.Speech.LaughterAt(speech.Position); ok {
				laugh = unit(*cue.Amount) * channels.LaughFace
				last := speech.Speech.LaughterLastBurst()
				laughClosing = speech.Position >= last
				faceRelease = 1 - ease((speech.Position-last)/.24)
			} else if c.Mandible != nil && speech.Speech.articulation != nil {
				analysis := speech.Speech.articulation
				channels := speech.Speech.ArticulationAt(speech.Position)
				laugh = unit(*cue.Amount) * channels.LaughFace
				laughClosing = speech.Position >= analysis.lastBurst
				faceRelease = 1 - ease((speech.Position-analysis.lastBurst)/.24)
				if speech.Position >= analysis.lastBurst+.24 {
					laughClosing = false
				}
			}

			// Laughter is a boundary action. Speech visemes cannot overwrite its
			// smile; the accepted audio drives small jaw releases instead.
			mouth = Mouth{"Rest", "A", laugh * (.015 + .045*speech.Speech.EnergyAt(speech.Position))}
			if c.ReferenceLaugh {
				// Release the jaw with the face. Once the renderer quantizes the
				// laugh to zero, Amount belongs to ordinary articulation again;
				// carrying unscaled audio energy there causes a final A-pose jolt.
				mouth.Amount = speech.Speech.EnergyAt(speech.Position) * laugh
			}
		}
	}
	if c.Expressions {
		// Reference laughter has its own oral path. Do not silently halve the
		// manual baseline as for speech, then expose that half-smile at exit.
		mouthBusy := busy && !(c.ReferenceLaugh && laughing)
		ex = e.expressions.sample(ms, expression{settings.Smile, settings.Brows}, settings.AutomaticExpressions && !settings.ReducedMotion, settings.ReducedMotion, speech, settings.Paused, mouthBusy, c)
	}
	if busy && !c.SpeakingSmile {
		ex.Smile = 0
	}
	if laughing {
		if c.ReferenceLaugh {
			ex.Smile = settings.Smile
		} else {
			ex.Smile = math.Max(ex.Smile, laugh)
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
	if c.Mandible != nil && speech.Active && speech.Speech != nil {
		channels := speech.Speech.ArticulationAt(speech.Position)
		if laughChannels, ok := speech.Speech.LaughterAt(speech.Position); ok {
			channels = laughChannels
		}
		out.JawOpen, out.LipOpening, out.LipTension = channels.JawOpen, channels.LipOpening, channels.LipTension
		if e.jawSession != speech.Session {
			e.jawHandoff = e.jawSession != 0 && e.last.JawOpen > 0 && (speech.Speech.articulation == nil || !speech.Speech.articulation.laugh)
			e.jawSession = speech.Session
			e.jawHandoffAt, e.jawHandoffFrom = ms, e.last.JawOpen
		}
		// Only the gross mandible bridges adjacent phrase boundaries. Lip
		// arrivals and contact residuals still use the new PCM's exact clock.
		if e.jawHandoff {
			phase := unit((ms - e.jawHandoffAt) / 40)
			out.JawOpen = blend(e.jawHandoffFrom, out.JawOpen, ease(phase))
			e.jawHandoff = phase < 1
		}

		e.jawWasActive = true
		if speech.Speech.articulation != nil && speech.Speech.articulation.laugh && !laughing {
			out.JawOpen = 0
		}
	} else if c.Mandible != nil {
		if e.jawWasActive {
			e.jawWasActive = false
			e.jawReleaseStart = ms
			e.jawReleaseFrom = e.last.JawOpen
		}
		out.JawOpen = e.jawReleaseFrom * (1 - ease((ms-e.jawReleaseStart)/80))
	}
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
		if moving && (c.PresenceVersion != PresenceVersion || speech.Preview) {
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
	if coordinatedSpeech(speech) || e.accent.value > 0 {
		accent := e.accent.sample(ms, browAccent(speech), speech.Active, speech.Session, settings.ReducedMotion || !settings.AutomaticExpressions)
		out.Brows = unit(1 - (1-out.Brows)*(1-accent))
	}
	if moving && (coordinatedSpeech(speech) || settings.Activity != "") {
		out.Gaze = gazeOffsets(c.Eyes, out.Head, a.Glance)
	}
	if c.OralRecovery && !c.LaughterV2 {
		if out.Laugh > 0 || speech.Active || settings.ReducedMotion || !settings.AutomaticExpressions {
			e.recovering = false
		} else if !e.recovering && e.last.Laugh > 0 {
			e.recovering = true
			e.recoveryStart = ms
			e.recoveryFrom = e.last
			e.recoveryFaceFrom = e.laughBridge.factor
			// Match the prepared renderer's visible reference step at handoff.
			e.recoveryFrom.Laugh = math.Round(e.recoveryFrom.Laugh*120) / 120
		}
		if e.recovering {
			weight := 1 - ease((ms-e.recoveryStart)/240)
			out.Laugh = e.recoveryFrom.Laugh * weight
			out.JawOpen = e.recoveryFrom.JawOpen * weight
			out.LipOpening = e.recoveryFrom.LipOpening * weight
			out.LipTension = e.recoveryFrom.LipTension * weight
			if weight > 0 {
				target := out.Mouth
				out.Mouth = e.recoveryFrom.Mouth
				if target.From == out.Mouth.From && target.To == out.Mouth.To {
					out.Mouth.Amount = blend(target.Amount, out.Mouth.Amount, weight)
				} else {
					out.Mouth.Amount *= weight
				}
				if out.Laugh > 0 {
					laughClosing, faceRelease = true, e.recoveryFaceFrom*weight
				}
			}
			if weight == 0 {
				e.recovering = false
			}
		}
	}
	face := e.laughBridge.sample(ms, laughClosing, faceRelease, out.Emotion != (rendercontract.EmotionWeights{}), c.ReferenceLaugh && c.OralRecovery && c.Emotions && !c.LaughterV2 && !settings.ReducedMotion)
	if face < 1 {
		for i := range out.Emotion {
			out.Emotion[i] *= face
		}
		out.Smile *= face
		if !speech.Active {
			out.Mouth.Amount *= face
		}
	}
	if c.LaughterV2 {
		e.presentLaughter(ms, speech, settings, &out)
	}
	e.last = out
	e.lastMS = ms
	e.initialized = true
	return out
}
