package nativeanimation

import "math"

type attention struct {
	Head   Head
	Glance [2]float64
}

func idleAttention(ms float64, seed uint32, expressive ...bool) attention {
	transition, holdRange := [2]float64{3000, 6000}, [2]float64{1000, 3000}
	if len(expressive) > 0 && expressive[0] {
		transition, holdRange = [2]float64{2500, 4500}, [2]float64{600, 1600}
	}
	if len(expressive) > 1 && expressive[1] {
		transition, holdRange = [2]float64{1800, 3600}, [2]float64{350, 1000}
	}
	r := random{seed ^ 0x51a7}
	from := Head{}
	start := 0.0
	index := 0
	values := Head{}
	for {
		duration := between(&r, transition)
		hold := between(&r, holdRange)
		yaw := (r.next()*2 - 1) * 2.4
		pitch := 0.0
		if index == 0 {
			sign := 1.0
			if r.next() < .5 {
				sign = -1
			}
			pitch = sign * .8 * .8125
		} else {
			pitch = (r.next()*2 - 1) * .8
		}
		targetYaw := yaw
		if index == 0 {
			sign := 1.0
			if yaw < 0 {
				sign = -1
			}
			targetYaw = sign * 2.4 * .75
		}
		target := Head{targetYaw, pitch, (r.next()*2 - 1) * .25}
		if ms < start+duration+hold {
			values = blendHead(from, target, ease((ms-start)/duration))
			break
		}
		from = target
		start += duration + hold
		index++
	}
	gr := random{seed ^ 0x37b8}
	next := between(&gr, [2]float64{8000, 14000})
	glance := [2]float64{}
	follow := 0.0
	for {
		closing := between(&gr, [2]float64{80, 140})
		hold := between(&gr, [2]float64{350, 800})
		opening := between(&gr, [2]float64{120, 220})
		sign := 1.0
		if gr.next() < .5 {
			sign = -1
		}
		x := sign * (.45 + gr.next()*.25)
		y := (gr.next()*2 - 1) * .22
		follows := gr.next() < .35
		if ms < next {
			break
		}
		maximum := closing + hold + opening
		if follows {
			maximum = math.Max(maximum, 1950)
		}
		if ms < next+maximum {
			local := ms - next
			t := 0.0
			if local < 0 {
				t = 0
			} else if local < closing {
				t = ease(local / closing)
			} else if local < closing+hold {
				t = 1
			} else {
				t = 1 - ease((local-closing-hold)/opening)
			}
			glance = [2]float64{x * t, y * t}
			if follows && local >= 150 {
				follow = x * math.Pow(math.Sin(math.Pi*math.Min(1, (local-150)/1800)), 2)
			}
			break
		}
		next += between(&gr, [2]float64{8000, 14000})
	}
	return attention{Head{values.Yaw*(1-math.Abs(follow)*.1) + follow*.3, values.Pitch, values.Roll}, glance}
}
func speakingAttentionAt(ms float64, seed uint32, brief ...bool) attention {
	r := random{seed ^ 0x6a23}
	start := between(&r, [2]float64{850, 1500})
	for {
		move := between(&r, [2]float64{110, 170})
		holdRange := [2]float64{900, 1700}
		if len(brief) > 0 && brief[0] {
			holdRange = [2]float64{250, 600}
		}
		hold := between(&r, holdRange)
		back := between(&r, [2]float64{200, 320})
		sign := 1.0
		if r.next() < .5 {
			sign = -1
		}
		x := sign * (.65 + r.next()*.2)
		y := 0.0
		if r.next() < .75 {
			y = .2 + r.next()*.25
		} else {
			y = -r.next() * .12
		}
		headStart := start + 180
		headReturn := start + move + hold
		end := headReturn + 850
		if ms < end {
			eye := ease((ms-start)/move) * (1 - ease((ms-start-move-hold)/back))
			head := ease((ms-headStart)/700) * (1 - ease((ms-headReturn)/850))
			return attention{Head{x * .85 * head, -y * .4 * head, -x * .06 * head}, [2]float64{x * eye, y * eye}}
		}
		start = end + between(&r, [2]float64{2200, 4300})
	}
}

type speakingDirector struct {
	active                                         bool
	start, changedAt, lastSpeakingAt, weight, from float64
	episodeSeed                                    uint32
}

