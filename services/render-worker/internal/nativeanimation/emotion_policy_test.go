package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func policyFixture(temperament string) (EmotionController, rendercontract.Settings) {
	s := rendercontract.Defaults()
	s.EmotionRegulation = rendercontract.EmotionRegulation{Enabled: true, Temperament: temperament}
	c := EmotionController{}
	c.Configure(0, s, "")
	return c, s
}
func propose(c *EmotionController, ms float64, state string, intensity float64) rendercontract.EmotionDecision {
	v := rendercontract.EmotionTarget{State: state, Intensity: intensity, Energy: 1}
	return c.Propose(ms, &v, "turn_fixture", int(ms/1000)%32, 1)
}
func TestEmotionPolicyCapsAndAdversarialAlternation(t *testing.T) {
	for _, profile := range []string{"calm", "warm", "expressive"} {
		t.Run(profile, func(t *testing.T) {
			for _, state := range rendercontract.EmotionStates[1:] {
				c, s := policyFixture(profile)
				d := propose(&c, 0, state, 1)
				if d.Reason != "accepted" || math.Abs(d.Effective.Intensity-rendercontract.EmotionCeiling(state)*s.EmotionRegulation.Profile().Multiplier) > 1e-9 || d.Effective.Energy != .65 {
					t.Fatal(state, d)
				}
			}
			c, _ := policyFixture(profile)
			accepted := 0
			for i := 0; i < 200; i++ {
				state := []string{"angry", "joyful", "fearful", "curious"}[i%4]
				d := propose(&c, float64(i*50), state, 1)
				if d.Reason == "accepted" {
					accepted++
				}
			}
			if accepted > 2 {
				t.Fatal("rapid requests bypassed state budget", accepted)
			}
		})
	}
}
func TestEmotionPolicyOppositionBudgetAndNoDelayedActivation(t *testing.T) {
	c, _ := policyFixture("calm")
	if propose(&c, 0, "joyful", 1).Reason != "accepted" {
		t.Fatal(c)
	}
	if propose(&c, 1000, "angry", 1).Reason != "minimum_duration" {
		t.Fatal("dwell bypassed")
	}
	if propose(&c, 6500, "curious", .01).Reason != "below_threshold" {
		t.Fatal("small target accepted")
	}
	if propose(&c, 7000, "angry", 1).Reason != "confirmation_required" {
		t.Fatal("contradictory proposal did not clear confirmation")
	}
	if d := propose(&c, 8000, "angry", 1); d.Reason != "accepted" || d.Phase != "bridge" {
		t.Fatal(d)
	}
	w, _ := c.Motion.Sample(8500)
	if w != (rendercontract.EmotionWeights{}) {
		t.Fatal("opposed transition did not pass through neutral", w)
	}
	if propose(&c, 15000, "surprised", 1).Reason != "change_budget" {
		t.Fatal("budget bypassed")
	}
	c.Advance(19000, true)
	if c.Motion.Effective.State != "angry" {
		t.Fatal("rejected proposal activated on timer")
	}
	if propose(&c, 21000, "surprised", 1).Reason != "accepted" {
		t.Fatal("expired budget did not recover")
	}
}
func TestEmotionPolicyRecoveryContinuityOverridesAndManualRange(t *testing.T) {
	c, s := policyFixture("warm")
	propose(&c, 0, "sad", 1)
	c.End(1000, false)
	if c.Advance(2999, false) {
		t.Fatal("reaction released early")
	}
	c.Advance(3000, false)
	if c.Motion.Phase != "recovery" {
		t.Fatal(c.Motion)
	}
	w, _ := c.Motion.Sample(6000)
	if w[5] <= 0 || w[5] >= .36 {
		t.Fatal(w)
	}
	c.Advance(9000, false)
	if c.Motion.Phase != "baseline" {
		t.Fatal(c.Motion)
	}
	s.EmotionBaseline = rendercontract.EmotionBaseline{State: "angry", Intensity: 1}
	c.Configure(9100, s, "turn_fixture")
	if propose(&c, 9200, "joyful", 1).Reason != "suppressed" {
		t.Fatal("manual override lost")
	}
	w, _ = c.Motion.Sample(10000)
	if w[6] != 1 {
		t.Fatal("manual intensity was capped", w)
	}
	s.ReducedMotion = true
	c.Configure(10010, s, "turn_fixture")
	w, _ = c.Motion.Sample(10010)
	if w[6] != 1 {
		t.Fatal(w)
	}
	s.ReducedMotion = false
	s.EmotionRegulation.Enabled = false
	c.Configure(10020, s, "turn_fixture")
	if !c.Controlled("turn_fixture") || c.Controlled("next_reply") {
		t.Fatal("mode edit suppression escaped current reply")
	}
}
func TestEmotionPolicyIntensityHysteresisAndSlew(t *testing.T) {
	c, _ := policyFixture("expressive")
	propose(&c, 0, "joyful", .3)
	if propose(&c, 600, "joyful", .35).Reason != "hysteresis" {
		t.Fatal("small fluctuation accepted")
	}
	if propose(&c, 700, "joyful", .65).Reason != "intensity_limited" {
		t.Fatal(c)
	}
	last, _ := c.Motion.Sample(700)
	for ms := 710.; ms <= 5000; ms += 10 {
		w, _ := c.Motion.Sample(ms)
		if math.Abs(w[1]-last[1]) > .0015001 {
			t.Fatal("intensity slew exceeded", ms, w, last)
		}
		last = w
	}
	c.End(6000, true)
	if c.confirmation != "" {
		t.Fatal("interruption retained confirmation")
	}
}

