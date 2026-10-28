package nativeanimation

import (
	"context"
	"math"
	"sort"
)

const ArticulationVersion = "audio-articulation/1"

type ArticulationFrame struct{ JawOpen, LipOpening, LipTension, LaughFace float64 }
type preparedArticulation struct {
	frames    []ArticulationFrame
	native    []NativeViseme
	laugh     bool
	lastBurst float64
}

func envelopeSample(values []ArticulationFrame, seconds float64) ArticulationFrame {
	if len(values) == 0 {
		return ArticulationFrame{}
	}
	at := math.Max(0, seconds/.02)
	lo := min(len(values)-1, int(at))
	hi := min(len(values)-1, lo+1)
	t := unit(at - float64(lo))
	a, b := values[lo], values[hi]
	return ArticulationFrame{blend(a.JawOpen, b.JawOpen, t), blend(a.LipOpening, b.LipOpening, t), blend(a.LipTension, b.LipTension, t), blend(a.LaughFace, b.LaughFace, t)}
}
func (s *Speech) ArticulationAt(seconds float64) ArticulationFrame {
	if s == nil || s.articulation == nil {
		return ArticulationFrame{}
	}
	return envelopeSample(s.articulation.frames, seconds)
}
func poseJaw(c Config, m Mouth) float64 {
	return blend(c.Mandible.PoseJaw[m.From], c.Mandible.PoseJaw[m.To], m.Amount)
}

// A 40ms centered periodicity proxy is used only for nonverbal preparation.
// Decimation bounds work without adding a recognizer or retaining PCM.
func periodicity(samples []float32, rate int, time float64) float64 {
	stride := max(1, rate/8000)
	start := max(0, int((time-.02)*float64(rate)))
	end := min(len(samples), int((time+.02)*float64(rate)))
	signal := make([]float64, 0, 320)
	mean := 0.
	for i := start; i < end; i += stride {
		v := float64(samples[i])
		signal = append(signal, v)
		mean += v
	}
	if len(signal) < 40 {
		return 0
	}
	mean /= float64(len(signal))
	for i := range signal {
		signal[i] -= mean
	}
	best := 0.
	r := rate / stride
	for lag := max(1, r/400); lag <= r/80 && lag < len(signal)/2; lag++ {
		sum, a, b := 0., 0., 0.
		for i := lag; i < len(signal); i++ {
			x, y := signal[i], signal[i-lag]
			sum += x * y
			a += x * x
			b += y * y
		}
		if a*b > 1e-16 {
			best = math.Max(best, sum/math.Sqrt(a*b))
		}
	}
	return unit(best)
}
func prepareArticulation(ctx context.Context, samples []float32, rate int, events []NativeViseme, timeline []TimelineEvent, energy []float64, c Config, laugh bool) (*preparedArticulation, error) {
	a := &preparedArticulation{native: append([]NativeViseme(nil), events...), frames: make([]ArticulationFrame, len(energy)), laugh: laugh}
	if laugh {
		target := make([]float64, len(energy))
		strengths := []float64{}
		step := max(1, int(float64(rate)*.02+.5))
		for i := range target {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			sum := 0.
			end := min(len(samples), (i+1)*step)
			for _, x := range samples[i*step : end] {
				sum += float64(x * x)
			}
			rms := math.Sqrt(sum / float64(max(1, end-i*step)))
			confidence := unit((periodicity(samples, rate, (float64(i)+.5)*.02) - .2) / .6)
			target[i] = rms * (.45 + .55*confidence)
			if rms < .003 {
				target[i] = 0
			}
			if target[i] > .0005 {
				strengths = append(strengths, rms)
			}
		}
		// Unvoiced laughter remains usable, but is still pulse-driven, never timed
		// from an invented viseme. The fallback is explicit in derived evidence.
		if len(strengths) == 0 {
			for i := range target {
				target[i] = energy[i]
				strengths = append(strengths, target[i])
			}
		}
		sort.Float64s(strengths)
		normal := .0005
		if len(strengths) > 0 {
			normal = math.Max(normal, strengths[int(float64(len(strengths)-1)*.8)])
		}
		previous, face := 0., 0.
		for i, t := range target {
			t = unit(t / normal)
			if t < .08 {
				t = 0
			}
			tau := .08
			if t > previous {
				tau = .02
			}
			previous = blend(previous, t, 1-math.Exp(-.02/tau))
			a.frames[i].JawOpen = previous
			if t > .15 {
				a.lastBurst = (float64(i) + 1) * .02
			}
			goal := .2 + .8*previous
			tau = .16
			if goal > face {
				tau = .04
			}
			face = blend(face, goal, 1-math.Exp(-.02/tau))
			a.frames[i].LaughFace = face
		}
		for i := range a.frames {
			time := float64(i) * .02
			end := a.lastBurst + .24
			release := 1 - ease((time-a.lastBurst)/.24)
			if time >= end {
				release = 0
			}
			a.frames[i].LaughFace *= release
		}
		if c.LaughterV2 {
			// Control samples, not extra PCM: retain the complete final burst.
			for i := 0; i < 16; i++ {
				previous *= math.Exp(-.02 / .08)
				a.frames = append(a.frames, ArticulationFrame{JawOpen: previous})
			}
			for i := range a.frames {
				time := float64(i) * .02
				a.frames[i].LaughFace = ease(time/.12) * (1 - ease((time-a.lastBurst)/.32))
			}
		}
		return a, nil
	}
	for i := range a.frames {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		time := float64(i) * .02
		m := MouthAt(timeline, time)
		jaw := poseJaw(c, m)
		// Preserve a syllabic jaw trajectory through a brief lip-contact consonant.
		for j, e := range timeline {
			if !mouthContact(e.Pose) || time < e.Time {
				continue
			}
			end := float64(len(samples)) / float64(rate)
			if j+1 < len(timeline) {
				end = timeline[j+1].Time
			}
			if time >= end || end-e.Time >= .12 {
				continue
			}
			before, after := 0., 0.
			if j > 0 {
				before = c.Mandible.PoseJaw[timeline[j-1].Pose]
			}
			if j+1 < len(timeline) {
				after = c.Mandible.PoseJaw[timeline[j+1].Pose]
			}
			jaw = blend(before, after, unit((time-e.Time)/math.Max(.001, end-e.Time)))
		}
		mean := 0.
		count := 0
		for j := max(0, i-4); j < min(len(energy), i+5); j++ {
			mean += energy[j]
			count++
		}
		mean /= float64(max(1, count))
		residual := math.Max(-.05, math.Min(.05, (energy[i]-mean)*.05))
		a.frames[i] = ArticulationFrame{JawOpen: unit(jaw + residual)}
		if !mouthContact(m.From) && !mouthContact(m.To) && m.From != "Rest" && m.To != "Rest" {
			a.frames[i].LipOpening = residual
			a.frames[i].LipTension = residual * .5
		}
	}
	// Centered smoothing does not shift existing contact/viseme arrival times.
	original := append([]ArticulationFrame(nil), a.frames...)
	for i := range a.frames {
		lo, hi := max(0, i-1), min(len(original)-1, i+1)
		a.frames[i].JawOpen = (original[lo].JawOpen + 2*original[i].JawOpen + original[hi].JawOpen) * .25
	}
	return a, nil
}
