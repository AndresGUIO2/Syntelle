package nativeanimation

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"syntelle/render-contract"
	"testing"
)

func TestPreparedPresenceControlsAndInspectionStates(t *testing.T) {
	path := os.Getenv("SYNTELLE_EMOTION_MANIFEST")
	if path == "" {
		t.Skip("explicit prepared-resource inspection")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Packages struct{ Motion, Oral, Expressions, Gaze json.RawMessage }
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	config, err := LoadBundle(manifest.Packages.Motion, manifest.Packages.Gaze, manifest.Packages.Expressions, manifest.Packages.Oral)
	if err != nil {
		t.Fatal(err)
	}
	states := []map[string]any{}
	for index, state := range rendercontract.EmotionStates {
		for _, amount := range []float64{0, .2, .4, .5, .75, 1} {
			engine := NewEngine(config)
			settings := DefaultSettings()
			settings.Movement = false
			settings.Blinking = false
			settings.AutomaticExpressions = false
			settings.EmotionBaseline = rendercontract.EmotionBaseline{State: state, Intensity: amount}
			engine.Sample(0, SpeechState{}, settings)
			frame := engine.Sample(2000, SpeechState{}, settings)
			if index > 0 && math.Abs(frame.Emotion[index]-amount) > 1e-9 {
				t.Fatal("manual slider was floored or attenuated", state, amount, frame.Emotion)
			}
			if !rendercontract.UnitAmount(frame.Smile) || !rendercontract.UnitAmount(frame.Mouth.Amount) {
				t.Fatal("composed control exceeds range", frame)
			}
			states = append(states, map[string]any{"Name": fmt.Sprintf("composed-%s-%g", state, amount), "State": map[string]any{"From": frame.Mouth.From, "To": frame.Mouth.To, "Amount": frame.Mouth.Amount, "Smile": frame.Smile, "Brows": frame.Brows, "Closure": frame.EyeClosure, "emotion": frame.Emotion, "RestingSmile": frame.Emotion == (rendercontract.EmotionWeights{}), "Background": "dark-lounge"}})
		}
	}
	if output := os.Getenv("SYNTELLE_EMOTION_STATES_OUTPUT"); output != "" {
		encoded, _ := json.Marshal(states)
		if err = os.WriteFile(output, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func presenceFixture(profile string) (EmotionController, rendercontract.Settings) {
	c, s := policyFixture(profile)
	c.Persistent = true
	return c, s
}

func TestPresenceAutomaticFloorAndQualifiedRange(t *testing.T) {
	for _, profile := range []string{"calm", "warm", "expressive"} {
		t.Run(profile, func(t *testing.T) {
			for _, state := range rendercontract.EmotionStates[1:] {
				for _, amount := range []float64{.01, .2, .4, .6, 1} {
					c, s := presenceFixture(profile)
					d := propose(&c, 0, state, amount)
					want := math.Max(.4, amount*s.EmotionRegulation.Profile().Multiplier)
					if d.Reason != "accepted" || math.Abs(d.Effective.Intensity-want) > 1e-9 {
						t.Fatalf("%s %.2f: got %+v want%.2f", state, amount, d, want)
					}
				}
			}
		})
	}
}

func TestPresenceZeroAndManualRemainBelowFloor(t *testing.T) {
	c, s := presenceFixture("expressive")
	if d := propose(&c, 0, "fearful", 0); d.Effective.State != "neutral" {
		t.Fatal("zero invented mood", d)
	}
	s.EmotionBaseline = rendercontract.EmotionBaseline{State: "sad", Intensity: .2}
	c.Configure(100, s, "")
	if c.Motion.Effective.Intensity != .2 {
		t.Fatal("manual baseline floored", c.Motion)
	}
}

func TestPresenceReplyPersistenceAndInactivityRecovery(t *testing.T) {
	c, _ := presenceFixture("expressive")
	propose(&c, 0, "fearful", .6)
	c.Activity(1000)
	c.End(1000, false)
	c.Advance(3000, false)
	if c.Motion.Effective.State != "fearful" || c.Motion.Phase == "recovery" {
		t.Fatal("reply end erased mood", c.Motion)
	}
	c.Advance(180999, false)
	if c.Motion.Phase == "recovery" {
		t.Fatal("early recovery")
	}
	c.Advance(181000, false)
	if c.Motion.Phase != "recovery" {
		t.Fatal("missing inactivity recovery", c.Motion)
	}
	w, _ := c.Motion.Sample(211000)
	if math.Abs(w[7]-.3) > 1e-9 {
		t.Fatal("recovery not sixty seconds", w)
	}
	c.Advance(241000, false)
	if c.Motion.Effective.State != "neutral" || c.Motion.Phase != "baseline" {
		t.Fatal(c.Motion)
	}
}

func TestPresenceActivityRenewsAndInterruptsRecoveryContinuously(t *testing.T) {
	c, _ := presenceFixture("expressive")
	propose(&c, 0, "fearful", .6)
	c.Activity(1000)
	c.End(1000, true)
	c.Activity(100000)
	c.Advance(181000, false)
	if c.Motion.Phase == "recovery" {
		t.Fatal("meaningful activity ignored")
	}
	c.Advance(280000, false)
	before, _ := c.Motion.Sample(300000)
	c.Activity(300000)
	after, _ := c.Motion.Sample(300000)
	if before != after {
		t.Fatal("activity snapped visible mood", before, after)
	}
	c.Advance(301000, false)
	if c.Motion.Effective.State != "fearful" {
		t.Fatal("activity lost committed mood")
	}
}

func TestPresenceReservationsDoNotCommitUnplayedMood(t *testing.T) {
	c, s := presenceFixture("expressive")
	request := rendercontract.DeliveryRequest{TurnID: "turn_fixture", Emotion: &rendercontract.EmotionTarget{State: "fearful", Intensity: .8, Energy: .6}}
	reserved, d := c.ReserveEmotion(0, s, request, 1)
	if d.Effective.Intensity != .8 || c.Motion.Effective.State != "neutral" {
		t.Fatal("reservation changed played state", d, c.Motion)
	}
	c.ActivateReserved(100, reserved)
	c.End(1000, false)
	c.Advance(60000, false)
	if c.Motion.Effective.State != "fearful" {
		t.Fatal("activated mood did not persist")
	}
}

func TestPresenceReplacementRestoresVisibleMoodAndAge(t *testing.T) {
	c, s := presenceFixture("expressive")
	propose(&c, 0, "fearful", .6)
	c.Activity(1000)
	c.End(1000, false)
	data := c.MoodMemory(61000)
	replacement, _ := presenceFixture("expressive")
	if err := replacement.RestoreMood(data); err != nil {
		t.Fatal(err)
	}
	before, _ := c.Motion.Sample(61000)
	after, _ := replacement.Motion.Sample(0)
	if before != after {
		t.Fatal("replacement changed visible mood", before, after)
	}
	replacement.Advance(119999, false)
	if replacement.Motion.Phase == "recovery" {
		t.Fatal("lost inactivity age")
	}
	replacement.Advance(120000, false)
	if replacement.Motion.Phase != "recovery" {
		t.Fatal("age was reset")
	}
	s.EmotionBaseline = rendercontract.EmotionBaseline{State: "sad", Intensity: .2}
	replacement.Configure(0, s, "")
	if replacement.RestoreMood(data) == nil {
		t.Fatal("stale manual state restored")
	}
}

func TestPresenceIdleInterruptionDoesNotRenewInactivity(t *testing.T) {
	c, _ := presenceFixture("expressive")
	propose(&c, 0, "fearful", .6)
	c.Activity(1000)
	c.End(1000, false)
	c.End(120000, true)
	c.Advance(181000, false)
	if c.Motion.Phase != "recovery" {
		t.Fatal("idle lifecycle traffic renewed mood", c.Motion)
	}
}
