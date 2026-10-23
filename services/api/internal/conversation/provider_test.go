package conversation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestResponsesStreamAndPrivateSettings(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("api-key") != "test-key" || r.URL.Path != "/openai/v1/responses" {
			t.Error("Wrong private request")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "owner-deployment" || body["stream"] != true || body["store"] != false || body["reasoning"].(map[string]any)["effort"] != "none" {
			t.Error(body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello.\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"id\":\"resp-test\",\"usage\":{\"output_tokens\":2}}}\n\n"))
	}))
	defer server.Close()
	p := &Responses{endpoint: server.URL + "/openai/v1/responses", deployment: "owner-deployment", key: "test-key", client: server.Client()}
	delta := ""
	result, e := p.Complete(context.Background(), "Be helpful", []Message{{"user", "Hi"}}, func(s string) { delta += s })
	if e != nil || result.Text != "Hello." || delta != result.Text || calls != 1 || !strings.Contains(string(result.Usage), "output_tokens") {
		t.Fatal(result, e, calls)
	}
	settings, _ := json.Marshal(p.Settings())
	if strings.Contains(string(settings), "test-key") {
		t.Fatal("Credential archived")
	}
}
func TestResponsesFailureNeverRetriesOrAcceptsIncomplete(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(429) }))
	defer server.Close()
	p := &Responses{endpoint: server.URL, deployment: "luna", key: "secret", client: server.Client()}
	if _, e := p.Complete(context.Background(), "", nil, func(string) {}); e == nil || calls != 1 {
		t.Fatal("Provider failure retried")
	}
	if _, e := readResponses(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n"), func(string) {}); e == nil {
		t.Fatal("Truncated response accepted")
	}
	for _, endpoint := range []string{"http://bad.openai.azure.com", "https://evil.example", "https://user:secret@x.openai.azure.com"} {
		if _, e := NewResponses(endpoint, "deployment", "key"); e == nil {
			t.Fatal("Invalid endpoint accepted")
		}
	}
}
func TestSpokenTextAndContextWindow(t *testing.T) {
	long := "First sentence. " + strings.Repeat("More words ", 80)
	if SpokenText(long) != strings.TrimSpace(long) {
		t.Fatal("Long reply was shortened")
	}
	emoji := strings.Repeat("🙂", 400)
	if len(utf16.Encode([]rune(SpokenText(emoji)))) != 800 {
		t.Fatal("Unicode reply was shortened")
	}
	turns := []Turn{}
	for i := 0; i < 30; i++ {
		turns = append(turns, Turn{User: "question", Reply: "answer"})
	}
	turns = append(turns, Turn{User: "current"})
	context := ContextMessages(turns)
	if len(context) != 41 || context[len(context)-1].Content != "current" {
		t.Fatal("Latest 20 exchanges plus current not preserved", len(context))
	}
}

func TestDelayedEchoCannotBecomeModelTurn(t *testing.T) {
	echo := RecognitionEvent{OffsetMS: 1000, DurationMS: 600}
	windows := []playbackWindow{{900, 1700}}
	if !overlapsPlayback(echo, windows, 2200, -10000) {
		t.Fatal("Delayed assistant echo was accepted")
	}
	if overlapsPlayback(echo, windows, 2200, 1300) {
		t.Fatal("Locally confirmed interruption was rejected")
	}
	user := RecognitionEvent{OffsetMS: 1800, DurationMS: 600}
	if overlapsPlayback(user, windows, 2400, -10000) {
		t.Fatal("Post-playback user speech was rejected")
	}
}
