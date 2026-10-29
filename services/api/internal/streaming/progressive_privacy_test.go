package streaming

import (
	"syntelle/api/internal/conversation"
	"testing"
)

func TestProgressiveAudioStaysOnPrivateWorkerChannel(t *testing.T) {
	if _, ok := publicConversation(conversation.Event{Type: "speech_prefix", Text: "private", Data: map[string]any{"audio_base64": "private-native-audio"}}, "stream"); ok {
		t.Fatal("native prefix leaked to public player events")
	}
}
