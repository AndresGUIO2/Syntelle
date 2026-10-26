package nativeanimation

import (
	"math"
	"syntelle/render-contract"
)

// Every segment uses the same C2 easing, with a faster stroke and slower
// recovery. Excursions stay inside each existing gesture's prepared amplitude.
func naturalGestureAt(g Gesture, p float64) Head {
	if p <= 0 || p >= 1 {
		return Head{}
	}
	if g.Kind == "shake" || g.Kind == "shake_cycle" {
		y := 0.
		if p < .24 {
			y = ease(p / .24)
		} else if p < .70 {
			y = 1 - 1.82*ease((p-.24)/.46)
		} else {
			y = -.82 * (1 - ease((p-.70)/.30))
		}
		return Head{Yaw: g.Yaw * y}
	}
	peak := .34
	if g.Kind == "emphasis" {
		peak = .28
	}
	w := 0.
	if g.Kind == "tilt" {
		w = ease(p/.34) * (1 - ease((p-.58)/.42))
	} else if p < peak {
		w = ease(p / peak)
	} else {
		w = 1 - ease((p-peak)/(1-peak))
	}
	return Head{Yaw: g.Yaw * w, Pitch: g.Pitch * w, Roll: g.Roll * w}
}

func naturalGestures(cue PerformanceCue, bounds Bounds, seed uint32, ordinal int) []Gesture {
	n := rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)
	if n == 0 || cue.Duration <= 0 {
		return nil
	}
	amount := 1.
	if cue.Amount != nil {
		amount = unit(*cue.Amount)
	}
	// Stable text/cue variation changes performance, never the saved baseline.
	r := random{seed ^ uint32(ordinal+1)*0x9e3779b9}
	sign := 1.
	if r.next() < .5 {
		sign = -1
	}
	pitch, yaw, roll := 0., 0., 0.
	kind, duration := cue.Kind, cue.Duration
	switch kind {
	case "nod", "double_nod":
		kind = "nod"
		pitch = -math.Min(3.8, bounds.Pitch*.95)
		duration = math.Min(duration, rendercontract.NaturalNodCycleSeconds*float64(n))
	case "shake":
		yaw = sign * math.Min(2.85, bounds.Yaw*.95)
		duration = math.Min(duration, rendercontract.NaturalShakeCycleSeconds*float64(n))
	case "emphasis":
		pitch = -math.Min(.9, bounds.Pitch*.3)
	case "tilt":
		roll = sign * math.Min(1.14, bounds.Roll*.95)
	default:
		return nil
	}
	weights := make([]float64, n)
	total := 0.
	for i := range weights {
		weights[i] = .96 + .08*r.next()
		total += weights[i]
	}
	at := cue.Time
	result := make([]Gesture, 0, n)
	for i := 0; i < n; i++ {
		length := duration * weights[i] / total
		if i == n-1 {
			length = cue.Time + duration - at
		}
		strength := amount * (.94 + .06*r.next())
		result = append(result, Gesture{at, length, pitch * strength, yaw * strength, kind, roll * strength})
		at += length
	}
	return result
}
