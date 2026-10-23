package nativeanimation

import "math"

type blinkPlanner struct {
	random                                                         random
	settings                                                       BehaviorBlink
	rate, capacity, lastTick, lastBlink, credit, hazard, threshold float64
	session                                                        uint64
	previousPosition                                               float64
	wasActive, reserved                                            bool
	lastDouble                                                     float64
}

func newBlinkPlanner(seed uint32, s BehaviorBlink) blinkPlanner {
	p := blinkPlanner{random: random{seed}, settings: s, rate: s.Rate / 60000, capacity: 1.35, lastBlink: math.Inf(-1), credit: 1, session: ^uint64(0), previousPosition: -.001, lastDouble: math.Inf(-1)}
	if s.Double != nil {
		p.capacity = 2
		p.credit = 2
	}
	p.threshold = -math.Log(1 - p.random.next())
	return p
}
func (p *blinkPlanner) record(ms float64, spend bool) {
	p.lastBlink = ms
	if spend {
		p.credit = math.Max(0, p.credit-1)
	}
	p.hazard = 0
	p.threshold = -math.Log(1 - p.random.next())
}
func (p *blinkPlanner) reserveDouble(ms float64) bool {
	d := p.settings.Double
	if d == nil || p.reserved || p.credit < 1 || ms-p.lastDouble < d.MinimumInterval || p.random.next() >= d.Probability {
		return false
	}
	p.credit--
	p.reserved = true
	return true
}
func (p *blinkPlanner) startReserved(ms float64) bool {
	if !p.reserved {
		return false
	}
	p.reserved = false
	p.lastDouble = ms
	p.record(ms, false)
	return true
}
func (p *blinkPlanner) cancelDouble() {
	if p.reserved {
		p.credit = math.Min(p.capacity, p.credit+1)
	}
	p.reserved = false
}
func (p *blinkPlanner) reset(ms float64) {
	p.lastTick = ms
	p.hazard = 0
	p.previousPosition = -.001
	p.session = ^uint64(0)
	p.wasActive = false
}
func (p *blinkPlanner) sample(ms float64, speech SpeechState, available bool) bool {
	dt := clamp(ms-p.lastTick, 0, 100)
	p.lastTick = ms
	p.credit = math.Min(p.capacity, p.credit+dt*p.rate)
	if p.session != speech.Session || (speech.Active && !p.wasActive) || !speech.Active || speech.Position < p.previousPosition {
		p.session = speech.Session
		p.previousPosition = speech.Position - .001
		if speech.Active && speech.Position < .25 {
			p.previousPosition = -.001
		}
	}
	p.wasActive = speech.Active
	cues := []Cue{}
	if speech.Active && speech.plan() != nil {
		for _, cue := range speech.plan().Cues {
			if cue.Time > p.previousPosition && cue.Time <= speech.Position && speech.Position-cue.Time < .25 {
				cues = append(cues, cue)
			}
		}
	}
	p.previousPosition = speech.Position
	if !available || ms-p.lastBlink < p.settings.MinimumGap {
		return false
	}
	contextual := false
	for _, cue := range cues {
		chance := 0.0
		switch cue.Kind {
		case "pause":
			chance = p.settings.Pause
		case "ending":
			chance = p.settings.Ending
		case "onset":
			chance = p.settings.Onset
		case "nod":
			chance = p.settings.Nod
		}
		if chance > 0 && p.random.next() < chance {
			contextual = true
			break
		}
	}
	factor := 1.1
	if speech.Active {
		factor = .65
	}
	p.hazard += dt * p.rate * factor
	if p.credit < 1 || (!contextual && p.hazard < p.threshold) {
		return false
	}
	p.record(ms, true)
	return true
}

type blinkClock struct {
	random                           random
	planner                          blinkPlanner
	settings                         BlinkSettings
	behavior                         BehaviorBlink
	deadline, start, scale, secondAt float64
	started, second, enabled         bool
}

func newBlinkClock(c Config) blinkClock {
	b := blinkClock{random: random{c.BlinkSeed}, planner: newBlinkPlanner(c.BlinkSeed, c.Behavior.Blink), settings: c.Blink, behavior: c.Behavior.Blink, scale: 1, enabled: true}
	b.deadline = b.interval()
	return b
}
func (b *blinkClock) interval() float64 {
	return b.settings.MinimumInterval + b.random.next()*(b.settings.MaximumInterval-b.settings.MinimumInterval)
}
func blinkAt(ms float64, s BlinkSettings) float64 {
	if ms < 0 {
		return 0
	}
	if ms < s.Closing {
		return smooth(ms / s.Closing)
	}
	if ms < s.Closing+s.Closed {
		return 1
	}
	return 1 - smooth((ms-s.Closing-s.Closed)/s.Opening)
}
func (b *blinkClock) sample(ms float64, speech SpeechState, enabled bool) float64 {
	if enabled != b.enabled {
		b.enabled = enabled
		if !enabled {
			b.second = false
			b.planner.cancelDouble()
		}
		b.deadline = ms + b.interval()
		b.planner.reset(ms)
	}
	duration := b.settings.Closing + b.settings.Closed + b.settings.Opening
	if b.started && ms-b.start >= duration*b.scale {
		b.started = false
		b.deadline = ms + b.interval()
	}
	if b.second && ms >= b.secondAt {
		b.second = false
		if b.planner.startReserved(ms) {
			b.scale = .88 + b.random.next()*.12
			b.start = ms
			b.started = true
		}
	}
	if enabled || b.behavior.Double != nil {
		if b.planner.sample(ms, speech, enabled && !b.started && !b.second) {
			b.scale = .88 + b.random.next()*.24
			b.start = ms
			b.started = true
			if b.planner.reserveDouble(ms) {
				gap := b.behavior.Double.OpenGap
				b.secondAt = ms + duration*b.scale + between(&b.random, gap)
				b.second = true
			}
		}
	}
	if !b.started {
		return 0
	}
	return blinkAt((ms-b.start)/b.scale, b.settings)
}
