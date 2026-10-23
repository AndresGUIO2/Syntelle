package nativeanimation

import (
	"context"
	"errors"
	"math"
	"testing"
)

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
	}{{"lip closure", .619999, "MBP"}, {"tooth lip contact", .659999, "FV"}, {"silence", 1.1, "Rest"}} {
		t.Run(tc.name, func(t *testing.T) {
			frame := MouthAt(got, tc.at)
			if frame.To != tc.pose || frame.Amount < .999 {
				t.Fatalf("gesture did not reach its target: %+v", frame)
			}
		})
	}
}

func TestMouthAtSmoothsWithoutAccumulatingDelay(t *testing.T) {
	t.Parallel()
	timeline := []TimelineEvent{{0, "A"}, {.3, "O"}, {.4, "EI"}, {.6, "Rest"}}
	frame := MouthAt(timeline, .315)
	if math.Abs(frame.Amount-.5) > 1e-9 {
		t.Fatalf("gesture should be halfway 15ms after its audio event: %+v", frame)
	}
	if early := MouthAt(timeline, .279); early.To != "A" {
		t.Fatalf("gesture anticipates its event by more than 20ms: %+v", early)
	}
	if reached := MouthAt(timeline, .35); reached.To != "O" || reached.Amount < .999999 {
		t.Fatalf("gesture still trails its event after 50ms: %+v", reached)
	}
	before := MouthAt(timeline, .38-1e-8)
	after := MouthAt(timeline, .38)
	if before.To != after.From || before.Amount < .999999 || after.Amount != 0 {
		t.Fatalf("boundary jumps: %+v -> %+v", before, after)
	}
	// Sampling out of order (seek/resume) must not depend on frame history.
	_ = MouthAt(timeline, 1)
	if MouthAt(timeline, .315) != frame {
		t.Fatal("pacing depends on sampling history")
	}
	if MouthAt(timeline, 10).To != "Rest" {
		t.Fatal("mouth did not return to rest")
	}
}
