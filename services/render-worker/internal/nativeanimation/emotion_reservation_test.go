package nativeanimation

import (
	"syntelle/render-contract"
	"testing"
)

func TestReservedEmotionStartsFromCurrentVisibleRecovery(t *testing.T) {
	c, settings := policyFixture("warm")
	propose(&c, 0, "affectionate", .7)
	c.End(1000, false)
	c.Advance(3000, false)
	request := rendercontract.DeliveryRequest{TurnID: "new_turn", PhraseIndex: 0, Emotion: &rendercontract.EmotionTarget{State: "curious", Intensity: .5, Energy: .6}}
	before := c
	reservation, decision := c.ReserveEmotion(6500, settings, request, 1)
	if c != before {
		t.Fatal("preflight mutated played state")
	}
	c.Advance(9000, false)
	visible, _ := c.Motion.Sample(9000)
	c.ActivateReserved(9000, reservation)
	actual, _ := c.Motion.Sample(9000)
	if actual != visible || c.Motion.Effective != decision.Effective {
		t.Fatal("activation restored an old facial snapshot", actual, visible)
	}
}
