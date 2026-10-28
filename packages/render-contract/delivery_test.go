package rendercontract

import "testing"

func TestCoordinatedDefaultVoiceAndVersion(t *testing.T) {
	if Defaults().Voice != "en-US-Emma:DragonHDLatestNeural" {
		t.Fatal("new sessions must use Emma")
	}
	if !DirectedPerformance("performance-intent/4") {
		t.Fatal("coordinated phrase direction is not admitted")
	}
}

func TestDeliveryIndexDoesNotLimitFullReplyToQueueCapacity(t *testing.T) {
	r := DeliveryRequest{RequestID: "fixture_request", TurnID: "fixture_turn", PhraseIndex: 40, Revision: 1, TextHash: TextHash("Forty complete thoughts."), Voice: DefaultVoice, Language: "en-US", Purpose: "explain"}
	if !r.Valid() {
		t.Fatal("queue capacity became a whole-reply phrase ceiling")
	}
}
