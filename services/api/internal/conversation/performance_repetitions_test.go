package conversation

import (
	"fmt"
	"testing"
)

func TestPhraseRepeatedNodsPreserveSpeechAndRejectBadOptionalCounts(t *testing.T) {
	for _, tc := range []struct {
		name, kind, count string
		want              int
	}{
		{"seven nods", "nod", "7", 1}, {"legacy missing count", "nod", "", 1},
		{"repeated shake", "shake", "4", 1}, {"invalid shake count", "shake", "9", 0},
		{"zero", "nod", "0", 0}, {"too many", "nod", "9", 0}, {"fraction", "nod", "1.5", 0},
		{"other gesture", "brows", "2", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := ""
			if tc.count != "" {
				count = `,"repetitions":` + tc.count
			}
			text := fmt.Sprintf(`{"version":"performance-intent/2","tone":"neutral","phrases":[{"text":"Yes indeed.","cues":[{"kind":%q,"anchor":"Yes","amount":0.8%s}]}]}`, tc.kind, count)
			p, err := completePhrases(text)
			if err != nil || len(p) != 1 || p[0].Text != "Yes indeed." || len(p[0].Cues) != tc.want {
				t.Fatal(p, err)
			}
		})
	}
}

func TestRepeatedNodsSpendSharedReplyAllowanceAndCannotOverlap(t *testing.T) {
	seven, two, one := 7, 2, 1
	b := newReplyGestures()
	makeCue := func(n *int, at, length float64) PerformanceValidation {
		return PerformanceValidation{Resolved: []ResolvedCue{{Kind: "nod", Time: at, Duration: length, Repetitions: n}}}
	}
	if len(b.admit(makeCue(&seven, 0, 4.34), 0).Resolved) != 1 {
		t.Fatal("sequence rejected")
	}
	if len(b.admit(makeCue(&one, 3, .62), 0).Resolved) != 0 {
		t.Fatal("overlap admitted")
	}
	if len(b.admit(makeCue(&two, 5, 1.24), 0).Resolved) != 0 {
		t.Fatal("allowance exceeded")
	}
	if len(b.admit(makeCue(&one, 5, .62), 0).Resolved) != 1 {
		t.Fatal("remaining allowance lost")
	}
	if len(b.admit(makeCue(&one, 8, .62), 0).Resolved) != 0 {
		t.Fatal("ninth nod admitted")
	}
}
