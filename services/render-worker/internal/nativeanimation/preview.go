package nativeanimation

import (
	"math"
	"syntelle/render-contract"
)

// PreviewFrame is a bounded visual inspection on the pause-aware animation
// clock. It never creates speech, conversation context or a model request.
func ValidPreview(kind string) bool {
	return isHeadCue(kind) || kind == "smile" || kind == "brows" || kind == "laugh"
}
func PreviewFrame(base Frame, kind string, seconds float64, c Config) Frame {
	return PreviewFrameAtIntensity(base, kind, seconds, 1, c)
}

func PreviewFrameAtIntensity(base Frame, kind string, seconds, intensity float64, c Config) Frame {
	return PreviewFrameRepeated(base, kind, seconds, intensity, 0, c)
}

func PreviewFrameRepeated(base Frame, kind string, seconds, intensity float64, repetitions int, c Config) Frame {
	var count *int
	if repetitions != 0 {
		count = &repetitions
	}
	n := rendercontract.GestureRepetitions(kind, count)
	maximum := 2.
	if kind == "nod" || kind == "double_nod" {
		maximum = rendercontract.NaturalNodCycleSeconds * float64(n)
	}
	if kind == "shake" {
		maximum = rendercontract.NaturalShakeCycleSeconds * float64(n)
	}
	if n == 0 || seconds <= 0 || seconds >= maximum || !ValidPreview(kind) || !patchPreviewAmount(intensity) {
		return base
	}
	weight := ease(seconds/.3) * (1 - ease((seconds-1.3)/.7))
	switch kind {
	case "smile":
		if c.SpeakingSmile {
			base.Smile = blend(base.Smile, 1, weight)
		}
	case "brows":
		if c.Expressions {
			base.Brows = blend(base.Brows, .5, weight)
		}
	case "laugh":
		if c.LaughAvailable {
			if !c.ReferenceLaugh {
				base.Smile = blend(base.Smile, 1, weight)
			}
			base.Laugh = weight
			base.Mouth = Mouth{"Rest", "A", .06 * weight * (.5 + .5*math.Sin(seconds*math.Pi*6))}
			if c.ReferenceLaugh {
				base.Mouth.Amount = .5 + .5*math.Sin(seconds*math.Pi*6)
			}
		}
	default:
		duration := .72
		if kind == "nod" {
			duration = rendercontract.NaturalNodCycleSeconds * float64(n)
		}
		if kind == "double_nod" {
			duration = rendercontract.NaturalNodCycleSeconds * float64(n)
		}
		if kind == "shake" {
			duration = rendercontract.NaturalShakeCycleSeconds * float64(n)
		}
		if kind == "tilt" {
			duration = 1.15
		}
		gesture := Gesture{Time: 0, Duration: duration, Kind: kind, Pitch: -c.Bounds.Pitch * .95 * intensity}
		if kind == "double_nod" {
			gesture.Kind = "nod"
		}
		if kind == "shake" {
			gesture.Pitch = 0
			gesture.Yaw = c.Bounds.Yaw * .95 * intensity
			gesture.Kind = "shake_cycle"
		}
		if kind == "tilt" {
			gesture.Pitch = 0
			gesture.Roll = c.Bounds.Roll * .95 * intensity
		}
		gestures := []Gesture{gesture}
		if kind == "nod" || kind == "double_nod" || kind == "shake" {
			cycle := rendercontract.NaturalNodCycleSeconds
			if kind == "shake" {
				cycle = rendercontract.NaturalShakeCycleSeconds
			}
			gestures = nil
			for i := 0; i < n; i++ {
				g := gesture
				g.Time = float64(i) * cycle
				g.Duration = cycle
				gestures = append(gestures, g)
			}
		}
		head := gestureAt(&Plan{Gestures: gestures, natural: true}, seconds)
		// Reserve the gesture range instead of clipping it against the idle posture.
		reserve := ease(seconds/.12) * (1 - ease((seconds-duration+.16)/.16))
		base.Head = Head{clamp(base.Head.Yaw*(1-reserve)+head.Yaw, -c.Bounds.Yaw, c.Bounds.Yaw), clamp(base.Head.Pitch*(1-reserve)+head.Pitch, -c.Bounds.Pitch, c.Bounds.Pitch), clamp(base.Head.Roll*(1-reserve)+head.Roll, -c.Bounds.Roll, c.Bounds.Roll)}
	}
	return base
}

func patchPreviewAmount(amount float64) bool {
	return !math.IsNaN(amount) && !math.IsInf(amount, 0) && amount >= 0 && amount <= 1
}
