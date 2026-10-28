package conversation

import (
	"encoding/binary"
	"syntelle/render-contract"
	"testing"
)

func TestConversationalReplyWindowReplenishesWithoutChangingLegacy(t *testing.T) {
	for _, continuous := range []bool{false, true} {
		name := "legacy"
		if continuous {
			name = "continuous"
		}
		t.Run(name, func(t *testing.T) {
			b := newReplyGestures()
			accepted := 0
			for i := 0; i < 12; i++ {
				v := PerformanceValidation{Intent: PerformanceIntent{Version: "performance-intent/3"}, Resolved: []ResolvedCue{{Kind: "emphasis", Time: .1, Duration: .72}}}
				if continuous {
					v.Motion = &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: "r", Offset: float64(i) * 3}
				}
				accepted += len(b.admit(v, float64(i)*3).Resolved)
			}
			want := 8
			if continuous {
				want = 12
			}
			if accepted != want {
				t.Fatal(accepted, want)
			}
		})
	}
}

func TestConversationalResolutionRetainsShortPhraseTail(t *testing.T) {
	amount := .7
	repetitions := 1
	intent := PerformanceIntent{Version: "performance-intent/3", Text: "That is correct.", Tone: "neutral", Cues: []PerformanceCue{{Kind: "nod", Anchor: "correct", Amount: &amount, Repetitions: &repetitions}}}
	wav := performanceFixtureWAV()[:44+12000]
	binary.LittleEndian.PutUint32(wav[40:], 12000)
	legacy := ValidatePerformance(intent, intent.Text)
	continuous := legacy
	continuous.Motion = &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: "reply", Offset: 0}
	if got := ResolvePerformance(legacy, wav, .25); len(got.Resolved) != 0 {
		t.Fatal("legacy phrase limit changed", got)
	}
	if got := ResolvePerformance(continuous, wav, .25); len(got.Resolved) != 1 || got.Resolved[0].Duration <= .25 {
		t.Fatal("valid short-phrase gesture was discarded", got)
	}
}
