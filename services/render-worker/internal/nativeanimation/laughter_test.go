package nativeanimation

import (
	"context"
	"math"
	"testing"
)

func TestReferenceLaughterJawSettlesBeforeReferenceEnds(t *testing.T) {
	config := Config{Expressions: true, SmileAvailable: true, SpeakingSmile: true, LaughAvailable: true, ReferenceLaugh: true, OralRecovery: true, Directed: true}
	samples := make([]float32, 16000)
	for i := range samples {
		samples[i] = .7 // Include a loud tail: PCM completion cannot guarantee silence.
	}
	prepared, err := PrepareSpeech(t.Context(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, config)
	if err != nil {
		t.Fatal(err)
	}
	amount := 1.
	speech := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Duration: 1, Amount: &amount}}})
	e := NewEngine(config)
	settings := DefaultSettings()
	settings.Movement, settings.Blinking = false, false
	for _, position := range []float64{.65, .9, .95, .98, .99, .999} {
		f := e.Sample(position*1000, SpeechState{Active: true, Position: position, Session: 1, Speech: speech}, settings)
		if f.Mouth.Amount > f.Laugh+1e-9 {
			t.Fatalf("jaw outlived the closing laugh envelope at %.3f: laugh=%g jaw=%g", position, f.Laugh, f.Mouth.Amount)
		}
		if math.Round(f.Laugh*120) == 0 && f.Mouth.Amount > 1./240 {
			t.Fatal("reference handoff opened the ordinary A pose", f)
		}
	}
	last := e.Sample(1000, SpeechState{}, settings)
	if last.Mouth.Amount > 1./240 {
		t.Fatal("completion recovery reopened the jaw", last)
	}
}

func TestReferenceLaughterCompletionKeepsManualSmile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		smile float64
	}{{"neutral", 0}, {"half", .5}, {"full", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{Expressions: true, SmileAvailable: true, SpeakingSmile: true, LaughAvailable: true, ReferenceLaugh: true, OralRecovery: true, Directed: true}
			samples := make([]float32, 16000)
			for i := range samples {
				samples[i] = .7
			}
			prepared, err := PrepareSpeech(t.Context(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, c)
			if err != nil {
				t.Fatal(err)
			}
			amount := 1.
			speech := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Duration: 1, Amount: &amount}}})
			e := NewEngine(c)
			settings := DefaultSettings()
			settings.Smile, settings.Movement, settings.Blinking = tc.smile, false, false
			e.Sample(0, SpeechState{}, settings)
			e.Sample(1000, SpeechState{}, settings)
			for i := 0; i < 30; i++ {
				position := float64(i) / 30
				e.Sample(1000+position*1000, SpeechState{Active: true, Position: position, Session: 1, Speech: speech}, settings)
			}
			for _, ms := range []float64{2000, 2033, 2120, 2240} {
				got := e.Sample(ms, SpeechState{Speech: speech, Position: speech.Duration, Session: 1}, settings)
				if math.Abs(got.Smile-tc.smile) > 1e-6 {
					t.Fatalf("completion dipped the manual smile at %.0f ms: want %g, got %g", ms, tc.smile, got.Smile)
				}
			}
		})
	}
}

func TestLaughterUsesAudioEnergyInsteadOfSpeechVisemes(t *testing.T) {
	samples := make([]float32, 16000)
	for i := range samples {
		if i < 8000 {
			samples[i] = .7
		} else {
			samples[i] = .04
		}
	}
	config := Config{Expressions: true, SmileAvailable: true, SpeakingSmile: true, LaughAvailable: true, Directed: true}
	prepared, err := PrepareSpeech(context.Background(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, config)
	if err != nil {
		t.Fatal(err)
	}
	amount := 1.0
	speech := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Duration: 1, Amount: &amount}}})
	e := NewEngine(config)
	loud := e.Sample(350, SpeechState{Active: true, Position: .35, Session: 1, Speech: speech}, DefaultSettings())
	quiet := e.Sample(650, SpeechState{Active: true, Position: .65, Session: 1, Speech: speech}, DefaultSettings())
	if loud.Mouth.From != "Rest" || loud.Mouth.To != "A" || loud.Mouth.Amount > .06 || loud.Mouth.Amount <= quiet.Mouth.Amount {
		t.Fatal("laughter did not follow the bounded audio envelope", loud.Mouth, quiet.Mouth)
	}
	// Laughter squint belongs to the lower-lid renderer, not the blink clock.
	if loud.EyeClosure != 0 {
		t.Fatal("laughter incorrectly lowered the blink lid")
	}
}

func TestReferenceLaughterKeepsManualBaselineAndIndependentAudioJaw(t *testing.T) {
	for _, tc := range []struct {
		name  string
		smile float64
	}{{"neutral", 0}, {"half smile", .5}, {"full smile", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			samples := make([]float32, 16000)
			for i := range samples {
				if i < 8000 {
					samples[i] = .7
				} else {
					samples[i] = .02
				}
			}
			config := Config{Expressions: true, SmileAvailable: true, SpeakingSmile: true, LaughAvailable: true, ReferenceLaugh: true, Directed: true}
			prepared, err := PrepareSpeech(context.Background(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, config)
			if err != nil {
				t.Fatal(err)
			}
			amount := 1.
			speech := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Duration: 1, Amount: &amount}}})
			settings := DefaultSettings()
			settings.Smile = tc.smile
			engine := NewEngine(config)
			loud := engine.Sample(350, SpeechState{Active: true, Position: .35, Session: 1, Speech: speech}, settings)
			quiet := engine.Sample(650, SpeechState{Active: true, Position: .65, Session: 1, Speech: speech}, settings)
			if loud.Smile != tc.smile || quiet.Smile != tc.smile {
				t.Fatal("laughter replaced the latest manual baseline", loud.Smile, quiet.Smile)
			}
			if loud.Laugh != 1 || quiet.Laugh != 1 {
				t.Fatal("audio energy weakened the independent reference expression")
			}
			if loud.Mouth.Amount <= .06 || loud.Mouth.Amount > 1 || loud.Mouth.Amount <= quiet.Mouth.Amount {
				t.Fatal("reference jaw did not expose normalized audio energy", loud.Mouth, quiet.Mouth)
			}
		})
	}
}
