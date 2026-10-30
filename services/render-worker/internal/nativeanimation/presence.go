package nativeanimation

import "math"

const PresenceVersion = "conversational-presence/3"

type presenceGesture struct {
	gesture Gesture
	started bool
}

// Owned by the video-loop attention director. Source progress advances speaking
// actions; wall time is used only for idle and interruption recovery. No PCM,
// timers, goroutines, or mutable prepared analysis are retained here.
type presenceDirector struct {
	ready, active, releasing bool
	random                   random
	clock, lastMS, position  float64
	duration                 float64
	session                  uint64
	reply                    string
	base, value, offset      Head
	from, target             Head
	moveAt, moveDuration     float64
	settling                 bool
	velocity                 Head
	releaseFrom              Head
	releaseAt                float64
	lastAdjustment, spacing  float64
	fallbackAt, nextIdle     float64
	adjustments              int
	gestures                 []presenceGesture
}

func presenceEnergy(s SpeechState, settings Settings) (float64, bool) {
	energy := .5
	if settings.EmotionControl != nil {
		_, energy = settings.EmotionControl.Sample(settings.EmotionTime)
	}
	quiet := energy <= .35
	if coordinatedSpeech(s) && s.plan().Performance.Intent.Purpose == "reassure" {
		quiet = true
	}
	return 1 + .3*(unit(energy)-.5), quiet
}

func (p *presenceDirector) sample(ms float64, s SpeechState, c Config, settings Settings) Head {
	if !p.ready {
		*p = presenceDirector{ready: true, random: random{c.Seed ^ 0x7d135}, lastMS: ms, lastAdjustment: -10000}
		p.spacing = p.random.next()
		p.fallbackAt, p.nextIdle = 1100, 800
	}
	if s.Waiting && !s.Active {
		p.lastMS = ms
		return p.value
	}
	previousPosition := p.position
	newPhrase := s.Active && (!p.active || p.session != s.Session)
	delta := math.Max(0, ms-p.lastMS)
	if s.Active {
		delta = math.Max(0, s.Position-p.position) * 1000
		if newPhrase {
			delta = s.Position * 1000
			if p.active && p.reply == s.ReplyID {
				delta += math.Max(0, p.duration-p.position) * 1000
			}
			previousPosition = -1e-9
		}
	} else if s.Waiting {
		delta = 0
	}
	p.clock += delta
	p.lastMS = ms
	if p.active && !s.Active && !s.Waiting {
		p.releaseFrom, p.releaseAt, p.releasing = p.offset, p.clock, true
		p.gestures = nil
		p.nextIdle = p.clock + 1200
	}
	if newPhrase {
		if p.reply != s.ReplyID {
			if p.active {
				p.releaseFrom, p.releaseAt, p.releasing = p.offset, p.clock, true
			}
			p.gestures = nil
		}
		if plan := s.plan(); plan != nil {
			for _, g := range plan.Gestures {
				g.Time += p.clock/1000 - s.Position
				p.gestures = append(p.gestures, presenceGesture{gesture: g})
			}
		}
		p.session, p.reply = s.Session, s.ReplyID
	}
	if s.Active {
		p.position = s.Position
		if s.Speech != nil {
			p.duration = s.Speech.Duration
		}
	}
	// Waiting can separate immutable prefixes/phrases without ending a reply.
	if !s.Waiting {
		p.active = s.Active
	}
	gain, quiet := presenceEnergy(s, settings)
	if p.moveDuration > 0 {
		if p.settling {
			u := unit((p.clock - p.moveAt) / p.moveDuration)
			p.base = addHead(p.from, scaleHead(p.velocity, p.moveDuration*(u-u*u/2)))
		} else {
			p.base = Head{
				blend(p.from.Yaw, p.target.Yaw, ease((p.clock-p.moveAt)/p.moveDuration)),
				blend(p.from.Pitch, p.target.Pitch, ease((p.clock-p.moveAt-35)/(p.moveDuration-35))),
				blend(p.from.Roll, p.target.Roll, ease((p.clock-p.moveAt-70)/(p.moveDuration-70))),
			}
		}
		if p.clock >= p.moveAt+p.moveDuration {
			p.moveDuration = 0
		}
	}
	if !s.Active {
		if p.moveDuration == 0 && p.clock >= p.nextIdle && !p.releasing {
			p.adjust(c, gain, false, false)
			p.nextIdle = p.clock + p.moveDuration + 1000 + 700*p.random.next()
		}
		p.offset = Head{}
		if p.releasing {
			p.offset = scaleHead(p.releaseFrom, 1-ease((p.clock-p.releaseAt)/240))
			p.releasing = p.clock-p.releaseAt < 240
		}
		p.value = addHead(p.base, p.offset)
		return p.value
	}

	p.offset = Head{}
	if p.releasing {
		p.offset = scaleHead(p.releaseFrom, 1-ease((p.clock-p.releaseAt)/240))
		p.releasing = p.clock-p.releaseAt < 240
	}
	kept := p.gestures[:0]
	occupied := p.releasing
	for _, queued := range p.gestures {
		g := queued.gesture
		at := p.clock / 1000
		if at >= g.Time+g.Duration {
			continue
		}
		if at >= g.Time {
			if !queued.started {
				if occupied {
					continue
				}
				// Fit the whole trajectory before admitting it. Never scale the
				// displayed base toward zero to manufacture gesture headroom.
				p.settle()
				scale := gain
				for _, axis := range [][3]float64{{math.Max(math.Abs(p.base.Yaw), math.Abs(p.target.Yaw)), g.Yaw, c.Bounds.Yaw}, {math.Max(math.Abs(p.base.Pitch), math.Abs(p.target.Pitch)), g.Pitch, c.Bounds.Pitch}, {math.Max(math.Abs(p.base.Roll), math.Abs(p.target.Roll)), g.Roll, c.Bounds.Roll}} {
					if axis[1] != 0 {
						scale = math.Min(scale, math.Max(0, axis[2]-math.Abs(axis[0]))/math.Abs(axis[1]))
					}
				}
				g.Yaw *= scale
				g.Pitch *= scale
				g.Roll *= scale
				queued.gesture, queued.started = g, true
			}
			p.offset = addHead(p.offset, naturalGestureAt(g, (at-g.Time)/g.Duration))
			occupied = true
		}
		kept = append(kept, queued)
	}
	p.gestures = kept
	gap := 2000 + 1000*p.spacing
	if quiet {
		gap += 2000
	}
	opportunity := false
	if plan := s.plan(); plan != nil {
		for _, cue := range plan.Cues {
			if cue.Time <= previousPosition || cue.Time > s.Position || (cue.Kind != "pause" && cue.Kind != "emphasis") {
				continue
			}
			// Ignore newly discovered opportunities behind the consumed source
			// cursor. Replacing a prefix cannot replay a historical movement.
			opportunity = true
		}
	}
	minimum := 2000.
	if quiet {
		minimum = 4000
	}
	due := opportunity && p.clock-p.lastAdjustment >= math.Max(minimum, gap-500)
	// Owner correction: prefer speech opportunities, but retain subtle presence
	// during uninterrupted speech instead of holding indefinitely.
	due = due || (p.clock >= p.fallbackAt && p.clock-p.lastAdjustment >= gap)
	collision := occupied
	for _, queued := range p.gestures {
		g := queued.gesture
		if g.Time*1000 < p.clock+950 && (g.Time+g.Duration)*1000 > p.clock-200 {
			collision = true
		}
	}
	if due && !collision && p.moveDuration == 0 {
		p.adjust(c, gain, quiet, true)
		p.fallbackAt = p.clock + 2000 + 1000*p.spacing
		if quiet {
			p.fallbackAt += 2000
		}
	}
	p.value = addHead(p.base, p.offset)
	return p.value
}

