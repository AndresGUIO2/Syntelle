package nativeanimation

import (
	"context"
	"errors"
	"math"
	"sort"
	"syntelle/render-contract"
)

type NativeViseme struct {
	ID       int     `json:"id"`
	OffsetMS float64 `json:"offset_ms"`
}
type Cue struct {
	Time     float64 `json:"time"`
	Kind     string  `json:"kind"`
	Strength float64 `json:"strength"`
}
type Gesture struct {
	Time     float64 `json:"time"`
	Duration float64 `json:"duration"`
	Pitch    float64 `json:"pitch"`
	Yaw      float64 `json:"yaw"`
	Kind     string  `json:"kind"`
	Roll     float64 `json:"roll,omitempty"`
}
type PerformanceCue struct {
	Kind        string   `json:"kind"`
	Time        float64  `json:"time"`
	Duration    float64  `json:"duration"`
	Amount      *float64 `json:"amount,omitempty"`
	Source      string   `json:"source,omitempty"`
	Repetitions *int     `json:"repetitions,omitempty"`
}
type PerformanceAnchor struct {
	Kind        string   `json:"kind"`
	Anchor      string   `json:"anchor"`
	Amount      *float64 `json:"amount,omitempty"`
	Repetitions *int     `json:"repetitions,omitempty"`
}
type PerformanceIntent struct {
	Purpose string                        `json:"purpose,omitempty"`
	Emotion *rendercontract.EmotionTarget `json:"emotion,omitempty"`
	Version string                        `json:"version"`
	Text    string                        `json:"text"`
	Tone    string                        `json:"tone"`
	Cues    []PerformanceAnchor           `json:"cues"`
}
type Performance struct {
	Motion   *rendercontract.MotionTiming `json:"motion,omitempty"`
	Intent   *PerformanceIntent           `json:"intent,omitempty"`
	Omitted  []string                     `json:"omitted,omitempty"`
	Timing   string                       `json:"timing,omitempty"`
	Resolved []PerformanceCue             `json:"resolved"`
	ReplyID  string                       `json:"reply_id,omitempty"`
}
type Plan struct {
	natural     bool         // Application-owned trajectory profile; never decoded from JSON.
	Version     string       `json:"version"`
	Duration    float64      `json:"duration"`
	Cues        []Cue        `json:"cues"`
	Gestures    []Gesture    `json:"gestures"`
	Performance *Performance `json:"performance,omitempty"`
}
type TimelineEvent struct {
	Time float64 `json:"time"`
	Pose string  `json:"pose"`
}
type Speech struct {
	laughter     *preparedLaughter
	articulation *preparedArticulation
	Plan         *Plan           `json:"plan"`
	Timeline     []TimelineEvent `json:"timeline"`
	Duration     float64         `json:"duration"`
	energy       []float64
}

// PreparedSpeech is immutable after preparation. It contains no retained PCM;
// optional performance has not spent the session engine's admission allowance.
type PreparedSpeech struct {
	laughter     *preparedLaughter
	articulation *preparedArticulation
	plan         Plan
	timeline     []TimelineEvent
	duration     float64
	directed     bool
	continuous   bool
	bounds       Bounds
	energy       []float64
	Performance  *Performance
}
type SpeechState struct {
	ReplyID  string  `json:"reply_id"`
	Active   bool    `json:"active"`
	Waiting  bool    `json:"waiting"`
	Position float64 `json:"position"`
	Session  uint64  `json:"session"`
	Speech   *Speech `json:"speech,omitempty"`
}

var azurePoses = [...]string{"Rest", "A", "A", "O", "EI", "EI", "EI", "U", "O", "A", "O", "A", "Rest", "CHSHR", "EI", "SZ", "CHSHR", "CHSHR", "FV", "EI", "Rest", "MBP"}

const VisemePacingVersion = "bounded-articulation/4"

