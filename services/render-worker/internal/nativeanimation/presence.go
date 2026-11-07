package nativeanimation

import "math"

const IntentionalPresenceVersion = "conversational-presence/6"
const SemanticMotionDirection = "semantic-anchors/1"

const PresenceVersion = "conversational-presence/3"
const ExpressivePresenceVersion = "conversational-presence/5"

const VariedPresenceVersion = "conversational-presence/4"

func presenceEnabled(c Config) bool {
	return c.PresenceVersion == PresenceVersion || (c.PresenceVersion == VariedPresenceVersion || c.PresenceVersion == ExpressivePresenceVersion || c.PresenceVersion == IntentionalPresenceVersion)
}

// Acoustic beats already populate the source-clock posture opportunities. They
// must not also create a pitch-return gesture that blocks those opportunities.
func acousticPostureCue(plan *Plan, g Gesture) bool {
	if plan == nil || plan.Performance == nil || g.Kind != "emphasis" {
		return false
	}
	for _, cue := range plan.Performance.Resolved {
		if cue.Source == "acoustic" && cue.Kind == "emphasis" && math.Abs(cue.Time-g.Time) < 1e-6 {
			return true
		}
	}
	return false
}

type presenceGesture struct {
	gesture Gesture
	started bool
	amount  float64
	purpose string
}