func TestEmotionPolicyRecoveryOppositionAndAutomaticOff(t *testing.T) {
	c, s := policyFixture("expressive")
	propose(&c, 0, "joyful", 1)
	c.End(3500, false)
	c.Advance(5500, false)
	if propose(&c, 6000, "angry", 1).Reason != "confirmation_required" {
		t.Fatal("recovery bypassed opposition confirmation")
	}
	s.AutomaticExpressions = false
	c.Configure(6100, s, "turn_fixture")
	if propose(&c, 6200, "angry", 1).Reason != "suppressed" {
		t.Fatal("disabled automatic variation accepted")
	}
	w, _ := c.Motion.Sample(7000)
	if w != (rendercontract.EmotionWeights{}) {
		t.Fatal("automatic off did not recover baseline", w)
	}
}

func TestControlledNegativeStateGatesOnlyAutomaticPositiveCues(t *testing.T) {
	amount := 1.
	config := Config{Emotions: true, Expressions: true, Directed: true, Expressive: true, SpeakingSmile: true, SmileAvailable: true, LaughAvailable: true, ReferenceLaugh: true}
	snapshot := EmotionSnapshot{To: rendercontract.EmotionWeights{6: .6}, EnergyTo: .5, Effective: rendercontract.EmotionTarget{State: "angry", Intensity: .6, Energy: .5}}
	render := func(cues []PerformanceCue, manual float64) Frame {
		e := NewEngine(config)
		s := Settings{AutomaticExpressions: true, Smile: manual, EmotionControl: &snapshot}
		e.Sample(0, SpeechState{}, s)
		p := &Plan{Performance: &Performance{Intent: &PerformanceIntent{Version: "performance-intent/3"}, Resolved: cues}}
		e.Sample(500, SpeechState{Active: true, ReplyID: "turn_fixture", Session: 1, Position: .5, Speech: &Speech{Duration: 2, Plan: p}}, s)
		return e.Sample(900, SpeechState{Active: true, ReplyID: "turn_fixture", Session: 1, Position: .9, Speech: &Speech{Duration: 2, Plan: p}}, s)
	}
	positive := []PerformanceCue{{Kind: "smile", Time: 0, Duration: 2, Amount: &amount}, {Kind: "laugh", Time: 0, Duration: 2, Amount: &amount}}
	with, without := render(positive, .6), render(nil, .6)
	if with != without || with.Laugh != 0 || with.Emotion[6] != .6 {
		t.Fatal("automatic positive cue bypassed held mood", with, without)
	}
	if with.Smile <= render(nil, 0).Smile {
		t.Fatal("manual smile adjustment was lost")
	}
}

func TestEmotionPolicyTemperamentScalesSubCeilingProposals(t *testing.T) {
	for _, name := range []string{"calm", "warm", "expressive"} {
		t.Run(name, func(t *testing.T) {
			c, s := policyFixture(name)
			d := propose(&c, 0, "joyful", .2)
			if math.Abs(d.Effective.Intensity-.2*s.EmotionRegulation.Profile().Multiplier) > 1e-9 {
				t.Fatal(d)
			}
		})
	}
}
