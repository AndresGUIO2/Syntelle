//go:build integration

package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"syntelle/api/internal/conversation"
	"syntelle/render-contract"
)

// This review demonstrates delivery of existing facial emotion with exact cached
// text/PCM. It is not evidence of LLM interpretation or emotional voice synthesis.
type performanceReviewDirectorFixture struct {
	*streamModelFixture
	text string
}

func (m *performanceReviewDirectorFixture) PhraseDirection() bool { return true }
func (m *performanceReviewDirectorFixture) PhraseRequest(string, []conversation.Message) any {
	return map[string]any{"offline_performance_review": true, "exact_cached_text": m.text}
}
func (m *performanceReviewDirectorFixture) CompleteWithPhrases(ctx context.Context, _ string, _ []conversation.Message, emit func(conversation.PerformancePhrase) error) (conversation.ModelResult, error) {
	if err := ctx.Err(); err != nil {
		return conversation.ModelResult{}, err
	}
	n := m.calls.Add(1)
	state := "curious"
	if n%2 == 0 {
		state = "affectionate"
	}
	amount, one := .65, 1
	p := conversation.PerformancePhrase{Version: "performance-intent/3", Text: m.text, Tone: "neutral",
		Emotion: &rendercontract.EmotionTarget{State: state, Intensity: .7, Energy: .6},
		Cues:    []conversation.PerformanceCue{{Kind: "brows", Anchor: m.text, Amount: &amount, Repetitions: &one}, {Kind: "emphasis", Anchor: m.text, Amount: &amount, Repetitions: &one}}}
	if os.Getenv("SYNTELLE_STREAMING_FIXTURE_COORDINATED") == "1" {
		p.Version = rendercontract.CoordinatedPerformanceVersion
		p.Purpose = "explain"
	}
	count := 1
	if os.Getenv("SYNTELLE_STREAMING_FIXTURE_PAIRED_PHRASES") == "1" {
		count = 2
	}
	for i := 0; i < count; i++ {
		if err := emit(p); err != nil {
			return conversation.ModelResult{}, err
		}
	}
	return conversation.ModelResult{Text: strings.TrimSpace(strings.Repeat(m.text+" ", count)), Response: json.RawMessage(`{"offline_performance_review":true,"exact_cached_text":true,"provider_calls":0}`)}, nil
}

func (v *cachedVoiceFixture) VoiceDirection(r rendercontract.DeliveryReservation) (rendercontract.VoiceDelivery, error) {
	if voice, ok := v.payload["voice"].(string); ok && voice != r.Request.Voice {
		return rendercontract.VoiceDelivery{}, errors.New("cached fixture voice must not be relabeled")
	}
	return rendercontract.VoiceDelivery{Version: rendercontract.DeliveryVersion, Purpose: r.Request.Purpose, Emotion: r.Effective, CapabilityRevision: "cached-visual-review/1"}, nil
}
func (v *cachedVoiceFixture) PrepareDirected(ctx context.Context, text, language, voice, id string, _ rendercontract.VoiceDelivery) (any, error) {
	return v.Prepare(ctx, text, language, voice, id)
}

func TestPerformanceReviewKeepsTextEmotionAndGestureTogether(t *testing.T) {
	m := &performanceReviewDirectorFixture{streamModelFixture: &streamModelFixture{}, text: "Hello. My voice and facial movements share the same clock."}
	for _, state := range []string{"curious", "affectionate"} {
		var got conversation.PerformancePhrase
		result, err := m.CompleteWithPhrases(context.Background(), "", nil, func(p conversation.PerformancePhrase) error { got = p; return nil })
		if err != nil || result.Text != m.text || got.Text != m.text || got.Emotion == nil || got.Emotion.State != state || len(got.Cues) != 2 {
			t.Fatal("review lost exact text or facial direction", got, err)
		}
	}
}

func TestPerformanceReviewRejectsSubstituteTextBeforePersistence(t *testing.T) {
	v := &cachedVoiceFixture{expectedText: "Actual cached speech text."}
	_, err := v.Prepare(context.Background(), "Unrelated gesture caption.", "en-US", "voice", "reply")
	if err == nil || !strings.Contains(err.Error(), "exact source text") || v.calls.Load() != 0 {
		t.Fatal("unrelated text was accepted", err)
	}
}
