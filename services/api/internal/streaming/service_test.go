package streaming

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syntelle/api/internal/conversation"
)

type configuredVoiceOnly struct{}

func (configuredVoiceOnly) Prepare(context.Context, string, string, string, string) (any, error) {
	return nil, nil
}
func TestSpeechReadinessDoesNotDependOnConversationModel(t *testing.T) {
	service := &Service{conversation: conversation.New(nil, nil, configuredVoiceOnly{}, nil)}
	result := service.response(&streamSession{}, "")
	if result.Configured || !result.SpeechConfigured {
		t.Fatal("literal speech incorrectly depends on the conversation model")
	}
}

func TestConnectionTicketsAreBoundedAndSingleUse(t *testing.T) {
	for _, mode := range []string{"valid", "wrong", "expired", "replayed"} {
		t.Run(mode, func(t *testing.T) {
			session := &streamSession{}
			ticket, err := issueTicket(session)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			switch mode {
			case "wrong":
				ticket += "wrong"
			case "expired":
				now = session.ticketExpires
			case "replayed":
				if !consumeTicket(session, ticket, now) {
					t.Fatal("first use failed")
				}
			}
			if got := consumeTicket(session, ticket, now); got != (mode == "valid") {
				t.Fatalf("ticket mode=%s accepted=%v", mode, got)
			}
		})
	}
}

func TestStreamAccessRemainsLocal(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin string
		want               bool
	}{
		{"local browser", "127.0.0.1:8080", "http://127.0.0.1:5173", true},
		{"local integration", "localhost:8080", "", true},
		{"foreign origin", "localhost:8080", "https://attacker.example", false},
		{"rebound host", "attacker.example:8080", "http://127.0.0.1:5173", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+tc.host+"/api/v1/streams", nil)
			r.Header.Set("Origin", tc.origin)
			if got := localRequest(r); got != tc.want {
				t.Fatalf("access=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestPublicEventsExcludePrivateSpeechAndPayloads(t *testing.T) {
	for _, kind := range []string{"speech_part", "prepared_speech", "provider_debug"} {
		t.Run(kind, func(t *testing.T) {
			_, ok := publicConversation(conversation.Event{Type: kind, Audio: []byte("private"), Data: map[string]string{"audio_base64": "private"}}, "public-stream")
			if ok {
				t.Fatal("private event reached player")
			}
		})
	}
	public, ok := publicConversation(conversation.Event{Type: "reply_complete", ConversationID: "private-conversation", Text: "Hello", Audio: []byte("private"), Data: map[string]string{"audio_base64": "private", "provider_key": "secret"}}, "public-stream")
	if !ok || public.ConversationID != "public-stream" {
		t.Fatal(public)
	}
	data, _ := json.Marshal(public)
	if strings.Contains(string(data), "private") || strings.Contains(string(data), "secret") || strings.Contains(string(data), "audio_base64") {
		t.Fatalf("private data in player event: %s", data)
	}
}