// Owned by the video-loop attention director. Source progress advances speaking
// actions; wall time is used only for idle and interruption recovery. No PCM,
// timers, goroutines, or mutable prepared analysis are retained here.
type presenceDirector struct {
	lastPlan                 *Plan
	gestureSeen              map[string]bool
	speakingAnchor           Head
	lastSemanticSign         float64
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
	lastFast                 float64
	fastOpportunity          bool
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
	intentional := c.PresenceVersion == IntentionalPresenceVersion
	if intentional && newPhrase && (!p.active || p.reply != s.ReplyID) {
		if p.moveDuration > 0 {
			p.settle()
		}
		p.speakingAnchor = p.base
	}
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
	if newPhrase || intentional && s.Active && s.plan() != p.lastPlan {
		if p.reply != s.ReplyID {
			if p.active {
				p.releaseFrom, p.releaseAt, p.releasing = p.offset, p.clock, true
			}
			p.gestures = nil
			p.gestureSeen = nil
		}
		if plan := s.plan(); plan != nil {
			for _, g := range plan.Gestures {
				queued := presenceGesture{}
				if intentional {
					id, cue := intentionalGestureCue(plan, g)
					if cue == nil || p.gestureSeen[id] {
						continue
					}
					if p.gestureSeen == nil {
						p.gestureSeen = map[string]bool{}
					}
					p.gestureSeen[id] = true
					// Late discoveries are checked at prefix admission. An
					// admitted zero-time cue can already be one frame in at
					// the first render. Keep its original phase and finish.
					if cue.Amount != nil {
						queued.amount = unit(*cue.Amount)
					}
					queued.purpose = plan.Performance.Intent.Purpose
				}
				if (c.PresenceVersion == VariedPresenceVersion || c.PresenceVersion == ExpressivePresenceVersion) && acousticPostureCue(plan, g) {
					continue
				}
				g.Time += p.clock/1000 - s.Position
				queued.gesture = g
				p.gestures = append(p.gestures, queued)
			}
		}
		p.session, p.reply = s.Session, s.ReplyID
		p.lastPlan = s.plan()
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
	expressive := c.PresenceVersion == ExpressivePresenceVersion
	if expressive && settings.EmotionControl != nil {
		_, energy := settings.EmotionControl.Sample(settings.EmotionTime)
		quiet = energy <= .35
	} else if expressive {
		quiet = false
	}
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
			if c.PresenceVersion == VariedPresenceVersion || c.PresenceVersion == ExpressivePresenceVersion || c.PresenceVersion == IntentionalPresenceVersion {
				p.nextIdle = p.clock + p.moveDuration + 650 + 500*p.random.next()
			}
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
				if intentional && g.Kind != "nod" {
					g = p.intentionalGesture(g, c, queued.amount, queued.purpose)
				}
				scale := gain
				if intentional && (g.Kind == "emphasis" || g.Kind == "tilt") {
					scale = math.Min(1, gain)
				}
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
			if intentional {
				p.offset = addHead(p.offset, intentionalGestureAt(g, (at-g.Time)/g.Duration))
			} else {
				p.offset = addHead(p.offset, naturalGestureAt(g, (at-g.Time)/g.Duration))
			}
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
	if expressive {
		gap = 700 + 750*p.spacing
		if quiet {
			gap = 2000 + 800*p.spacing
		}
	}
	if intentional {
		opportunity = false
		gap = 3500 + 1500*p.spacing
	}
	minimum := 2000.
	if quiet {
		minimum = 4000
	}
	if expressive {
		minimum = 650
		if quiet {
			minimum = 1800
		}
	}
	due := opportunity && p.clock-p.lastAdjustment >= math.Max(minimum, gap-500)
	// Owner correction: prefer speech opportunities, but retain subtle presence
	// during uninterrupted speech instead of holding indefinitely.
	due = due || (p.clock >= p.fallbackAt && p.clock-p.lastAdjustment >= gap)
	collision := occupied
	for _, queued := range p.gestures {
		g := queued.gesture
		if g.Time*1000 < p.clock+func() float64 {
			if expressive {
				return 120
			}
			return 950
		}() && (g.Time+g.Duration)*1000 > p.clock-200 {
			collision = true
		}
	}
	if due && !collision && p.moveDuration == 0 {
		before := *p
		p.fastOpportunity = opportunity
		p.adjust(c, gain, quiet, true)
		if expressive {
			for _, queued := range p.gestures {
				g := queued.gesture
				if g.Time*1000 > p.clock && g.Time*1000 < p.clock+p.moveDuration+120 {
					*p = before
					p.value = addHead(p.base, p.offset)
					return p.value
				}
			}
		}
		p.fallbackAt = p.clock + 2000 + 1000*p.spacing
		if intentional {
			p.fallbackAt = p.clock + p.moveDuration + 1200 + 1200*p.spacing
		}
		if expressive {
			p.fallbackAt = p.clock + p.moveDuration + 450 + 550*p.spacing
		}
		if quiet {
			if expressive {
				p.fallbackAt = p.clock + p.moveDuration + 1600 + 1200*p.spacing
			} else {
				p.fallbackAt += 2000
			}
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
	travel := Head{Yaw: 1, Pitch: .5, Roll: .15}
	minimumChange, distinctYaw := .25, .3
	if c.PresenceVersion == VariedPresenceVersion || c.PresenceVersion == ExpressivePresenceVersion || c.PresenceVersion == IntentionalPresenceVersion {
		limits = Head{math.Min(4.6, c.Bounds.Yaw*.7), math.Min(2.2, c.Bounds.Pitch*.6), math.Min(.65, c.Bounds.Roll*.5)}
		if !speaking {
			limits = Head{math.Min(2.4, c.Bounds.Yaw*.65), math.Min(1.1, c.Bounds.Pitch*.45), math.Min(.38, c.Bounds.Roll*.4)}
		}
		travel = Head{Yaw: 1.8, Pitch: .85, Roll: .28}
		minimumChange, distinctYaw = .65, .7
	}
	target := Head{step(p.base.Yaw, travel.Yaw, limits.Yaw), step(p.base.Pitch, travel.Pitch, limits.Pitch), step(p.base.Roll, travel.Roll, limits.Roll)}
	if headChange(p.base, target) < minimumChange {
		direction := 1.
		if p.base.Yaw > 0 {
			direction = -1
		}
		target.Yaw = clamp(p.base.Yaw+direction*distinctYaw, -limits.Yaw, limits.Yaw)
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
	if c.PresenceVersion == VariedPresenceVersion || c.PresenceVersion == ExpressivePresenceVersion || c.PresenceVersion == IntentionalPresenceVersion {
		p.moveDuration = 700 + 250*p.random.next()
		if !speaking {
			p.moveDuration = 650 + 300*p.random.next()
		}
	}
	if speaking && c.PresenceVersion == ExpressivePresenceVersion {
		limits = Head{math.Min(6.4, c.Bounds.Yaw*.8), math.Min(3, c.Bounds.Pitch*.75), math.Min(.84, c.Bounds.Roll*.7)}
		for _, g := range p.gestures {
			if g.gesture.Time*1000 < p.clock+700 {
				limits.Yaw = math.Min(limits.Yaw, math.Max(0, c.Bounds.Yaw-math.Abs(g.gesture.Yaw)))
				limits.Pitch = math.Min(limits.Pitch, math.Max(0, c.Bounds.Pitch-math.Abs(g.gesture.Pitch)))
				limits.Roll = math.Min(limits.Roll, math.Max(0, c.Bounds.Roll-math.Abs(g.gesture.Roll)))
			}
		}
		choose := func(from, minimum, maximum, limit float64) float64 {
			sign := 1.
			if p.random.next() < .5 {
				sign = -1
			}
			d := (minimum + (maximum-minimum)*p.random.next()) * math.Min(1, gain)
			if quiet {
				d *= .55
			}
			to := clamp(from+sign*d, -limit, limit)
			if math.Abs(to-from) < minimum*.5 {
				to = clamp(from-sign*d, -limit, limit)
			}
			if from*to < 0 && math.Abs(from) > .8 && math.Abs(to) > .8 {
				to = from * .15
			}
			return to
		}
		p.target = Head{choose(p.base.Yaw, 3, 6, limits.Yaw), choose(p.base.Pitch, .8, 2, limits.Pitch), choose(p.base.Roll, .2, .6, limits.Roll)}
		p.moveDuration = 240 + 180*p.random.next()
		if !quiet && p.fastOpportunity && p.clock-p.lastFast >= 3000 && p.random.next() < .35 {
			p.moveDuration = 180 + 100*p.random.next()
			p.lastFast = p.clock
		}
		for _, axis := range [][3]float64{{p.from.Yaw, p.target.Yaw, 70}, {p.from.Pitch, p.target.Pitch, 45}, {p.from.Roll, p.target.Roll, 15}} {
			p.moveDuration = math.Max(p.moveDuration, 70+1875*math.Abs(axis[1]-axis[0])/axis[2])
		}
	}
	if speaking && c.PresenceVersion == IntentionalPresenceVersion {
		small := func(from, center, limit, bound float64) float64 {
			return clamp(from+(p.random.next()*2-1)*limit, math.Max(-limit+center, -bound), math.Min(limit+center, bound))
		}
		p.target = Head{small(p.base.Yaw, p.speakingAnchor.Yaw, .6, c.Bounds.Yaw), small(p.base.Pitch, p.speakingAnchor.Pitch, .25, c.Bounds.Pitch), small(p.base.Roll, p.speakingAnchor.Roll, .08, c.Bounds.Roll)}
		p.moveDuration = 1600 + 800*p.random.next()
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