func newSpeaking(seed uint32) speakingDirector {
	return speakingDirector{lastSpeakingAt: math.Inf(-1), episodeSeed: seed}
}
func (d *speakingDirector) sample(ms float64, speaking bool, idle attention, seed uint32, expressive bool, continuous ...bool) attention {
	if speaking != d.active {
		if speaking && ms-d.lastSpeakingAt > 900 {
			d.start = ms
			d.episodeSeed = seed ^ uint32(math.Floor(ms/1000))
		}
		d.active = speaking
		d.changedAt = ms
		d.from = d.weight
	}
	if speaking {
		d.lastSpeakingAt = ms
	}
	goal := 0.0
	duration := 1200.0
	if speaking {
		goal = 1
		duration = 700
	}
	d.weight = blend(d.from, goal, ease((ms-d.changedAt)/duration))
	speech := speakingAttentionAt(math.Max(0, ms-d.start), d.episodeSeed, len(continuous) > 0 && continuous[0])
	reduction := .7
	if expressive {
		reduction = .35
	}
	if len(continuous) > 0 && continuous[0] {
		reduction = 0
		// Speaking gaze remains independent; its slow automatic head-follow
		// used to add another lateral excursion on top of posture changes.
		speech.Head = Head{}
	}
	postureWeight := 1 - d.weight*reduction
	return attention{Head{idle.Head.Yaw*postureWeight + speech.Head.Yaw*d.weight, idle.Head.Pitch*postureWeight + speech.Head.Pitch*d.weight, idle.Head.Roll*postureWeight + speech.Head.Roll*d.weight}, [2]float64{idle.Glance[0]*(1-d.weight) + speech.Glance[0]*d.weight, idle.Glance[1]*(1-d.weight) + speech.Glance[1]*d.weight}}
}

type attentionDirector struct {
	presence                                         presenceDirector
	posture                                          briefPosture
	semanticRelease                                  accentMotion
	purposeGain, purposeFrom, purposeGoal, purposeAt float64
	motionReply                                      string
	motionSession                                    uint64
	motionTrack                                      []Gesture
	motionLastActive                                 float64
	natural                                          bool
	active                                           bool
	session                                          uint64
	changedAt                                        float64
	previous, from                                   Head
	idleWeight, idleFrom                             float64
	speaking                                         speakingDirector
}

