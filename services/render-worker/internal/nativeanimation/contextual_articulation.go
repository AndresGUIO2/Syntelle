package nativeanimation

import "math"

// Original native events remain immutable; pose simplification is legacy only.
func (s *Speech) MouthAt(at float64) Mouth {
	if s == nil {
		return Mouth{"Rest", "A", 0}
	}
	if !s.contextual {
		return MouthAt(s.Timeline, at)
	}
	previous := "Rest"
	previousTime := -.07
	for _, e := range s.NativeEvents {
		t := math.Min(s.Duration, e.OffsetMS/1000)
		pose := azurePoses[e.ID]
		if at <= t {
			duration := math.Min(.10, math.Max(.001, t-previousTime))
			if mouthContact(pose) || mouthContact(previous) {
				duration = math.Min(duration, .035)
			}
			if at < t-duration || previous == pose {
				return Mouth{previous, previous, 0}
			}
			return Mouth{previous, pose, smooth((at - t + duration) / duration)}
		}
		previous, previousTime = pose, t
	}
	if at >= s.Duration {
		return Mouth{"Rest", "A", 0}
	}
	return Mouth{previous, previous, 0}
}

func (s *Speech) ResidualAt(at float64, enabled bool) [2]float64 {
	if s == nil || !enabled || !s.contextual || at < 0 || at >= s.Duration {
		return [2]float64{}
	}
	index := -1
	for i, e := range s.NativeEvents {
		if e.OffsetMS/1000 <= at {
			index = i
		} else {
			break
		}
	}
	if index < 0 {
		return [2]float64{}
	}
	for _, e := range s.NativeEvents {
		if (e.ID == 0 || e.ID == 21 || e.ID == 18) && math.Abs(at-e.OffsetMS/1000) < .04 {
			return [2]float64{}
		}
	}
	target := func(i int) [2]float64 {
		if i < 0 || i >= len(s.NativeEvents) {
			return [2]float64{}
		}
		e := s.NativeEvents[i]
		pose := azurePoses[e.ID]
		if pose == "Rest" || mouthContact(pose) {
			return [2]float64{}
		}
		// Shape classes retain distinctions hidden by the eight-image mapping.
		opening, corner := 0., 0.
		switch e.ID {
		case 4, 5:
			opening = .5
			corner = .4
		case 6:
			opening = -.3
			corner = .6
		case 14:
			opening = -.5
			corner = -.2
		case 19:
			opening = -.65
			corner = -.35
		}
		at := e.OffsetMS / 1000
		local := s.EnergyAt(at)
		mean := (s.EnergyAt(at-.04) + s.EnergyAt(at+.04)) * .5
		opening += .25 * clamp((local-mean)*3, -1, 1)
		if i > 0 && s.NativeEvents[i-1].ID != e.ID {
			opening += .1
		}
		return [2]float64{.05 * clamp(corner, -1, 1), .05 * clamp(opening, -1, 1)}
	}
	from := target(index)
	next := index + 1
	if next >= len(s.NativeEvents) {
		return from
	}
	end := s.NativeEvents[next].OffsetMS / 1000
	start := math.Max(s.NativeEvents[index].OffsetMS/1000, end-.04)
	weight := smooth((at - start) / math.Max(.001, end-start))
	to := target(next)
	return [2]float64{blend(from[0], to[0], weight), blend(from[1], to[1], weight)}
}
