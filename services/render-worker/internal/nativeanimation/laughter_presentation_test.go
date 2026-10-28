package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func presentationFixture(t *testing.T) (Config, *Speech) {
	t.Helper()
	c := articulationConfig()
	c.LaughterV2 = true
	c.Expressions = true
	c.SmileAvailable = true
	c.SpeakingSmile = true
	c.ReferenceLaugh = true
	c.OralRecovery = true
	c.Emotions = true
	c.LaughAvailable = true
	samples := make([]float32, 24000)
	for i := range samples {
		time := float64(i) / 24000
		if time < .25 || time > .7 {
			samples[i] = float32(.3 * math.Sin(2*math.Pi*180*time))
		}
	}
	p, err := PrepareSpeech(t.Context(), samples, 24000, []NativeViseme{{0, 0}, {0, 1000}}, 1, c, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = AttachLaughterSpan(t.Context(), p, samples, 24000, &rendercontract.LaughterSpan{Offset: 0, Count: len(samples)}, c); err != nil {
		t.Fatal(err)
	}
	amount := 1.
	return c, BindSpeech(p, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Duration: 1, Amount: &amount}}})
}
func TestCompleteLaughterKeepsLiveSelectedExpression(t *testing.T) {
	c, s := presentationFixture(t)
	e := NewEngine(c)
	settings := DefaultSettings()
	settings.Movement = false
	settings.Blinking = false
	settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: .5}
	e.Sample(0, SpeechState{}, settings)
	base := e.Sample(1000, SpeechState{}, settings)
	peak := 0.
	for i := 0; i <= 50; i++ {
		position := float64(i) / 30
		active := position < 1
		f := e.Sample(1000+position*1000, SpeechState{Active: active, Position: math.Min(1, position), Session: 1, Speech: s}, settings)
		peak = math.Max(peak, f.Laugh)
		if f.Emotion != base.Emotion {
			t.Fatal("selected expression was erased instead of composed", position, f.Emotion, base.Emotion)
		}
	}
	if peak < .999 {
		t.Fatal("full carcajada was partial", peak)
	}
	if e.last.Laugh != 0 {
		t.Fatal("visual tail did not settle")
	}
}
func TestManualLaughterSurvivesAutomaticOffAndPause(t *testing.T) {
	c, s := presentationFixture(t)
	e := NewEngine(c)
	settings := DefaultSettings()
	settings.AutomaticExpressions = false
	settings.ManualLaughter = true
	settings.Movement = false
	f := e.Sample(200, SpeechState{Active: true, Position: .2, Session: 1, Speech: s}, settings)
	if f.Laugh < .99 {
		t.Fatal("explicit preview was suppressed", f.Laugh)
	}
	settings.Paused = true
	if got := e.Sample(500, SpeechState{}, settings); got != f {
		t.Fatal("pause advanced preview")
	}
}

func TestLaughterMouthBaselineDoesNotPumpWithAudioEnergy(t *testing.T) {
	c, s := presentationFixture(t)
	opening := [10]float64{}
	opening[2] = .14
	c.EmotionRestOpening = &opening
	e := NewEngine(c)
	settings := DefaultSettings()
	settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: .5}
	settings.Movement = false
	e.Sample(0, SpeechState{}, settings)
	e.Sample(1000, SpeechState{}, settings)
	for _, position := range []float64{.2, .4, .8} {
		f := e.Sample(1000+position*1000, SpeechState{Active: true, Session: 1, Speech: s, Position: position}, settings)
		if math.Abs(f.Mouth.Amount-.07) > 1e-9 {
			t.Fatal("reference baseline mouth still contains an acoustic A-pose", position, f.Mouth)
		}
	}
}

func TestLaughterInterruptionAndNewContacts(t *testing.T) {
	c, s := presentationFixture(t)
	settings := DefaultSettings()
	settings.Movement = false
	settings.Blinking = false
	settings.ManualLaughter = true
	e := NewEngine(c)
	peak := e.Sample(200, SpeechState{Active: true, Session: 1, Speech: s, Position: .2}, settings)
	cancelled := e.Sample(220, SpeechState{}, settings)
	if cancelled.Laugh != peak.Laugh || cancelled.JawOpen != peak.JawOpen {
		t.Fatal("interruption did not start from rendered state")
	}
	middle := e.Sample(340, SpeechState{}, settings)
	if middle.Laugh <= 0 || middle.Laugh >= peak.Laugh {
		t.Fatal("missing continuous cancellation", middle.Laugh)
	}
	if end := e.Sample(460, SpeechState{}, settings); end.Laugh != 0 {
		t.Fatal("interruption exceeded 240ms", end.Laugh)
	}
	e = NewEngine(c)
	e.Sample(200, SpeechState{Active: true, Session: 1, Speech: s, Position: .2}, settings)
	samples := make([]float32, 2400)
	p, err := PrepareSpeech(t.Context(), samples, 24000, []NativeViseme{{21, 0}, {0, 100}}, 1, c, false)
	if err != nil {
		t.Fatal(err)
	}
	next := BindSpeech(p, nil)
	frame := e.Sample(220, SpeechState{Active: true, Session: 2, Speech: next, Position: 0}, settings)
	if !frame.LaughSpeech || frame.Mouth != MouthAt(next.Timeline, 0) {
		t.Fatal("laughter delayed following contact", frame.Mouth, MouthAt(next.Timeline, 0))
	}
}
func TestRepeatedPreviewStartsFromVisiblePresentation(t *testing.T) {
	c, s := presentationFixture(t)
	e := NewEngine(c)
	settings := DefaultSettings()
	settings.ManualLaughter = true
	e.Sample(200, SpeechState{Active: true, Session: 1, Speech: s, Position: .2}, settings)
	e.Sample(220, SpeechState{}, settings)
	before := e.Sample(300, SpeechState{}, settings)
	next := e.Sample(300, SpeechState{Active: true, Session: 2, Speech: s, Position: 0}, settings)
	if next.Laugh != before.Laugh || next.JawOpen != before.JawOpen {
		t.Fatal("repeat reset current face", before.Laugh, next.Laugh, before.JawOpen, next.JawOpen)
	}
}
