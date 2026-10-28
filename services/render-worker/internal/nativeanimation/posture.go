package nativeanimation

import "math"

const PostureTimingVersion = "conversational-presence/2"

// Owned by the existing attention director and sampled on its pause-aware
// animation clock. Phrase changes do not restart posture or choose new targets.
type briefPosture struct {
	random                random
	ready                 bool
	from, target, value   Head
	start, duration, hold float64
}

func (p *briefPosture) sample(ms float64, speaking bool, seed uint32, bounds Bounds, quiet ...bool) Head {
	if !p.ready {
		*p = briefPosture{random: random{seed ^ 0x7d135}, ready: true, start: ms, duration: 0, hold: 800}
	}
	for ms >= p.start+p.duration+p.hold {
		p.start += p.duration + p.hold
		p.from = p.target
		move, hold := [2]float64{550, 900}, [2]float64{1600, 2600}
		yaw, pitch, roll := math.Min(1.8, bounds.Yaw*.45), math.Min(.65, bounds.Pitch*.4), math.Min(.18, bounds.Roll*.35)
		if speaking {
			move, hold = [2]float64{280, 500}, [2]float64{800, 1600}
			if len(quiet) > 0 && quiet[0] {
				hold = [2]float64{1600, 2800}
			}
			yaw, pitch, roll = math.Min(3.6, bounds.Yaw*.55), math.Min(1.7, bounds.Pitch*.55), math.Min(.45, bounds.Roll*.45)
		}
		p.duration, p.hold = between(&p.random, move), between(&p.random, hold)
		// A bounded random walk avoids repeated opposite-side targets. Reach
		// near center first rather than sweeping across it in one movement.
		y := clamp(p.from.Yaw+(p.random.next()*2-1)*math.Min(2.2, yaw), -yaw, yaw)
		if y*p.from.Yaw < 0 && math.Abs(y) > .6 && math.Abs(p.from.Yaw) > .6 {
			y = p.from.Yaw * .2
		}
		// Speech varies which diagonal leads instead of making every adjustment
		// another lateral turn. Idle consumes its original random sequence.
		if speaking && p.random.next() < .35 {
			y = blend(p.from.Yaw, y, .35)
		}
		z := clamp(p.from.Pitch+(p.random.next()*2-1)*pitch, -pitch, pitch)
		r := clamp(p.from.Roll+(p.random.next()*2-1)*roll, -roll, roll)
		if math.Abs(y-p.from.Yaw) < .3 && math.Abs(z-p.from.Pitch) < .18 {
			// A quiet target is a longer genuine hold, not a forced gesture.
			p.target = p.from
			if !speaking {
				p.hold += 900
			}
		} else {
			p.target = Head{y, z, r}
		}
	}
	p.value = blendHead(p.from, p.target, ease((ms-p.start)/math.Max(1, p.duration)))
	return p.value
}
