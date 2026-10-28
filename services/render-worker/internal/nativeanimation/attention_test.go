package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func focusHandoffFixture() (Config, attentionDirector, Head, SpeechState) {
	c := Config{Conversational: true, ConversationalMotion: true, Expressive: true, Seed: 17, Bounds: Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}}
	d := newAttention(c.Seed)
	base := Head{Yaw: 3, Pitch: 1, Roll: .3}
	d.posture = briefPosture{ready: true, from: base, target: base, value: base, start: 0, duration: 0, hold: 10000}
	amount := .6
	speech := SpeechState{Active: true, Session: 1, Position: .99, Speech: &Speech{Plan: &Plan{natural: true, Duration: 1, Performance: &Performance{
		Motion:   &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: "same-reply", Offset: 0},
		Intent:   &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "explain"},
		Resolved: []PerformanceCue{{Kind: "brows", Time: .8, Duration: .72, Amount: &amount}},
	}}}}
	return c, d, base, speech
}

func TestSemanticFocusReleaseSurvivesPhraseAndIdleHandoff(t *testing.T) {
	for _, active := range []bool{true, false} {
		name := "reply-completion"
		if active {
			name = "adjacent-phrase"
		}
		t.Run(name, func(t *testing.T) {
			c, d, base, first := focusHandoffFixture()
			before := d.sample(1000, first, c).Head
			next := SpeechState{Active: active, Session: 2, Speech: &Speech{Plan: &Plan{natural: true, Duration: 1, Performance: &Performance{
				Motion: &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: "same-reply", Offset: 1},
				Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "explain"},
			}}}}
			if !active {
				next.Session = first.Session
			}
			after := d.sample(1033.333, next, c).Head
			if delta := math.Abs(after.Yaw - before.Yaw); delta > .001 {
				t.Fatalf("phrase handoff restored competing posture abruptly: %.6f degrees (%+v -> %+v)", delta, before, after)
			}
			half := d.sample(1193.333, next, c).Head
			if half.Yaw <= before.Yaw || half.Yaw >= base.Yaw {
				t.Fatal("competing posture did not recover continuously", before, half, base)
			}
			settled := d.sample(1353.333, next, c).Head
			if math.Abs(settled.Yaw-base.Yaw) > .001 || math.Abs(settled.Pitch-base.Pitch) > .001 {
				t.Fatal("competing posture did not finish the existing 320ms release", settled, base)
			}
			t.Logf("yaw before %.6f, handoff %.6f, 160ms %.6f, 320ms %.6f; gesture timing unchanged", before.Yaw, after.Yaw, half.Yaw, settled.Yaw)
		})
	}
}

func TestSemanticFocusDoesNotFilterAnchoredGesture(t *testing.T) {
	c, d, _, speech := focusHandoffFixture()
	d.posture = briefPosture{ready: true, start: 0, hold: 10000}
	speech.Speech.Plan.Gestures = []Gesture{{Time: .8, Duration: .72, Pitch: -2.3, Kind: "emphasis"}}
	for i, position := range []float64{.81, .95, 1.01, 1.2, 1.51} {
		speech.Position = position
		got := d.sample(1000+float64(i)*50, speech, c).Head
		want := gestureAt(speech.plan(), position)
		if math.Abs(got.Pitch-want.Pitch) > 1e-9 || math.Abs(got.Yaw-want.Yaw) > 1e-9 || math.Abs(got.Roll-want.Roll) > 1e-9 {
			t.Fatal("semantic focus changed the admitted gesture trajectory", position, got, want)
		}
	}
}
