package nativeanimation

import (
	"math"
	"testing"
)

func TestDirectedTargetsCanSoftenAndRestoreManualBaseline(t *testing.T) {
	zero := 0.
	plan := &Plan{Duration: 5, Performance: &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "smile", Time: .5, Duration: 2, Amount: &zero}, {Kind: "brows", Time: .5, Duration: 2, Amount: &zero}}}}
	state := SpeechState{Active: true, Session: 1, Speech: &Speech{Plan: plan}}
	director := newExpressions()
	config := Config{SmileAvailable: true, SpeakingSmile: true, Directed: true}
	var atCue, after expression
	for ms := 0.; ms <= 4500; ms += 20 {
		state.Position = ms / 1000
		value := director.sample(ms, expression{1, .8}, true, false, state, false, true, config)
		if ms == 1400 {
			atCue = value
		}
		after = value
	}
	if atCue.Smile > .15 || atCue.Brows > .2 {
		t.Fatal("automatic zero target could not soften manual baseline", atCue)
	}
	if after.Smile < .49 || after.Brows < .79 {
		t.Fatal("manual baseline did not return", after)
	}
}

func TestDoubleNodAndTiltAreDistinctBoundedGestures(t *testing.T) {
	plan := &Plan{Gestures: []Gesture{{Time: 0, Duration: 1.5, Pitch: -1, Kind: "double_nod"}, {Time: 2, Duration: 1.4, Roll: .3, Kind: "tilt"}}}
	first, second := gestureAt(plan, .315), gestureAt(plan, 1.065)
	if first.Pitch > -.9 || second.Pitch > -.9 || gestureAt(plan, .75).Pitch != 0 {
		t.Fatal(first, second)
	}
	if gestureAt(plan, 2.5).Roll <= 0 {
		t.Fatal("tilt has no roll")
	}
}

func TestDirectedRepeatedNodBindsTheSamePrimitiveForEachCycle(t *testing.T) {
	count := 7
	amount := .8
	prepared := &PreparedSpeech{duration: 6, directed: true}
	performance := &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "nod", Time: .2, Duration: 4.34, Amount: &amount, Repetitions: &count}}}
	speech := BindSpeech(prepared, performance)
	if len(speech.Plan.Gestures) != count {
		t.Fatal("sequence was not expanded", speech.Plan)
	}
	for i, g := range speech.Plan.Gestures {
		if g.Kind != "nod" || math.Abs(g.Duration-.62) > 1e-9 {
			t.Fatal("not the same nod primitive", g)
		}
		if got := gestureAt(speech.Plan, g.Time+.62*.42); math.Abs(got.Pitch+3.8*amount) > 1e-8 {
			t.Fatal("incomplete cycle", i, got)
		}
	}
	if got := gestureAt(speech.Plan, 4.6); got != (Head{}) {
		t.Fatal("sequence did not release", got)
	}
}

func TestRepeatedNodAdmissionSpendsEachCycleAndReservesItsWindow(t *testing.T) {
	seven, two, one := 7, 2, 1
	amount := 1.
	value := &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2", Text: "Yes, agreed, indeed.", Cues: []PerformanceAnchor{
		{Kind: "nod", Anchor: "Yes", Amount: &amount, Repetitions: &seven},
		{Kind: "nod", Anchor: "agreed", Amount: &amount, Repetitions: &two},
		{Kind: "nod", Anchor: "indeed", Amount: &amount, Repetitions: &one},
	}}, Resolved: []PerformanceCue{
		{Kind: "nod", Time: 0, Duration: 4.34, Amount: &amount, Repetitions: &seven},
		{Kind: "nod", Time: 3, Duration: .62, Amount: &amount, Repetitions: &one},
		{Kind: "nod", Time: 5, Duration: 1.24, Amount: &amount, Repetitions: &two},
		{Kind: "nod", Time: 5, Duration: .62, Amount: &amount, Repetitions: &one},
		{Kind: "nod", Time: 8, Duration: .62, Amount: &amount, Repetitions: &one},
	}}
	got := NewEngine(Config{}).AdmitPerformance(value, value.Intent.Text, 0)
	if len(got.Resolved) != 2 || *got.Resolved[0].Repetitions != 7 || *got.Resolved[1].Repetitions != 1 {
		t.Fatal("bad repeated-nod allowance", got.Resolved)
	}
}

func TestPreparedLaughterRespectsPauseAndAutomaticExpressionControl(t *testing.T) {
	_, config, _ := loadReference(t)
	config.Expressions = true
	config.SmileAvailable = true
	config.SpeakingSmile = true
	config.LaughAvailable = true
	amount := .8
	plan := &Plan{Duration: 3, Performance: &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Time: 1, Duration: 1, Amount: &amount}}}}
	speech := SpeechState{Active: true, Session: 1, Position: 1.5, Speech: &Speech{Plan: plan, Duration: 3}}
	e := NewEngine(config)
	settings := DefaultSettings()
	settings.Smile = 1
	frame := e.Sample(1500, speech, settings)
	if frame.Laugh < .7 || frame.Smile < .7 || frame.Mouth.From != "Rest" || frame.Mouth.To != "A" || frame.Mouth.Amount > .06 {
		t.Fatal("laughter is not coordinated", frame)
	}
	settings.Paused = true
	if frozen := e.Sample(1800, speech, settings); frozen != frame {
		t.Fatal("pause advanced expression")
	}
	settings.Paused = false
	settings.AutomaticExpressions = false
	if stopped := e.Sample(1850, speech, settings); stopped.Laugh != 0 {
		t.Fatal("automatic expressions ignored")
	}
	settings.AutomaticExpressions = true
	settings.ReducedMotion = true
	if reduced := e.Sample(1900, speech, settings); reduced.Laugh != 0 {
		t.Fatal("reduced motion ignored")
	}
}

func TestDirectedShakeMatchesPreviewAndHistoricalShapeRemainsReadable(t *testing.T) {
	count := 4
	amount := 1.
	prepared := &PreparedSpeech{duration: 6, directed: true, bounds: Bounds{3, 4, 1.2}}
	performance := &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "shake", Time: .2, Duration: 3.68, Amount: &amount, Repetitions: &count}, {Kind: "tilt", Time: 4, Duration: 1.15, Amount: &amount, Repetitions: new(int(1))}}}
	speech := BindSpeech(prepared, performance)
	if len(speech.Plan.Gestures) != 5 {
		t.Fatal(speech.Plan)
	}
	for i := 0; i < count; i++ {
		g := speech.Plan.Gestures[i]
		if g.Kind != "shake_cycle" || math.Abs(g.Duration-.92) > 1e-8 {
			t.Fatal(g)
		}
		if got := gestureAt(speech.Plan, g.Time+g.Duration*.25); math.Abs(got.Yaw-2.85) > 1e-8 {
			t.Fatal("weak directed shake", got)
		}
	}
	if math.Abs(gestureAt(speech.Plan, 4+1.15*.42).Roll-1.14) > 1e-8 {
		t.Fatal("weak directed tilt")
	}
	performance.Resolved = []PerformanceCue{{Kind: "shake", Time: .2, Duration: 1.25, Amount: &amount}}
	legacy := BindSpeech(prepared, performance)
	if legacy.Plan.Gestures[0].Kind != "shake" || legacy.Plan.Gestures[0].Yaw != 1.5 {
		t.Fatal("historical shake interpretation changed")
	}
}
