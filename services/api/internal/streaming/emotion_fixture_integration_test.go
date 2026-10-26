//go:build integration

package streaming

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"syntelle/api/internal/conversation"
	"syntelle/render-contract"
)

// A deterministic director plus cached PCM validates delivery, not a model's
// interpretation of emotions. This adapter is absent from production builds.
type emotionDirectorFixture struct {
	*streamModelFixture
	mu         sync.Mutex
	directions []string
}

func (m *emotionDirectorFixture) PhraseDirection() bool { return true }
func (m *emotionDirectorFixture) PhraseRequest(string, []conversation.Message) any {
	return map[string]bool{"fixture": true, "provider_calls": false}
}
func (m *emotionDirectorFixture) CompleteWithPhrases(ctx context.Context, instructions string, _ []conversation.Message, emit func(conversation.PerformancePhrase) error) (conversation.ModelResult, error) {
	m.mu.Lock()
	m.directions = append(m.directions, instructions)
	m.mu.Unlock()
	m.calls.Add(1)
	text := []string{}
	for i, state := range []string{"joyful", "sad", "angry"} {
		phrase := conversation.PerformancePhrase{Version: "performance-intent/3", Text: []string{"This is a cheerful reaction.", "Now the context is sad.", "This is a firm, angry response."}[i], Tone: "neutral", Cues: []conversation.PerformanceCue{}, Emotion: &rendercontract.EmotionTarget{State: state, Intensity: .9, Energy: .5}}
		if ctx.Err() != nil {
			return conversation.ModelResult{}, ctx.Err()
		}
		if err := emit(phrase); err != nil {
			return conversation.ModelResult{}, err
		}
		text = append(text, phrase.Text)
	}
	return conversation.ModelResult{Text: strings.Join(text, " "), Response: json.RawMessage(`{"fixture":true,"provider_calls":0}`)}, nil
}
