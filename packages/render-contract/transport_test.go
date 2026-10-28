package rendercontract

import (
	"encoding/json"
	"testing"
)

func TestWireCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
		wire  string
	}{
		{"control", Envelope{Type: "signal", SessionID: "s1", CommandID: "c1", Data: json.RawMessage(`{"candidate":null}`)}, `{"type":"signal","session_id":"s1","command_id":"c1","data":{"candidate":null}}`},
		{"registration", Envelope{Type: "registered"}, `{"type":"registered"}`},
		{"signaling", Signal{Description: &SessionDescription{Type: "offer", SDP: "v=0\r\n"}}, `{"description":{"type":"offer","sdp":"v=0\r\n"}}`},
		{"viseme", Viseme{ID: 4, OffsetMS: 125.5}, `{"id":4,"offset_ms":125.5}`},
		{"settings", Defaults(), `{"emotion_baseline":{"state":"neutral","intensity":0},"emotion_regulation":{"enabled":true,"temperament":"calm"},"character":"syntelle","instructions":"Be a clear, friendly assistant. Match the depth and length to the question, and explain fully when asked. Do not invent company knowledge.","language":"en-US","voice":"en-US-Emma:DragonHDLatestNeural","background":"light-studio","blur":0,"blinking":true,"movement":true,"automatic_expressions":true,"smile":0,"brows":0}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.wire {
				t.Fatalf("wire changed: %s", got)
			}
		})
	}
}