func Timeline(events []NativeViseme, durationMS float64) ([]TimelineEvent, error) {
	return timeline(context.Background(), events, durationMS)
}
func timeline(ctx context.Context, events []NativeViseme, durationMS float64, nonverbal ...bool) ([]TimelineEvent, error) {
	previous := -1.0
	timeline := []TimelineEvent{}
	spoken := false
	for i, event := range events {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if event.ID < 0 || event.ID >= len(azurePoses) || math.IsNaN(event.OffsetMS) || math.IsInf(event.OffsetMS, 0) || event.OffsetMS < previous || event.OffsetMS > durationMS+rendercontract.MaxNativeVisemeTailMS {
			return nil, errors.New("invalid native Azure viseme timeline")
		}
		previous = event.OffsetMS
		pose := azurePoses[event.ID]
		if pose != "Rest" {
			spoken = true
		}
		if len(timeline) == 0 || timeline[len(timeline)-1].Pose != pose {
			timeline = append(timeline, TimelineEvent{event.OffsetMS / 1000, pose})
		}
	}
	allowRest := len(nonverbal) > 0 && nonverbal[0] && durationMS > 0 && durationMS <= 2500 && len(events) >= 2
	if !spoken && !allowRest {
		return nil, errors.New("dragon HD returned no spoken visemes")
	}
	// Suppress fleeting targets without shifting retained events off the PCM clock.
	end := durationMS / 1000
	paced := make([]TimelineEvent, 0, len(timeline)+1)
	for i, event := range timeline {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		next := end
		if i+1 < len(timeline) {
			next = math.Min(end, timeline[i+1].Time)
		}
		minimum := .06
		if event.Pose == "MBP" || event.Pose == "FV" {
			minimum = .035
		}
		if i != 0 && next-event.Time+1e-9 < minimum {
			continue
		}
		if len(paced) == 0 || paced[len(paced)-1].Pose != event.Pose {
			paced = append(paced, event)
		}
	}
	return append(paced, TimelineEvent{end, "Rest"}), nil
}
func MouthAt(timeline []TimelineEvent, seconds float64) Mouth {
	// Anchor arrivals to sound times; distribute travel before the next arrival
	// instead of reaching a pose quickly and parking until its successor starts.
	previous, previousPeak := "Rest", 0.0
	for i, event := range timeline {
		peak := mouthPeak(event)
		if i == 0 {
			previousPeak = peak - .07
		}
		if seconds <= peak {
			maximum, hold := .14, 0.0
			if previous == "Rest" || event.Pose == "Rest" {
				maximum = .07
			} else if mouthContact(event.Pose) {
				maximum = .07
			}
			if mouthContact(previous) {
				hold = .015
			}
			duration := math.Max(.001, math.Min(maximum, peak-previousPeak-hold))
			start := peak - duration
			if seconds < start || previous == event.Pose {
				return Mouth{previous, previous, 0}
			}
			return Mouth{previous, event.Pose, smooth((seconds - start) / duration)}
		}
		previous, previousPeak = event.Pose, peak
	}
	return Mouth{previous, previous, 0}
}