func newAttention(seed uint32) attentionDirector {
	return attentionDirector{purposeGain: 1, purposeFrom: 1, purposeGoal: 1, session: ^uint64(0), idleWeight: 1, idleFrom: 1, speaking: newSpeaking(seed)}
}
func (d *attentionDirector) sample(ms float64, speech SpeechState, c Config, presentation ...Settings) attention {
	if c.PresenceVersion == PresenceVersion && !speech.Preview {
		settings := Settings{}
		if len(presentation) > 0 {
			settings = presentation[0]
		}
		head := d.presence.sample(ms, speech, c, settings)
		a := attention{Head: head, Glance: idleAttention(ms, c.Seed, c.Expressive).Glance}
		return d.speaking.sample(ms, speech.Active, a, c.Seed, c.Expressive, true)
	}
	if speech.Active {
		d.natural = speech.plan() != nil && speech.plan().natural
	}
	if d.active != speech.Active || d.session != speech.Session {
		d.active = speech.Active
		d.session = speech.Session
		d.changedAt = ms
		d.from = d.previous
		d.idleFrom = d.idleWeight
	}
	duration := 600.
	if d.natural {
		duration = 240
	}
	t := ease((ms - d.changedAt) / duration)
	goal := Head{}
	if speech.Active {
		goal = gestureAt(speech.plan(), speech.Position)
		if p := speech.plan(); c.ConversationalMotion && p != nil && p.Performance != nil && p.Performance.Motion.Valid() {
			m := p.Performance.Motion
			if d.motionReply != m.ReplyID {
				d.motionReply = m.ReplyID
				d.motionTrack = nil
				d.motionSession = ^uint64(0)
			}
			at := m.Offset + speech.Position
			if d.motionSession != speech.Session {
				kept := make([]Gesture, 0, len(p.Gestures)+2)
				for _, g := range d.motionTrack {
					if g.Time+g.Duration > at && ms-d.motionLastActive <= 240 {
						kept = append(kept, g)
					}
				}
				for _, g := range p.Gestures {
					g.Time += m.Offset
					kept = append(kept, g)
				}
				d.motionTrack = kept
				d.motionSession = speech.Session
			}
			goal = gestureAt(&Plan{natural: true, Gestures: d.motionTrack}, at)
			d.motionLastActive = ms
		}
	}
	d.previous = blendHead(d.from, goal, t)
	if d.natural && speech.Active {
		// Preserve the previous phrase's residual while the new audio-clock
		// trajectory starts at full strength, without a second onset fade.
		d.previous = Head{d.from.Yaw*(1-t) + goal.Yaw, d.from.Pitch*(1-t) + goal.Pitch, d.from.Roll*(1-t) + goal.Roll}
		if c.ConversationalMotion {
			d.previous = goal
		}
	}
	a := idleAttention(ms, c.Seed, c.Expressive)
	if c.ConversationalMotion {
		speaking := speech.Active || ms-d.speaking.lastSpeakingAt < 900
		quiet := false
		if coordinatedSpeech(speech) {
			quiet = speech.plan().Performance.Intent.Purpose == "reassure"
		}
		if len(presentation) > 0 && presentation[0].EmotionControl != nil {
			_, energy := presentation[0].EmotionControl.Sample(presentation[0].EmotionTime)
			quiet = quiet || energy <= .35
		}
		a.Head = d.posture.sample(ms, speaking, c.Seed, c.Bounds, quiet)
	}
	if c.Conversational {
		a = d.speaking.sample(ms, speech.Active, a, c.Seed, c.Expressive, c.ConversationalMotion)
	} else if c.Directed {
		goal := 1.0
		duration := 1800.0
		if speech.Active {
			goal = 0
			duration = 700
		}
		d.idleWeight = blend(d.idleFrom, goal, ease((ms-d.changedAt)/duration))
		a.Head.Yaw *= d.idleWeight
		a.Head.Pitch *= d.idleWeight
		a.Head.Roll *= d.idleWeight
		a.Glance[0] *= d.idleWeight
		a.Glance[1] *= d.idleWeight
	}
	purposeGoal := 1.
	if speech.Active && coordinatedSpeech(speech) && speech.plan().Performance.Intent.Purpose == "reassure" {
		purposeGoal = .6
	}
	if purposeGoal != d.purposeGoal {
		d.purposeFrom, d.purposeAt, d.purposeGoal = d.purposeGain, ms, purposeGoal
	}
	d.purposeGain = blend(d.purposeFrom, d.purposeGoal, ease((ms-d.purposeAt)/320))
	a.Head.Yaw *= d.purposeGain
	a.Head.Pitch *= d.purposeGain
	a.Head.Roll *= d.purposeGain
	glanceGain := .15 + .85*unit((d.purposeGain-.6)/.4)
	a.Glance[0] *= glanceGain
	a.Glance[1] *= glanceGain
	// A phrase-local cue can finish after its phrase's PCM. Keep its existing
	// release on the session animation clock rather than exposing all of the
	// competing posture in the first frame of the following phrase or idle.
	// The admitted head gesture itself keeps its original audio-clock path.
	if focus := d.semanticRelease.sample(ms, semanticFocus(speech), speech.Active, speech.Session, false); focus > 0 {
		a.Head.Yaw *= 1 - .8*focus
		a.Head.Pitch *= 1 - .8*focus
		a.Head.Roll *= 1 - .8*focus
		a.Glance[0] *= 1 - focus
		a.Glance[1] *= 1 - focus
	}
	reserve := func(base, gesture, limit float64) float64 {
		if limit <= 0 {
			return 0
		}
		return base*math.Min(1, limit/math.Max(.001, math.Abs(base)))*(1-math.Min(1, math.Abs(gesture)/limit)) + clamp(gesture, -limit, limit)
	}
	a.Head = Head{reserve(a.Head.Yaw, d.previous.Yaw, c.Bounds.Yaw), reserve(a.Head.Pitch, d.previous.Pitch, c.Bounds.Pitch), reserve(a.Head.Roll, d.previous.Roll, c.Bounds.Roll)}
	return a
}
func gazeOffsets(eyes [2]GazeEye, head Head, glance [2]float64) [2][2]float64 {
	desired := [2]float64{head.Yaw*(-.09) + glance[0], head.Pitch*.14 + glance[1]}
	fractions := [2]float64{.05, .02}
	safe := [2]float64{}
	for axis, v := range desired {
		sign := 1
		if v < 0 {
			sign = 0
		}
		common := math.Inf(1)
		for _, eye := range eyes {
			common = math.Min(common, eye.Bounds[axis][sign]/(eye.Width*fractions[axis]))
		}
		safe[axis] = math.Copysign(math.Min(math.Abs(v), common), v)
	}
	length := math.Max(1, math.Hypot(safe[0], safe[1]))
	result := [2][2]float64{}
	for i, eye := range eyes {
		for axis := 0; axis < 2; axis++ {
			result[i][axis] = math.Trunc((safe[axis]/length)*eye.Width*fractions[axis]*20) / 20
		}
	}
	return result
}
func breathingAt(ms float64, seed uint32, settings *Breathing) float64 {
	if settings == nil {
		return 0
	}
	r := random{seed}
	start := 0.0
	ms = math.Max(0, ms)
	for {
		duration := between(&r, settings.Duration)
		inhale := between(&r, settings.Inhale)
		amplitude := between(&r, settings.Amplitude)
		if ms < start+duration {
			phase := (ms - start) / duration
			coverage := 0.0
			if phase < inhale {
				coverage = ease(phase / inhale)
			} else {
				coverage = 1 - ease((phase-inhale)/(1-inhale))
			}
			if coverage == 0 {
				return 0
			}
			return -amplitude * coverage
		}
		start += duration
	}
}
