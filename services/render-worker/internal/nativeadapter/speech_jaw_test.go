package nativeadapter

import (
	"math"
	"reflect"
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"testing"
)

func TestSpeechJawPreservesContactsAndQuietStates(t *testing.T) {
	rig := &rendercontract.SpeechJawRig{PoseOpening: map[string]float64{"Rest": 0, "A": .75, "O": 1, "MBP": 0, "FV": .14}}
	timeline := []nativeanimation.TimelineEvent{{Time: 0, Pose: "A"}, {Time: .5, Pose: "MBP"}, {Time: .57, Pose: "A"}, {Time: 1, Pose: "Rest"}}
	original := append([]nativeanimation.TimelineEvent(nil), timeline...)
	speech := nativeanimation.SpeechState{Active: true, Position: .53, Speech: &nativeanimation.Speech{Duration: 1, Timeline: timeline}}
	if got := speechJawAt(speech, rig, false, false, 0); math.Abs(got-.75) > 1e-8 {
		t.Fatal("brief lip contact forced skin jaw pumping", got)
	}
	if !reflect.DeepEqual(timeline, original) {
		t.Fatal("skin jaw changed the validated lip/contact timeline")
	}
	withLaugh := speech
	copySpeech := *speech.Speech
	copySpeech.Plan = &nativeanimation.Plan{Performance: &nativeanimation.Performance{Resolved: []nativeanimation.PerformanceCue{{Kind: "laugh", Time: .5, Duration: .2}}}}
	withLaugh.Speech = &copySpeech
	if got := speechJawAt(withLaugh, rig, false, false, 0); got != 0 {
		t.Fatal("speech jaw leaked into the laughter span entry/gap", got)
	}
	for _, row := range []struct {
		name             string
		preview, reduced bool
		laugh            float64
		active           bool
	}{
		{"preview", true, false, 0, true}, {"laugh", false, false, .01, true}, {"rest", false, false, 0, false}, {"reduced", false, true, 0, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			s := speech
			s.Active = row.active
			if got := speechJawAt(s, rig, row.preview, row.reduced, row.laugh); got != 0 {
				t.Fatal("speech jaw leaked into protected presentation", got)
			}
		})
	}
}

func TestSpeechJawSustainedVowelIsStable(t *testing.T) {
	rig := &rendercontract.SpeechJawRig{PoseOpening: map[string]float64{"Rest": 0, "A": .75}}
	s := nativeanimation.SpeechState{Active: true, Speech: &nativeanimation.Speech{Duration: 2, Timeline: []nativeanimation.TimelineEvent{{Time: 0, Pose: "A"}, {Time: 2, Pose: "Rest"}}}}
	for _, time := range []float64{.2, .7, 1.5} {
		s.Position = time
		if got := speechJawAt(s, rig, false, false, 0); math.Abs(got-.75) > 1e-8 {
			t.Fatal("sustained viseme acquired arbitrary jaw movement", got)
		}
	}
}
