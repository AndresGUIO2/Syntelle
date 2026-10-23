package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpeningPrefixWaitsForCompleteEscapesAndStableBoundaries(t *testing.T) {
	for _, raw := range []string{
		"{\"text\":\"An unfinished escape \\",
		"{\"text\":\"An unfinished unicode \\uD83D",
		"{\"text\":\"<speak>Do not expose markup",
		"{\"text\":\"partial \\u12",
	} {
		if decodedTextPrefix(raw) != "" {
			t.Fatal("unsafe partial string admitted", raw)
		}
	}
	text := "This opening phrase is ready. A full explanation follows."
	encoded, _ := json.Marshal(text)
	prefix := "{\"version\":\"performance-intent/1\",\"text\":" + string(encoded[:len(encoded)-1])
	if got := decodedTextPrefix(prefix); got != text {
		t.Fatal(got)
	}
	if OpeningPhrase("The number is 3.14 and remains exact. Rest", false) != "The number is 3.14 and remains exact." {
		t.Fatal("decimal split")
	}
	for _, text := range []string{"Brief answer.", "A complete explanation with 🙂. Another sentence.", strings.Repeat("word ", 100)} {
		full := OpeningPhrase(text, true)
		growing := OpeningPhrase(text+" ", false)
		if growing != "" && full != growing {
			t.Fatal("opening changed", full, growing)
		}
	}
}

func TestGeminiPreparesOpeningBeforeFullTextWithoutExposingJSON(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	opened := make(chan struct{})
	visible := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"event_type\":\"step.start\",\"index\":0,\"step\":{\"type\":\"model_output\"}}\n\n")
		fragment := "{\"text\":\"The opening phrase is ready. The remaining"
		emit := func(s string) {
			data, _ := json.Marshal(map[string]any{"event_type": "step.delta", "index": 0, "delta": map[string]string{"type": "text", "text": s}})
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
		emit(fragment)
		select {
		case <-opened:
		case <-ctx.Done():
			return
		}
		select {
		case <-visible:
		case <-ctx.Done():
			return
		}
		emit(" explanation follows.\",\"version\":\"performance-intent/1\",\"tone\":\"neutral\",\"cues\":[]}")
		fmt.Fprint(w, "data: {\"event_type\":\"interaction.completed\",\"interaction\":{\"status\":\"completed\",\"model\":\"gemini-3.8-flash\"}}\n\n")
	}))
	defer server.Close()
	model, _ := NewGemini("fixture", GeminiChatModel)
	model.endpoint = server.URL
	model.WithDirection()
	var text strings.Builder
	seen := false
	result, err := model.CompleteWithOpening(ctx, "", nil, func(s string) {
		text.WriteString(s)
		if !seen {
			close(visible)
			seen = true
		}
	}, func(s string) {
		if s != "The opening phrase is ready." {
			t.Error("incorrect opening", s)
		}
		close(opened)
	})
	if err != nil || result.Text != text.String() || result.Text != "The opening phrase is ready. The remaining explanation follows." {
		t.Fatal(result, text.String(), err)
	}
}
