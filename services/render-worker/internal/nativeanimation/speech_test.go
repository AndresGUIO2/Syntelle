package nativeanimation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestNonverbalRestTimelineRemainsSeparateFromSpeech(t *testing.T) {
	events := []NativeViseme{{ID: 0, OffsetMS: 0}, {ID: 0, OffsetMS: 400}}
	if _, err := timeline(t.Context(), events, 800); err == nil {
		t.Fatal("ordinary speech accepted only Rest")
	}
	if _, err := timeline(t.Context(), events, 800, true); err != nil {
		t.Fatal(err)
	}
	if _, err := timeline(t.Context(), events, 2600, true); err == nil {
		t.Fatal("nonverbal duration gate weakened")
	}
}

func TestBuildSpeechMatchesOriginalTypeScript(t *testing.T) {
	t.Parallel()
	ref, c, samples := loadReference(t)
	t.Run("directed current worker", func(t *testing.T) {
		speech, err := BuildSpeechWithPerformance(samples, ref.Speech.SampleRate, ref.Speech.Visemes, 123456789, c, ref.Speech.Performance)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONNumbers(t, speech.Plan, ref.Speech.Plan)
		assertJSONNumbers(t, speech.Timeline, ref.Speech.Timeline)
	})
	t.Run("acoustic contextual candidate", func(t *testing.T) {
		c.Directed = false
		speech, err := BuildSpeech(samples, ref.Speech.SampleRate, ref.Speech.Visemes, 123456789, c)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONNumbers(t, speech.Plan, ref.Speech.AutonomousPlan)
	})
}

func TestPreparedSpeechBindsOriginalPlanWithoutRetainingPCM(t *testing.T) {
	t.Parallel()
	ref, config, samples := loadReference(t)
	prepared, err := PrepareSpeech(t.Context(), samples, ref.Speech.SampleRate, ref.Speech.Visemes, 123456789, config)
	if err != nil {
		t.Fatal(err)
	}
	clear(samples)
	first := BindSpeech(prepared, ref.Speech.Performance)
	_ = BindSpeech(prepared, nil)
	assertJSONNumbers(t, first.Plan, ref.Speech.Plan)
	assertJSONNumbers(t, first.Timeline, ref.Speech.Timeline)
	if prepared.plan.Performance != nil || len(prepared.plan.Gestures) != 0 {
		t.Fatal("binding mutated the prepared directed analysis")
	}
}

// Cancel at a deterministic interior checkpoint rather than depending on PCM
// processing speed or a sleep racing the analysis on another processor.
type cancelDuringAnalysis struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *cancelDuringAnalysis) Err() error {
	c.checks++
	if c.checks == 4 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestPrepareSpeechObservesCancellationDuringPCMAnalysis(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	check := &cancelDuringAnalysis{Context: ctx, cancel: cancel}
	_, err := PrepareSpeech(check, make([]float32, 24000), 24000, []NativeViseme{{ID: 1}}, 17, Config{Directed: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("analysis ignored cancellation: %v", err)
	}
}
func TestTimelineRejectsInvalidSpeech(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		events []NativeViseme
	}{{"missing spoken articulation", []NativeViseme{{0, 0}}}, {"unknown Azure id", []NativeViseme{{25, 0}}}, {"reversed offsets", []NativeViseme{{1, 100}, {2, 50}}}, {"nonfinite clock", []NativeViseme{{1, math.NaN()}}}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Timeline(tc.events, 500); err == nil {
				t.Fatal("invalid timeline accepted")
			}
		})
	}
}

func TestTimelinePacesBriefTargetsAndPreservesContacts(t *testing.T) {
	t.Parallel()
	// Brief vowels/silence must not interrupt a stable vowel. Audible lip
	// closures and tooth/lip contacts retain their original audio timestamps.
	events := []NativeViseme{{1, 0}, {7, 200}, {1, 220}, {0, 400}, {1, 420}, {21, 600}, {18, 640}, {4, 680}, {0, 900}}
	got, err := Timeline(events, 1200)
	if err != nil {
		t.Fatal(err)
	}
	want := []TimelineEvent{{0, "A"}, {.6, "MBP"}, {.64, "FV"}, {.68, "EI"}, {.9, "Rest"}, {1.2, "Rest"}}
	assertJSONNumbers(t, got, want)
	for _, tc := range []struct {
		name string
		at   float64
		pose string
	}{{"lip closure", .6, "MBP"}, {"tooth lip contact", .64, "FV"}, {"silence", 1.1, "Rest"}} {
		t.Run(tc.name, func(t *testing.T) {
			frame := MouthAt(got, tc.at)
			if frame.To != tc.pose || (frame.From != frame.To && frame.Amount < .999) {
				t.Fatalf("gesture did not reach its target: %+v", frame)
			}
		})
	}
}

func TestMouthAtLinksAudioAnchoredTargets(t *testing.T) {
	t.Parallel()
	timeline := []TimelineEvent{{0, "A"}, {.1, "O"}, {.2, "EI"}, {.4, "Rest"}}
	frame := MouthAt(timeline, .065)
	if math.Abs(frame.Amount-.5) > 1e-9 {
		t.Fatalf("travel should span the interval between arrivals: %+v", frame)
	}
	for _, arrival := range []struct {
		at   float64
		pose string
	}{{.015, "A"}, {.115, "O"}, {.215, "EI"}, {.4, "Rest"}} {
		f := MouthAt(timeline, arrival.at)
		if mouthPoseWeight(f, arrival.pose) < .999999 {
			t.Fatalf("late target at %v: %+v", arrival.at, f)
		}
		before, after := MouthAt(timeline, arrival.at-1e-8), MouthAt(timeline, arrival.at+1e-8)
		for _, pose := range []string{"Rest", "A", "O", "EI"} {
			if math.Abs(mouthPoseWeight(before, pose)-mouthPoseWeight(after, pose)) > 1e-5 {
				t.Fatalf("pose weight jumps at %v: %+v -> %+v", arrival.at, before, after)
			}
		}
	}
	// Sampling out of order (seek/resume) must not depend on frame history.
	_ = MouthAt(timeline, 1)
	if MouthAt(timeline, .065) != frame {
		t.Fatal("pacing depends on sampling history")
	}
	if MouthAt(timeline, 10).To != "Rest" {
		t.Fatal("mouth did not return to rest")
	}
}

func mouthPoseWeight(frame Mouth, pose string) float64 {
	weight := 0.0
	if frame.From == pose {
		weight += 1 - frame.Amount
	}
	if frame.To == pose {
		weight += frame.Amount
	}
	return weight
}

func TestMouthAtRetainsContactAndLongHolds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		timeline []TimelineEvent
		at       float64
		pose     string
	}{
		{"brief lip seal", []TimelineEvent{{0, "A"}, {.1, "MBP"}, {.135, "EI"}}, .11, "MBP"},
		{"sustained vowel", []TimelineEvent{{0, "A"}, {.5, "O"}}, .2, "A"},
		{"silence before speech", []TimelineEvent{{0, "Rest"}, {.5, "A"}}, .4, "Rest"},
		{"empty timeline", nil, .1, "Rest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if frame := MouthAt(tc.timeline, tc.at); mouthPoseWeight(frame, tc.pose) < .999999 {
				t.Fatalf("lost required hold: %+v", frame)
			}
		})
	}
}
