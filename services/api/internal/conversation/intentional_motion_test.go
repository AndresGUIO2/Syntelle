package conversation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func TestIntentionalMotionNeverFillsEmptyDirectionFromAudio(t *testing.T) {
	s := Service{Performance: &PerformanceProfile{MotionDirection: SemanticMotionDirection}}
	v := ValidatePerformance(PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Text: "A quiet explanation.", Tone: "neutral", Purpose: "explain", Cues: []PerformanceCue{}}, "A quiet explanation.")
	a := s.newPerformanceAcousticSchedule("fixture", true)
	a.next = 2
	got := resolvePerformance(v, accentWAV(), 4, a, 0)
	if len(got.Resolved) != 0 {
		t.Fatal("audio invented semantic motion", got.Resolved)
	}
}
func TestIntentionalPartialCuesRequireActualWordTiming(t *testing.T) {
	s := Service{Performance: &PerformanceProfile{MotionDirection: SemanticMotionDirection}}
	amount := .8
	intent := PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Text: "aa mark aa", Tone: "neutral", Purpose: "explain", Cues: []PerformanceCue{{Kind: "emphasis", Anchor: "mark", Amount: &amount}}}
	v := ValidatePerformance(intent, intent.Text)
	missing := ResolvePartialPerformance(v, accentWAV(), 4, s.newPerformanceAcousticSchedule("fixture", true), 0)
	if len(missing.Resolved) != 0 {
		t.Fatal("partial waveform guessed a word anchor", missing)
	}
	words := []rendercontract.WordBoundary{{Text: "mark", TextOffset: 3, Length: 4, OffsetMS: 1900, DurationMS: 200}}
	got := ResolvePartialPerformance(v, accentWAV(), 4, s.newPerformanceAcousticSchedule("fixture", true), 0, words...)
	AssignMotionCueIDs(&got, 3)
	if len(got.Resolved) != 1 {
		t.Fatal(got)
	}
	c := got.Resolved[0]
	if c.ID != "p3-cue-0" || c.Confidence < .9 || c.Timing != "native-word-boundaries/1" || math.Abs(c.Peak-2) > .15 || c.Time >= c.Peak || c.Duration > .75 {
		t.Fatal("invalid anchored motion", c)
	}
}
