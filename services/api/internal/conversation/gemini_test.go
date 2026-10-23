package conversation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func geminiFixtureStream() string {
	return `data: {"event_type":"interaction.created","interaction":{"id":"gemini-fixture-01","model":"gemini-3.8-flash","status":"in_progress"}}

data: {"event_type":"step.start","index":0,"step":{"type":"thought"}}

data: {"event_type":"step.delta","index":0,"delta":{"type":"text","text":"Do not show this thought"}}

data: {"event_type":"step.start","index":1,"step":{"type":"model_output"}}

data: {"event_type":"step.delta","index":1,"delta":{"type":"text","text":"Hello. "}}

data: {"event_type":"step.delta","index":1,"delta":{"type":"text","text":"How can I help?"},"metadata":{"total_usage":{"total_tokens":36}}}

data: {"event_type":"interaction.completed","interaction":{"id":"gemini-fixture-01","model":"gemini-3.8-flash","status":"completed"}}

`
}
func TestGeminiStatelessHistoryStreamingAndPrivateKey(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-goog-api-key") != "private-fixture-key" || r.URL.RawQuery != "" {
			t.Error("Private key routing changed")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != GeminiChatModel || body["stream"] != true || body["store"] != false {
			t.Error(body)
		}
		if body["generation_config"].(map[string]any)["thinking_level"] != "low" {
			t.Error("Unsupported thinking configuration")
		}
		input := body["input"].([]any)
		if len(input) != 3 || input[0].(map[string]any)["type"] != "user_input" || input[1].(map[string]any)["type"] != "model_output" {
			t.Error("Conversation history roles lost")
		}
		if !strings.Contains(body["system_instruction"].(string), "Selected language: es-CO") {
			t.Error("Instructions lost")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(geminiFixtureStream()))
	}))
	defer server.Close()
	p, e := NewGemini("private-fixture-key", GeminiChatModel)
	if e != nil {
		t.Fatal(e)
	}
	p.endpoint = server.URL
	p.client = server.Client()
	output := ""
	result, e := p.Complete(context.Background(), "Selected language: es-CO", []Message{{"user", "Hello"}, {"assistant", "Hi"}, {"user", "Hola"}}, func(delta string) { output += delta })
	if e != nil || result.Text != "Hello. How can I help?" || output != result.Text || calls != 1 || result.RequestID != "gemini-fixture-01" || !strings.Contains(string(result.Usage), "36") {
		t.Fatal(result, e)
	}
	settings, _ := json.Marshal(p.Settings())
	if strings.Contains(string(settings), "private-fixture-key") {
		t.Fatal("Credential leaked into settings")
	}
	if !strings.Contains(string(result.Response), "interaction.completed") {
		t.Fatal("Raw provider stream was not retained")
	}
}
func TestGeminiRejectsIncompleteAndNoRetryOrFallback(t *testing.T) {
	complete := geminiFixtureStream()
	for _, stream := range []string{strings.Split(complete, "data: {\"event_type\":\"interaction.completed\"")[0], strings.ReplaceAll(complete, `"status":"completed"`, `"status":"incomplete"`), strings.ReplaceAll(complete, GeminiChatModel, "wrong-model")} {
		if _, e := readGemini(strings.NewReader(stream), GeminiChatModel, func(string) {}); e == nil {
			t.Fatal("Invalid stream accepted")
		}
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":{"message":"private-fixture-key rejected"}}`))
	}))
	defer server.Close()
	p, _ := NewGemini("private-fixture-key", GeminiChatModel)
	p.endpoint = server.URL
	p.client = server.Client()
	result, e := p.Complete(context.Background(), "", nil, func(string) {})
	if e == nil || calls != 1 || strings.Contains(string(result.Response), "private-fixture-key") {
		t.Fatal("Provider error retried or credential exposed")
	}
	if _, e = NewGemini("key", "gemini-flash-latest"); e == nil {
		t.Fatal("Unpinned model accepted")
	}
}
