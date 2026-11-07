package nativeanimation

import (
	"fmt"
	"math"
	"syntelle/render-contract"
	"testing"
)

func intentionalFixture() (Config, SpeechState, Settings) {
	c, s, settings := conversationalPresenceFixture()
	c.PresenceVersion = IntentionalPresenceVersion
	return c, s, settings
}
func TestIntentionalMotionDoesNotCreateLargeActionsWithoutMeaning(t *testing.T) {
	c, s, settings := intentionalFixture()
	s.Speech.Plan.Cues = []Cue{{Time: 1, Kind: "emphasis", Strength: 1}, {Time: 3, Kind: "pause", Strength: 1}}
	p := presenceDirector{}
	var first Head
	for i := 0; i < 600; i++ {
		s.Position = float64(i) / 30
		h := p.sample(s.Position*1000, s, c, settings)
		if i == 0 {
			first = h
		}
		if math.Abs(h.Yaw-first.Yaw) > .6001 || math.Abs(h.Pitch-first.Pitch) > .2501 || math.Abs(h.Roll-first.Roll) > .0801 {
			t.Fatal("unanchored conspicuous motion", h)
		}
	}
}
func TestIntentionalIdleRetainsPhase121Trajectory(t *testing.T) {
	c, s, settings := intentionalFixture()
	s.Active = false
	a, b := presenceDirector{}, presenceDirector{}
	old := c
	old.PresenceVersion = ExpressivePresenceVersion
	for i := 0; i < 900; i++ {
		at := float64(i) * 1000 / 30
		if a.sample(at, s, c, settings) != b.sample(at, s, old, settings) {
			t.Fatal("idle changed", i)
		}
	}
}
func motionCue(id string, at float64) PerformanceCue {
	a := .8
	return PerformanceCue{ID: id, Kind: "emphasis", Time: at, Duration: .52, Peak: at + .2, AnchorStart: at + .12, AnchorEnd: at + .28, Confidence: .95, Timing: "native-word-boundaries/1", Source: "semantic", Amount: &a}
}
func motionPerformance(cues ...PerformanceCue) *Performance {
	return &Performance{MotionDirection: SemanticMotionDirection, Motion: &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: "reply"}, Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Text: "An important point.", Purpose: "explain", Cues: []PerformanceAnchor{{Kind: "emphasis", Anchor: "important"}}}, Resolved: cues}
}
func TestIntentionalUpdatesAdmitEachFutureCueOnlyOnce(t *testing.T) {
	c, _, _ := intentionalFixture()
	e := NewEngine(c)
	first := motionPerformance()
	if e.AdmitPerformance(first, first.Intent.Text, 0) == nil {
		t.Fatal("empty prefix lost direction")
	}
	update := motionPerformance(motionCue("p0-cue-0", 1))
	a := e.AdmitPerformanceUpdate(update, update.Intent.Text, 100, .1)
	if len(a.Resolved) != 1 {
		t.Fatal(a)
	}
	for i := 0; i < 3; i++ {
		a = e.AdmitPerformanceUpdate(update, update.Intent.Text, 200, .2)
		if len(a.Resolved) != 1 {
			t.Fatal("duplicate spent allowance", a)
		}
	}
	late := motionPerformance(motionCue("p0-cue-0", 1), motionCue("p0-cue-1", .1))
	a = e.AdmitPerformanceUpdate(late, late.Intent.Text, 500, .5)
	if len(a.Resolved) != 1 {
		t.Fatal("late cue replayed", a)
	}
}

func TestIntentionalStrongPeakAndStreamMergeStayAnchored(t *testing.T) {
	for _, seed := range []uint32{1, 91273} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			c, s, settings := intentionalFixture()
			c.Seed = seed
			c.Bounds = Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}
			amount := .8
			cue := motionCue("p0-cue-0", 1)
			plan := &Plan{natural: true, Performance: motionPerformance(cue), Gestures: naturalGestures(cue, c.Bounds, 33, 0)}
			s.Speech.Plan = plan
			p := presenceDirector{}
			peak := Head{}
			for i := 0; i < 75; i++ {
				s.Position = float64(i) / 30
				if i > 40 {
					copyPlan := *plan
					copyPlan.Gestures = append([]Gesture(nil), plan.Gestures...)
					s.Speech.Plan = &copyPlan
				}
				h := p.sample(s.Position*1000, s, c, settings)
				if i == 36 {
					peak = h
				}
			}
			if math.Abs(peak.Yaw) < 6*amount-.01 {
				t.Fatal("semantic emphasis became tiny", peak)
			}
			if len(p.gestureSeen) != 1 {
				t.Fatal("stream replacement queued a duplicate", p.gestureSeen)
			}
		})
	}
}

func TestIntentionalProfileAllowsRepeatedStandaloneLaughter(t *testing.T) {
	c, _, _ := intentionalFixture()
	c.LaughAvailable = true
	e := NewEngine(c)
	amount := 1.
	p := &Performance{Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Text: "laugh", Tone: "demonstration", Cues: []PerformanceAnchor{{Kind: "laugh", Anchor: "laugh"}}}, Resolved: []PerformanceCue{{Kind: "laugh", Time: 0, Duration: 2.1, Amount: &amount}}}
	for i := 0; i < 3; i++ {
		got := e.AdmitPerformance(p, "laugh", float64(i)*3000)
		if got == nil || len(got.Resolved) != 1 {
			t.Fatal("standalone preview was deduplicated", i, got)
		}
	}
}

func TestMovingOnlyTheAnchorMovesTheDiagonalPeak(t *testing.T) {
	for _, start := range []float64{.4, 2.2} {
		c, s, settings := intentionalFixture()
		c.Bounds = Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}
		cue := motionCue("p0-cue-0", start)
		s.Speech.Plan = &Plan{natural: true, Performance: motionPerformance(cue), Gestures: naturalGestures(cue, c.Bounds, 33, 0)}
		p := presenceDirector{}
		maxYaw, peakAt := 0., 0.
		for i := range 100 {
			s.Position = float64(i) / 30
			head := p.sample(s.Position*1000, s, c, settings)
			if yaw := math.Abs(head.Yaw); yaw > maxYaw {
				maxYaw, peakAt = yaw, s.Position
			}
		}
		if maxYaw < 4 || math.Abs(peakAt-cue.Peak) > 1./30 {
			t.Fatalf("cue peak at %.3f rendered at %.3f with %.2f degrees", cue.Peak, peakAt, maxYaw)
		}
	}
}
