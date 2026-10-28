package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func TestCoordinatedAccentIsComposedWithEmotionRatherThanAttenuatedTwice(t *testing.T) {
	_, c, samples := loadReference(t)
	c.Emotions = true
	c.Expressions = true
	c.SpeakingSmile = true
	prepared, err := PrepareSpeech(t.Context(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, c)
	if err != nil {
		t.Fatal(err)
	}
	amount := .6
	performance := &Performance{Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "explain"}, Resolved: []PerformanceCue{{Kind: "emphasis", Time: .4, Duration: .72, Amount: &amount}}}
	speech := BindSpeech(prepared, performance)
	settings := DefaultSettings()
	settings.Blinking = false
	settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: .5}
	e := NewEngine(c)
	e.Sample(0, SpeechState{}, settings)
	f := e.Sample(601.6, SpeechState{Active: true, Position: .6016, Session: 1, Speech: speech}, settings)
	if f.Brows < .28 || f.Brows > .31 {
		t.Fatal("coordinated eyebrow accent lost its qualified strength", f.Brows)
	}
}

func TestCoordinatedGazeUsesFinalHeadAfterEmotionalGain(t *testing.T) {
	_, c, _ := loadReference(t)
	c.Emotions = true
	settings := DefaultSettings()
	settings.Blinking = false
	settings.Activity = "listening"
	weights := rendercontract.EmotionWeights{}
	weights[2] = 1
	settings.EmotionControl = &EmotionSnapshot{To: weights, EnergyTo: .65}
	e := NewEngine(c)
	for ms := 0.; ms < 5000; ms += 100 {
		f := e.Sample(ms, SpeechState{}, settings)
		want := gazeOffsets(c.Eyes, f.Head, [2]float64{})
		if f.Gaze != want {
			t.Fatal("gaze uses an earlier head frame", ms, f.Gaze, want)
		}
	}
}

func TestReassuranceQuietensCompetingMotionWithoutInventingGestures(t *testing.T) {
	_, c, samples := loadReference(t)
	c.Conversational, c.ConversationalMotion = true, true
	prepared, err := PrepareSpeech(t.Context(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, c)
	if err != nil {
		t.Fatal(err)
	}
	quiet := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "reassure"}})
	normal := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "explain"}})
	a, b := newAttention(c.Seed), newAttention(c.Seed)
	quietTravel, normalTravel := 0., 0.
	var lastQ, lastN Head
	for ms := 0.; ms <= 30000; ms += 100 {
		q := a.sample(ms, SpeechState{Active: true, Position: ms / 1000, Session: 1, Speech: quiet}, c)
		n := b.sample(ms, SpeechState{Active: true, Position: ms / 1000, Session: 1, Speech: normal}, c)
		quietTravel += math.Abs(q.Head.Yaw-lastQ.Yaw) + math.Abs(q.Head.Pitch-lastQ.Pitch)
		normalTravel += math.Abs(n.Head.Yaw-lastN.Yaw) + math.Abs(n.Head.Pitch-lastN.Pitch)
		lastQ, lastN = q.Head, n.Head
	}
	if quietTravel >= normalTravel*.8 {
		t.Fatal("reassurance did not reduce competing movement", quietTravel, normalTravel)
	}

	if len(a.motionTrack) != 0 {
		t.Fatal("purpose invented a gesture")
	}
}

func TestQuietPresenceUsesApprovedEnergyRatherThanProposedIntensity(t *testing.T) {
	_, c, samples := loadReference(t)
	c.Conversational, c.ConversationalMotion = true, true
	prepared, err := PrepareSpeech(t.Context(), samples, 16000, []NativeViseme{{ID: 18, OffsetMS: 0}}, 1, c)
	if err != nil {
		t.Fatal(err)
	}
	speech := BindSpeech(prepared, &Performance{Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "explain", Emotion: &rendercontract.EmotionTarget{State: "neutral", Energy: .1}}})
	normal, quiet := newAttention(c.Seed), newAttention(c.Seed)
	aSettings, bSettings := DefaultSettings(), DefaultSettings()
	aSettings.EmotionControl = &EmotionSnapshot{EnergyTo: .8}
	bSettings.EmotionControl = &EmotionSnapshot{EnergyTo: .2}
	var aPrevious, bPrevious Head
	aTravel, bTravel := 0., 0.
	for ms := 0.; ms < 30000; ms += 100 {
		state := SpeechState{Active: true, Session: 1, Speech: speech, Position: ms / 1000}
		a, b := normal.sample(ms, state, c, aSettings).Head, quiet.sample(ms, state, c, bSettings).Head
		aTravel += math.Abs(a.Yaw-aPrevious.Yaw) + math.Abs(a.Pitch-aPrevious.Pitch)
		bTravel += math.Abs(b.Yaw-bPrevious.Yaw) + math.Abs(b.Pitch-bPrevious.Pitch)
		aPrevious, bPrevious = a, b
	}
	if aTravel <= bTravel {
		t.Fatal("approved quiet energy did not govern presence", aTravel, bTravel)
	}
}