func mouthContact(pose string) bool { return pose == "MBP" || pose == "FV" }
func mouthPeak(event TimelineEvent) float64 {
	if event.Pose == "Rest" || mouthContact(event.Pose) {
		return event.Time
	}
	return event.Time + .015
}
func AcousticCues(samples []float32, sampleRate int) []Cue {
	return acousticCues(context.Background(), samples, sampleRate)
}
func acousticCues(ctx context.Context, samples []float32, sampleRate int, envelope ...*[]float64) []Cue {
	if sampleRate <= 0 || len(samples) == 0 {
		return []Cue{}
	}
	step := max(1, int(math.Floor(float64(sampleRate)*.02+.5)))
	energy := []float64{}
	peak := 0.0
	for i := 0; i < len(samples); i += step {
		if ctx.Err() != nil {
			return nil
		}
		length := min(step, len(samples)-i)
		sum := 0.0
		for _, value := range samples[i : i+length] {
			sum += float64(value) * float64(value)
		}
		rms := math.Sqrt(sum / float64(length))
		energy = append(energy, rms)
		peak = math.Max(peak, rms)
	}
	if len(envelope) > 0 {
		values := make([]float64, len(energy))
		previous := 0.0
		for i, value := range energy {
			target := 0.0
			if peak >= .005 {
				target = unit(value / peak)
			}
			weight := .35
			if target > previous {
				weight = .55
			}
			previous = blend(previous, target, weight)
			values[i] = previous
		}
		*envelope[0] = values
	}
	if peak < .005 {
		return []Cue{}
	}
	seconds := float64(step) / float64(sampleRate)
	threshold := math.Max(.003, peak*.09)
	voiced := make([]bool, len(energy))
	for i, value := range energy {
		voiced[i] = value >= threshold
	}
	for i := 0; i < len(voiced); {
		if voiced[i] {
			i++
			continue
		}
		start := i
		for i < len(voiced) && !voiced[i] {
			i++
		}
		if start > 0 && i < len(voiced) && float64(i-start)*seconds < .16 {
			for j := start; j < i; j++ {
				voiced[j] = true
			}
		}
	}
	type region struct{ start, end float64 }
	regions := []region{}
	for i := 0; i < len(voiced); {
		if !voiced[i] {
			i++
			continue
		}
		start := i
		for i < len(voiced) && voiced[i] {
			i++
		}
		if float64(i-start)*seconds >= .1 {
			regions = append(regions, region{float64(start) * seconds, float64(i) * seconds})
		}
	}
	if len(regions) == 0 {
		return []Cue{}
	}
	cues := []Cue{{regions[0].start, "onset", .5}}
	for i := 1; i < len(regions); i++ {
		gap := regions[i].start - regions[i-1].end
		if gap >= .32 {
			cues = append(cues, Cue{regions[i-1].end + .1, "pause", math.Min(1, gap/.8)})
		}
	}
	lastAccent := -2.0
	for i := 4; i < len(energy)-4; i++ {
		value := energy[i]
		time := float64(i) * seconds
		if !voiced[i] || time-lastAccent < 1.5 || value < peak*.65 {
			continue
		}
		sum := 0.0
		higher := false
		for _, other := range energy[i-4 : i+5] {
			sum += other
			if other > value {
				higher = true
			}
		}
		if higher || value < sum/9*1.08 {
			continue
		}
		cues = append(cues, Cue{time, "emphasis", value / peak})
		lastAccent = time
	}
	cues = append(cues, Cue{math.Max(regions[0].start, regions[len(regions)-1].end-.12), "ending", .8})
	sort.SliceStable(cues, func(i, j int) bool { return cues[i].Time < cues[j].Time })
	return cues
}
func BuildSpeech(samples []float32, sampleRate int, visemes []NativeViseme, seed uint32, config Config) (*Speech, error) {
	return BuildSpeechWithPerformance(samples, sampleRate, visemes, seed, config, nil)
}

// Performance cues must already pass the same admission policy as the private
// player. Speech preparation never infers semantic gestures from audio.
func BuildSpeechWithPerformance(samples []float32, sampleRate int, visemes []NativeViseme, seed uint32, config Config, performance *Performance) (*Speech, error) {
	prepared, err := PrepareSpeech(context.Background(), samples, sampleRate, visemes, seed, config)
	if err != nil {
		return nil, err
	}
	return BindSpeech(prepared, performance), nil
}

