package conversation

import (
	"math"
	"sort"
	"syntelle/render-contract"
)

type acousticSchedule struct {
	seed uint32
	next float64
}

func newAcousticSchedule(identity string) *acousticSchedule {
	a := &acousticSchedule{seed: 2166136261}
	for _, b := range []byte(identity) {
		a.seed = (a.seed ^ uint32(b)) * 16777619
	}
	a.next = a.gap()
	return a
}
func (a *acousticSchedule) gap() float64 {
	a.seed = 1664525*a.seed + 1013904223
	return 4 + float64(a.seed)/4294967296*3
}

func (a *acousticSchedule) supplement(v PerformanceValidation, boundaries, energy []float64, peak, duration, offset float64) PerformanceValidation {
	for a.next < offset {
		a.next += a.gap()
	}
	for a.next < offset+duration {
		next := a.next - offset
		a.next += a.gap()
		if peak < .005 || len(boundaries) == 0 {
			continue
		}
		at, distance := 0., .8
		for _, candidate := range boundaries {
			if d := math.Abs(candidate - next); d < distance {
				at, distance = candidate, d
			}
		}
		start, length := at, .85
		if v.Intent.Version == "performance-intent/3" {
			length = .72
			start = math.Max(0, at-rendercontract.NaturalGestureLead("emphasis", length))
		}
		if distance >= .8 || at < .15 || start+length > duration {
			continue
		}
		heads, near := 0, false
		for _, other := range v.Resolved {
			if headCue(other.Kind) {
				heads += rendercontract.GestureRepetitions(other.Kind, other.Repetitions)
				if math.Abs(other.Time-start) < 2.2 || (start < other.Time+other.Duration && start+length > other.Time) {
					near = true
				}
			}
		}
		if heads >= 8 || near {
			continue
		}
		amount := .5
		if index := int(at / .02); index < len(energy) {
			amount += .5 * energy[index] / peak
		}
		v.Resolved = append(v.Resolved, ResolvedCue{Kind: "emphasis", Time: start, Duration: length, Amount: &amount, Source: "acoustic"})
	}
	sort.SliceStable(v.Resolved, func(i, j int) bool { return v.Resolved[i].Time < v.Resolved[j].Time })
	return v
}
