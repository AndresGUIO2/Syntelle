//go:build integration

package streaming

import (
	"context"
	"encoding/json"
	"strings"
	"syntelle/api/internal/conversation"
)

// This director is absent from production. It replays approved cached PCM;
// captions establish test anchors, not a claim of word-aligned synthesis.
type gestureDirectorFixture struct{ *streamModelFixture }

func (m *gestureDirectorFixture) PhraseDirection() bool { return true }
func (m *gestureDirectorFixture) PhraseRequest(string, []conversation.Message) any {
	return map[string]bool{"offline_gestures": true}
}
func (m *gestureDirectorFixture) CompleteWithPhrases(ctx context.Context, _ string, _ []conversation.Message, emit func(conversation.PerformancePhrase) error) (conversation.ModelResult, error) {
	m.calls.Add(1)
	texts := []string{}
	amount := .85
	one := 1
	for i, kind := range []string{"nod", "", "shake", "", "emphasis", "", "tilt", ""} {
		if err := ctx.Err(); err != nil {
			return conversation.ModelResult{}, err
		}
		text := "A calm pause between the points."
		if kind != "" {
			text = []string{"Yes, that is the point.", "", "That is not the same thing.", "", "This point deserves emphasis.", "", "Could we consider this possibility?", ""}[i]
		}
		cues := []conversation.PerformanceCue{}
		if kind != "" {
			cues = append(cues, conversation.PerformanceCue{Kind: kind, Anchor: text, Amount: &amount, Repetitions: &one}, conversation.PerformanceCue{Kind: "brows", Anchor: text, Amount: &amount, Repetitions: &one})
		}
		if err := emit(conversation.PerformancePhrase{Version: "performance-intent/3", Text: text, Tone: "neutral", Cues: cues}); err != nil {
			return conversation.ModelResult{}, err
		}
		texts = append(texts, text)
	}
	return conversation.ModelResult{Text: strings.Join(texts, " "), Response: json.RawMessage(`{"offline_gestures":true,"provider_calls":0}`)}, nil
}
