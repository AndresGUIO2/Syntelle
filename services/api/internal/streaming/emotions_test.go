package streaming

import (
	"syntelle/render-contract"
	"testing"
)

func TestEmotionDecisionRejectsStaleConfigurationAndRetiredWorkers(t *testing.T) {
	s := &streamSession{emotionPolicyRevision: 1, currentTurn: "turn-current", settings: rendercontract.Defaults()}
	d := rendercontract.EmotionDecision{Instance: "instance-one", Version: rendercontract.EmotionPolicyVersion, PolicyRevision: 1, Sequence: 1, PhraseIndex: -1, Reason: "configuration", Effective: rendercontract.EmotionTarget{State: "neutral", Energy: .5}, Baseline: rendercontract.EmotionBaseline{State: "neutral"}, Regulation: rendercontract.EmotionRegulation{}.Normalized()}
	if !admitEmotionDecision(s, d) || admitEmotionDecision(s, d) {
		t.Fatal("initialization or duplicate gate failed")
	}
	d.Sequence++
	d.PhraseIndex = 0
	d.TurnID = "turn-current"
	d.Reason = "accepted"
	if !admitEmotionDecision(s, d) {
		t.Fatal(d)
	}
	s.emotionPolicyRevision = 3
	d.Sequence++
	if admitEmotionDecision(s, d) {
		t.Fatal("A/B/A configuration resurrected old evidence")
	}
	d.PolicyRevision = 3
	d.Instance = "instance-two"
	d.Sequence = 1
	d.PhraseIndex = -1
	d.Reason = "configuration"
	if !admitEmotionDecision(s, d) {
		t.Fatal("new private worker could not initialize")
	}
	d.Instance = "instance-one"
	if admitEmotionDecision(s, d) {
		t.Fatal("retired worker accepted")
	}
	d.Instance = "instance-two"
	d.Sequence = 2
	d.TurnID = ""
	d.Reason = "clock_transition"
	s.currentTurn = ""
	if !admitEmotionDecision(s, d) {
		t.Fatal("recovery completion was lost after reply end")
	}
}

func TestEmotionActivationRejectsUnplayedStaleAndDuplicateTargets(t *testing.T) {
	s := &streamSession{currentTurn: "turn-current", settings: rendercontract.Defaults()}
	s.settings.EmotionRegulation = rendercontract.EmotionRegulation{Enabled: false, Temperament: "calm"}
	v := rendercontract.EmotionActivation{TurnID: "turn-current", Generation: 2, PhraseIndex: 1, Emotion: rendercontract.EmotionTarget{State: "sad", Intensity: .7, Energy: .4}, Baseline: rendercontract.EmotionBaseline{State: "neutral"}}
	if !admitEmotionActivation(s, v) {
		t.Fatal("current activation rejected")
	}
	if admitEmotionActivation(s, v) {
		t.Fatal("duplicate activation admitted")
	}
	old := v
	old.PhraseIndex = 0
	if admitEmotionActivation(s, old) {
		t.Fatal("old phrase admitted")
	}
	old = v
	old.Generation = 1
	old.PhraseIndex = 2
	if admitEmotionActivation(s, old) {
		t.Fatal("old generation admitted")
	}
	old = v
	old.TurnID = "turn-cancelled"
	if admitEmotionActivation(s, old) {
		t.Fatal("cancelled target admitted")
	}
	s.settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "joyful", Intensity: .5}
	v.PhraseIndex = 2
	if admitEmotionActivation(s, v) {
		t.Fatal("previous baseline overwrote user edit")
	}
}
