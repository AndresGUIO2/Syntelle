package conversation

import (
	"syntelle/render-contract"
	"testing"
)

func TestApprovedEmotionSuppressesConflictingPlayfulCues(t *testing.T) {
	amount := .7
	intent := PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "reassure", Text: "Stay close.", Tone: "playful", Emotion: &rendercontract.EmotionTarget{State: "playful", Intensity: .5, Energy: .6}, Cues: []PerformanceCue{{Kind: "smile", Anchor: "Stay close", Amount: &amount}, {Kind: "laugh", Anchor: "Stay close", Amount: &amount}}}
	reservation := &rendercontract.DeliveryReservation{Expressive: true, Version: rendercontract.DeliveryVersion, Token: "fixture_token", Instance: "fixture_instance", Effective: rendercontract.EmotionTarget{State: "fearful", Intensity: .6, Energy: .5}, Request: rendercontract.DeliveryRequest{RequestID: "fixture_request", TurnID: "fixture_turn", Revision: 1, TextHash: rendercontract.TextHash(intent.Text), Voice: rendercontract.DefaultVoice, Language: "en-US", Purpose: "reassure"}}
	actual := coherentIntent(intent, reservation)
	validated := ValidatePerformance(actual, intent.Text)
	if actual.Emotion.State != "fearful" || actual.Tone != "serious" || len(validated.Intent.Cues) != 0 {
		t.Fatal("approved fear acquired amusement", actual, validated)
	}
	if intent.Emotion.State != "playful" || intent.Tone != "playful" {
		t.Fatal("proposal evidence was mutated")
	}
	legacy := coherentIntent(intent, nil)
	if legacy.Tone != intent.Tone || legacy.Emotion != intent.Emotion {
		t.Fatal("legacy direction changed")
	}
}