// PrepareSpeech performs the PCM-sized work without touching an Engine. Its
// caller owns the input during this call; the result does not retain samples.
func PrepareSpeech(ctx context.Context, samples []float32, sampleRate int, visemes []NativeViseme, seed uint32, config Config, nonverbal ...bool) (*PreparedSpeech, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sampleRate <= 0 || len(samples) == 0 {
		return nil, errors.New("empty speech PCM")
	}
	duration := float64(len(samples)) / float64(sampleRate)
	timeline, err := timeline(ctx, visemes, duration*1000, nonverbal...)
	if err != nil {
		return nil, err
	}
	var energy []float64
	cues := acousticCues(ctx, samples, sampleRate, &energy)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	gestures := []Gesture{}
	if !config.Directed {
		random := random{seed}
		candidates := []Cue{}
		for _, cue := range cues {
			if cue.Kind == "pause" || cue.Kind == "ending" {
				candidates = append(candidates, cue)
			}
		}
		for _, cue := range cues {
			if cue.Kind == "emphasis" {
				candidates = append(candidates, cue)
			}
		}
		for _, cue := range candidates {
			chance := .5
			if cue.Kind == "emphasis" {
				chance = .22
			} else if cue.Kind == "ending" {
				chance = .6
			}
			draw := random.next()
			near := false
			for _, g := range gestures {
				if math.Abs(g.Time-cue.Time) < config.Behavior.Head.MinimumGestureGap {
					near = true
				}
			}
			if draw >= chance || cue.Time < .35 || near {
				continue
			}
			duration := 1.2 + random.next()*.3
			pitch := -config.Behavior.Head.MaximumNod * (.55 + cue.Strength*.45)
			yaw := 0.0
			if random.next() < .2 {
				yaw = (random.next()*2 - 1) * config.Behavior.Head.MaximumTurn
			}
			gestures = append(gestures, Gesture{math.Max(0, cue.Time-.16), duration, pitch, yaw, cue.Kind, 0})
		}
		sort.SliceStable(gestures, func(i, j int) bool { return gestures[i].Time < gestures[j].Time })
		for _, g := range gestures {
			cues = append(cues, Cue{g.Time + g.Duration*.3, "nod", .5})
		}
		sort.SliceStable(cues, func(i, j int) bool { return cues[i].Time < cues[j].Time })
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var articulation *preparedArticulation
	if config.Mandible != nil {
		articulation, err = prepareArticulation(ctx, samples, sampleRate, visemes, timeline, energy, config, len(nonverbal) > 0 && nonverbal[0])
		if err != nil {
			return nil, err
		}
	}
	return &PreparedSpeech{articulation: articulation, plan: Plan{Version: "speech-context/1", Duration: duration, Cues: cues, Gestures: gestures}, timeline: timeline, duration: duration, directed: config.Directed, continuous: config.ConversationalMotion, bounds: config.Bounds, energy: energy}, nil
}

// BindSpeech attaches only already-admitted performance. It shares immutable
// timeline/acoustic data, and never scans PCM or mutates the prepared result.
func BindSpeech(prepared *PreparedSpeech, performance *Performance) *Speech {
	plan := prepared.plan
	plan.Performance = performance
	plan.natural = prepared.directed && performance != nil && performance.Intent != nil && rendercontract.NaturalPerformance(performance.Intent.Version)
	if prepared.directed {
		plan.Gestures = []Gesture{}
		if performance != nil {
			for _, cue := range performance.Resolved {
				if !isHeadCue(cue.Kind) || (!(prepared.continuous && performance.Motion.Valid()) && cue.Time+cue.Duration > prepared.duration) {
					continue
				}
				if plan.natural {
					plan.Gestures = append(plan.Gestures, naturalGestures(cue, prepared.bounds, SpeechSeed(performance.Intent.Text), len(plan.Gestures))...)
					continue
				}
				pitch, yaw, roll := -1.05, 0.0, 0.
				duration := cue.Duration
				if performance.Intent != nil && rendercontract.DirectedPerformance(performance.Intent.Version) {
					if cue.Kind == "nod" {
						pitch = -1.5
						duration = math.Min(duration, rendercontract.NodCycleSeconds*float64(rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)))
						pitch = -3.8
					}
					if cue.Kind == "double_nod" {
						pitch = -1.5
						duration = math.Min(duration, 1.12)
					}
				}
				if cue.Kind == "shake" {
					pitch, yaw = 0, 1.5
					if performance.Intent != nil && rendercontract.DirectedPerformance(performance.Intent.Version) && cue.Repetitions != nil {
						yaw = prepared.bounds.Yaw * .95
						duration = math.Min(duration, rendercontract.ShakeCycleSeconds*float64(rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)))
					}
				} else if cue.Kind == "emphasis" {
					pitch = -.55
					if performance.Intent != nil && rendercontract.DirectedPerformance(performance.Intent.Version) {
						pitch = -.9
					}
				}
				if cue.Kind == "tilt" {
					pitch = 0
					roll = .3
					if cue.Repetitions != nil {
						roll = prepared.bounds.Roll * .95
					}
				}
				amount := 1.
				if cue.Amount != nil {
					amount = unit(*cue.Amount)
				}
				if (cue.Kind == "nod" || cue.Kind == "shake" && cue.Repetitions != nil) && performance.Intent != nil && rendercontract.DirectedPerformance(performance.Intent.Version) {
					count := rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)
					if count == 0 {
						continue
					}
					cycle := duration / float64(count)
					for n := 0; n < count; n++ {
						kind := cue.Kind
						if kind == "shake" {
							kind = "shake_cycle"
						}
						plan.Gestures = append(plan.Gestures, Gesture{cue.Time + float64(n)*cycle, cycle, pitch * amount, yaw * amount, kind, 0})
					}
				} else {
					plan.Gestures = append(plan.Gestures, Gesture{cue.Time, duration, pitch * amount, yaw * amount, cue.Kind, roll * amount})
				}
			}
		}
	}
	return &Speech{laughter: prepared.laughter, articulation: prepared.articulation, Plan: &plan, Timeline: prepared.timeline, Duration: prepared.duration, energy: prepared.energy}
}
func gestureAt(plan *Plan, position float64) Head {
	result := Head{}
	if plan == nil {
		return result
	}
	for _, g := range plan.Gestures {
		phase := (position - g.Time) / g.Duration
		if phase <= 0 || phase >= 1 {
			continue
		}
		if plan.natural {
			h := naturalGestureAt(g, phase)
			result.Pitch += h.Pitch
			result.Yaw += h.Yaw
			result.Roll += h.Roll
			continue
		}
		if g.Kind == "shake_cycle" {
			yaw := 0.
			if phase < .25 {
				yaw = ease(phase / .25)
			} else if phase < .75 {
				yaw = 1 - 2*ease((phase-.25)/.5)
			} else {
				yaw = -1 + ease((phase-.75)/.25)
			}
			result.Yaw += g.Yaw * yaw
			continue
		}
		amount := 0.0
		if g.Kind == "double_nod" {
			phase = math.Mod(phase*2, 1)
		}
		if phase < .42 {
			amount = ease(phase / .42)
		} else {
			amount = 1 - ease((phase-.42)/.58)
		}
		result.Pitch += g.Pitch * amount
		result.Roll += g.Roll * amount
		if g.Kind == "shake" {
			result.Yaw += g.Yaw * math.Sin(phase*math.Pi*4) * math.Pow(math.Sin(phase*math.Pi), 2)
		} else {
			result.Yaw += g.Yaw * amount
		}
	}
	return result
}
func (s SpeechState) plan() *Plan {
	if s.Speech == nil {
		return nil
	}
	return s.Speech.Plan
}

// EnergyAt samples a bounded 20 ms envelope on the emitted-audio clock. No
// runtime PCM scan, phoneme imitation or independent laughter timer is used.
func (s *Speech) EnergyAt(position float64) float64 {
	if s == nil || len(s.energy) == 0 {
		return 0
	}
	at := math.Max(0, position/.02)
	lo := min(len(s.energy)-1, int(at))
	hi := min(len(s.energy)-1, lo+1)
	return blend(s.energy[lo], s.energy[hi], unit(at-float64(lo)))
}
