//go:build integration

package streaming

import (
	"context"
	"encoding/json"

	"syntelle/api/internal/conversation"
)

// Fixed controller for the private fixture host; never calls a model provider.
type expressionDirectorFixture struct{ *streamModelFixture }

func (m *expressionDirectorFixture) PhraseDirection() bool { return true }
func (m *expressionDirectorFixture) PhraseRequest(string, []conversation.Message) any {
	return map[string]bool{"fixture": true, "provider_calls": false}
}
func (m *expressionDirectorFixture) CompleteWithPhrases(ctx context.Context, _ string, _ []conversation.Message, emit func(conversation.PerformancePhrase) error) (conversation.ModelResult, error) {
	m.calls.Add(1)
	brow, smile, laugh, head := .5, .9, .85, .8
	phrases := []conversation.PerformancePhrase{
		{Text: "Hello from the cached expression fixture.", Tone: "playful", Cues: []conversation.PerformanceCue{{Kind: "brows", Anchor: "Hello", Amount: &brow}, {Kind: "smile", Anchor: "expression", Amount: &smile}, {Kind: "laugh", Anchor: "fixture", Amount: &laugh}}},
		{Text: "Here is another complete phrase.", Tone: "neutral", Cues: []conversation.PerformanceCue{{Kind: "double_nod", Anchor: "another", Amount: &head}, {Kind: "tilt", Anchor: "complete", Amount: &head}}},
	}
	text := ""
	for _, phrase := range phrases {
		if err := ctx.Err(); err != nil {
			return conversation.ModelResult{}, err
		}
		if err := emit(phrase); err != nil {
			return conversation.ModelResult{}, err
		}
		if text != "" {
			text += " "
		}
		text += phrase.Text
	}
	return conversation.ModelResult{Text: text, Response: json.RawMessage(`{"fixture":true,"provider_calls":0}`)}, nil
}
