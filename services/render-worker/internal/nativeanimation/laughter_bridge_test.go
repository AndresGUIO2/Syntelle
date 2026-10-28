package nativeanimation

import (
	"math"
	"testing"

	"syntelle/render-contract"
)

func TestLaughterReturnsToSelectedExpressionContinuously(t *testing.T) {
	for _, tc := range []struct {
		name, mood               string
		intensity, smile, amount float64
	}{
		{"playful half", "playful", .5, 0, 1},
		{"playful full with smile", "playful", 1, .5, 1},
		{"joyful restrained laugh", "joyful", .5, 0, .4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{Expressions: true, Emotions: true, SmileAvailable: true, SpeakingSmile: true, LaughAvailable: true, ReferenceLaugh: true, OralRecovery: true, Directed: true}
			samples := make([]float32, 16000)
			for i := range samples {
				samples[i] = .7
			}
			prepared, err := PrepareSpeech(t.Context(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, c)
			if err != nil {
				t.Fatal(err)
			}
			speech := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Duration: 1, Amount: &tc.amount}}})
			settings := DefaultSettings()
			settings.Movement, settings.Blinking, settings.Smile = false, false, tc.smile
			settings.EmotionBaseline = rendercontract.EmotionBaseline{State: tc.mood, Intensity: tc.intensity}
			e := NewEngine(c)
			e.Sample(0, SpeechState{}, settings)
			baseline := e.Sample(1000, SpeechState{}, settings)
			previous := baseline
			for i := 0; i < 30; i++ {
				position := float64(i) / 30
				f := e.Sample(1000+position*1000, SpeechState{Active: true, Position: position, Session: 1, Speech: speech}, settings)
				for state, weight := range f.Emotion {
					if math.Abs(weight-previous.Emotion[state]) > .25 {
						t.Fatal("expression jumped during release", position, previous.Emotion, f.Emotion)
					}
				}
				previous = f
			}
			// The reference renderer rounds this laugh to zero. Its first
			// ordinary frame must start at the shared neutral geometry.
			handoff := e.Sample(2000, SpeechState{Speech: speech, Position: 1, Session: 1}, settings)
			if handoff.Smile > .02 {
				t.Fatal("full expression appeared at the reference handoff", handoff)
			}
			for _, weight := range handoff.Emotion {
				if weight > .02 {
					t.Fatal("emotional geometry popped on at handoff", handoff)
				}
			}
			settings.Paused = true
			if frozen := e.Sample(2000, SpeechState{}, settings); frozen != handoff {
				t.Fatal("pause advanced the expression return")
			}
			settings.Paused = false
			for _, ms := range []float64{2033, 2100, 2240, 2320, 2500} {
				e.Sample(ms, SpeechState{Speech: speech, Position: 1, Session: 1}, settings)
			}
			restored := e.Sample(2600, SpeechState{}, settings)
			if restored.Emotion != baseline.Emotion || math.Abs(restored.Smile-baseline.Smile) > 1e-5 {
				t.Fatal("selected expression did not restore", baseline, restored)
			}
			if settings.EmotionBaseline.State != tc.mood {
				t.Fatal("presentation changed the selected expression")
			}
		})
	}
}

func TestLaughterExpressionReturnDoesNotDelayNewSpeech(t *testing.T) {
	c := Config{Expressions: true, Emotions: true, SpeakingSmile: true, ReferenceLaugh: true, OralRecovery: true, Directed: true}
	e := NewEngine(c)
	e.laughBridge = laughterBridge{active: true, returning: true, from: 0, factor: 0, at: 1000}
	p := &PreparedSpeech{timeline: []TimelineEvent{{Time: 0, Pose: "MBP"}, {Time: 1, Pose: "Rest"}}, duration: 1, directed: true}
	settings := DefaultSettings()
	settings.Movement, settings.Blinking = false, false
	settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: .5}
	frame := e.Sample(1100, SpeechState{Active: true, Position: .2, Session: 2, Speech: BindSpeech(p, nil)}, settings)
	if frame.Mouth.From != "MBP" || frame.Mouth.Amount != 0 {
		t.Fatal("expression return overrode the new MBP closure", frame.Mouth)
	}
}
