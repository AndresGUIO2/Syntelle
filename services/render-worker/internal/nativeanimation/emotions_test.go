package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func directedEmotion(state string, session uint64) SpeechState {
	return SpeechState{Active: true, ReplyID: "reply-one", Session: session, Speech: &Speech{Plan: &Plan{Performance: &Performance{Intent: &PerformanceIntent{Version: "performance-intent/3", Emotion: &rendercontract.EmotionTarget{State: state, Intensity: .8, Energy: .5}}}}}}
}
func TestEmotionBaselineContextAndReturn(t *testing.T) {
	d := emotionDirector{}
	settings := Settings{AutomaticExpressions: true, EmotionBaseline: rendercontract.EmotionBaseline{State: "joyful", Intensity: .6}}
	idle := SpeechState{}
	d.sample(0, idle, settings)
	base := d.sample(900, idle, settings)
	if math.Abs(base[1]-.6) > 1e-9 {
		t.Fatal(base)
	}
	speaking := directedEmotion("sad", 1)
	start := d.sample(1000, speaking, settings)
	if start != base {
		t.Fatal("abrupt mood switch", start, base)
	}
	sad := d.sample(1500, speaking, settings)
	if math.Abs(sad[5]-.8) > 1e-9 || sad[1] != 0 {
		t.Fatal(sad)
	}
	waiting := SpeechState{Waiting: true, ReplyID: speaking.ReplyID, Session: 1}
	if d.sample(2000, waiting, settings) != sad {
		t.Fatal("phrase gap reset mood")
	}
	speaking.Session = 2
	speaking.Speech.Plan.Performance.Intent.Emotion = nil
	if d.sample(2500, speaking, settings) != sad {
		t.Fatal("missing direction erased previous target")
	}
	d.sample(3000, idle, settings)
	after := d.sample(3900, idle, settings)
	if after != base {
		t.Fatal("baseline did not return", after, base)
	}
}

func TestEmotionBaselineEditSuppressesRemainingReplyWithoutStoppingSpeech(t *testing.T) {
	d := emotionDirector{}
	settings := Settings{AutomaticExpressions: true}
	s := directedEmotion("sad", 1)
	d.sample(0, s, settings)
	d.sample(500, s, settings)
	settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "curious", Intensity: .5}
	d.sample(600, s, settings)
	s.Session = 2
	s.Speech.Plan.Performance.Intent.Emotion.State = "angry"
	v := d.sample(1100, s, settings)
	if v[4] != .5 || v[6] != 0 {
		t.Fatal("queued emotional target overrode user edit", v)
	}
	settings.AutomaticExpressions = false
	d.sample(1200, s, settings)
	if d.sample(2100, s, settings)[4] != .5 {
		t.Fatal("disabled variation lost selected baseline")
	}
	s.ReplyID = "reply-two"
	s.Session = 3
	settings.AutomaticExpressions = true
	d.sample(2200, s, settings)
	if d.sample(2700, s, settings)[6] != .8 {
		t.Fatal("next reply remained suppressed")
	}
}

func TestEmotionSmileUsesFullPreparedSpeakingRange(t *testing.T) {
	if emotionSmile(rendercontract.EmotionWeights{1: .8}) != .8 {
		t.Fatal("emotion smile was attenuated")
	}
}