func (p *presenceDirector) adjust(c Config, gain float64, quiet, speaking bool) {
	if quiet {
		gain *= .65
	}
	limits := Head{math.Min(3.6, c.Bounds.Yaw*.55), math.Min(1.7, c.Bounds.Pitch*.55), math.Min(.45, c.Bounds.Roll*.45)}
	if !speaking {
		limits = Head{math.Min(1.8, c.Bounds.Yaw*.45), math.Min(.65, c.Bounds.Pitch*.4), math.Min(.18, c.Bounds.Roll*.35)}
	}
	step := func(from, maximum, limit float64) float64 {
		// Preserve bounded per-action travel even when entering a smaller idle range.
		target := clamp(from+(p.random.next()*2-1)*math.Min(maximum, maximum*gain), -limit, limit)
		return from + clamp(target-from, -maximum, maximum)
	}
	target := Head{step(p.base.Yaw, 1, limits.Yaw), step(p.base.Pitch, .5, limits.Pitch), step(p.base.Roll, .15, limits.Roll)}
	if headChange(p.base, target) < .25 {
		direction := 1.
		if p.base.Yaw > 0 {
			direction = -1
		}
		target.Yaw = clamp(p.base.Yaw+direction*.3, -limits.Yaw, limits.Yaw)
	}
	if target.Yaw*p.base.Yaw < 0 && math.Abs(target.Yaw) > .6 && math.Abs(p.base.Yaw) > .6 {
		target.Yaw = p.base.Yaw * .2
	}
	p.from, p.target, p.moveAt = p.base, target, p.clock
	p.settling = false
	p.moveDuration = 650 + 100*p.random.next()
	if !speaking {
		p.moveDuration = 550 + 350*p.random.next()
	}
	p.lastAdjustment, p.spacing = p.clock, p.random.next()
	p.adjustments++
}

func headChange(a, b Head) float64 {
	return math.Abs(a.Yaw-b.Yaw) + math.Abs(a.Pitch-b.Pitch) + math.Abs(a.Roll-b.Roll)
}

func addHead(a, b Head) Head {
	return Head{Yaw: a.Yaw + b.Yaw, Pitch: a.Pitch + b.Pitch, Roll: a.Roll + b.Roll}
}
func scaleHead(a Head, scale float64) Head {
	return Head{Yaw: a.Yaw * scale, Pitch: a.Pitch * scale, Roll: a.Roll * scale}
}

// Preserve velocity when a newly arriving gesture interrupts a background
// adjustment, decelerating the latter over120ms without recentering its pose.
func (p *presenceDirector) settle() {
	if p.settling {
		return
	}
	if p.moveDuration == 0 {
		p.target = p.base
		return
	}
	velocity := func(from, to, offset float64) float64 {
		d := p.moveDuration - offset
		u := unit((p.clock - p.moveAt - offset) / d)
		return (to - from) * 30 * u * u * (1 - u) * (1 - u) / d
	}
	p.velocity = Head{velocity(p.from.Yaw, p.target.Yaw, 0), velocity(p.from.Pitch, p.target.Pitch, 35), velocity(p.from.Roll, p.target.Roll, 70)}
	p.from, p.moveAt, p.moveDuration, p.settling = p.base, p.clock, 120, true
	p.target = addHead(p.base, scaleHead(p.velocity, 60))
}
