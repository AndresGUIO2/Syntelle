package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func TestNaturalSequencesAreDeterministicContinuousAndInsidePreparedBounds(t *testing.T) {
	for _, kind := range []string{"nod", "shake", "tilt", "emphasis"} {
		t.Run(kind, func(t *testing.T) {
			count := 1
			duration := 1.4
			if kind == "nod" {
				count = 3
				duration = 3 * rendercontract.NaturalNodCycleSeconds
			}
			if kind == "shake" {
				count = 3
				duration = 3 * rendercontract.NaturalShakeCycleSeconds
			}
			amount := .8
			cue := PerformanceCue{Kind: kind, Time: .1, Duration: duration, Amount: &amount, Repetitions: &count}
			prepared := &PreparedSpeech{directed: true, duration: 8, bounds: Bounds{3, 4, 1.2}}
			performance := &Performance{Intent: &PerformanceIntent{Version: "performance-intent/3", Text: "A meaningful spoken point."}, Resolved: []PerformanceCue{cue}}
			a, b := BindSpeech(prepared, performance), BindSpeech(prepared, performance)
			if len(a.Plan.Gestures) != count || len(prepared.plan.Gestures) != 0 {
				t.Fatal("binding changed preparation or repetition count")
			}
			if math.Abs(a.Plan.Gestures[count-1].Time+a.Plan.Gestures[count-1].Duration-(.1+duration)) > 1e-9 {
				t.Fatal("sequence escaped its admitted window")
			}
			peak := 0.
			previous := Head{}
			for at := 0.; at < duration+.2; at += .001 {
				x := gestureAt(a.Plan, at)
				if x != gestureAt(b.Plan, at) {
					t.Fatal("non-deterministic performance")
				}
				if math.Abs(x.Yaw) > 3 || math.Abs(x.Pitch) > 4 || math.Abs(x.Roll) > 1.2 {
					t.Fatal("prepared geometry exceeded", x)
				}
				if at > 0 && math.Max(math.Abs(x.Yaw-previous.Yaw), math.Max(math.Abs(x.Pitch-previous.Pitch), math.Abs(x.Roll-previous.Roll))) > .065 {
					t.Fatal("trajectory discontinuity", at, x, previous)
				}
				peak = math.Max(peak, math.Max(math.Abs(x.Pitch), math.Max(math.Abs(x.Yaw), math.Abs(x.Roll))))
				previous = x
			}
			if peak < .2 || gestureAt(a.Plan, duration+.2) != (Head{}) {
				t.Fatal("invisible gesture or incomplete recovery", peak)
			}
		})
	}
}

func TestNaturalFirstPhraseGestureIsNotSwallowedByAnotherOnsetFade(t *testing.T) {
	amount := 1.
	one := 1
	p := &PreparedSpeech{directed: true, duration: 2, bounds: Bounds{3, 4, 1.2}}
	speech := BindSpeech(p, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/3", Text: "Yes, that is right."}, Resolved: []PerformanceCue{{Kind: "nod", Time: 0, Duration: .82, Amount: &amount, Repetitions: &one}}})
	_, config, _ := loadReference(t)
	config.Directed, config.Expressions, config.Emotions, config.Conversational = true, false, false, false
	config.Bounds = p.bounds
	e := NewEngine(config)
	s := SpeechState{Active: true, Session: 1, Speech: speech}
	e.Sample(0, s, DefaultSettings())
	g := speech.Plan.Gestures[0]
	s.Position = g.Duration * .34
	frame := e.Sample(s.Position*1000, s, DefaultSettings())
	if frame.Head.Pitch > -3.3 {
		t.Fatal("first accent lost its prepared strength", frame.Head)
	}
	paused := DefaultSettings()
	paused.Paused = true
	if e.Sample(800, s, paused) != frame {
		t.Fatal("pause advanced the gesture")
	}
	settings := DefaultSettings()
	settings.Movement = false
	if e.Sample(820, s, settings).Head != (Head{}) {
		t.Fatal("movement off ignored")
	}
	settings.Movement = true
	settings.ReducedMotion = true
	settings.Brows = .6
	if f := e.Sample(840, s, settings); f.Head != (Head{}) || f.Brows != 0 { // no expression companion in this fixture
		t.Fatal("reduced motion admitted automatic movement", f)
	}
}

func TestNaturalBrowAccentPeaksOnAudioAndReleasesAfterInterruption(t *testing.T) {
	amount := .75
	plan := &Plan{Performance: &Performance{Intent: &PerformanceIntent{Version: "performance-intent/3"}, Resolved: []PerformanceCue{{Kind: "brows", Time: .3, Duration: .95, Amount: &amount}}}}
	s := SpeechState{Active: true, Session: 1, Speech: &Speech{Plan: plan}}
	d := newExpressions()
	c := Config{Directed: true, Expressions: true}
	firstPeak := -1.
	var last expression
	for ms := 0.; ms <= 700; ms += 10 {
		s.Position = ms / 1000
		last = d.sample(ms, expression{Brows: .2}, true, false, s, false, true, c)
		if last.Brows >= .749 && firstPeak < 0 {
			firstPeak = ms
		}
	}
	if firstPeak < 540 || firstPeak > 590 {
		t.Fatal("brow accent drifted behind the spoken target", firstPeak)
	}
	s.Active = false
	if stopped := d.sample(700, expression{Brows: .2}, true, false, s, false, false, c); math.Abs(stopped.Brows-last.Brows) > 1e-9 {
		t.Fatal("interruption snapped brows")
	}
	if released := d.sample(940, expression{Brows: .2}, true, false, s, false, false, c); math.Abs(released.Brows-.2) > 1e-9 {
		t.Fatal("manual brow baseline not restored", released)
	}
	if reduced := d.sample(950, expression{Brows: .6}, true, true, s, false, false, c); reduced.Brows != .6 {
		t.Fatal("reduced motion did not show manual baseline", reduced)
	}
}
