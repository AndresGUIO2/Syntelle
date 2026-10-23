package conversation

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

type phraseTurnFixture struct {
	first     chan struct{}
	completed atomic.Bool
}

func (m *phraseTurnFixture) Settings() any { return map[string]bool{"fixture": true} }
func (m *phraseTurnFixture) Complete(context.Context, string, []Message, func(string)) (ModelResult, error) {
	panic("legacy path used")
}
func (m *phraseTurnFixture) PhraseDirection() bool { return true }
func (m *phraseTurnFixture) PhraseRequest(string, []Message) any {
	return map[string]bool{"fixture": true}
}
func (m *phraseTurnFixture) CompleteWithPhrases(ctx context.Context, _ string, _ []Message, emit func(PerformancePhrase) error) (ModelResult, error) {
	amount := .5
	if err := emit(PerformancePhrase{Text: "Hello there.", Tone: "neutral", Cues: []PerformanceCue{{Kind: "brows", Anchor: "Hello", Amount: &amount}}}); err != nil {
		return ModelResult{}, err
	}
	select {
	case <-ctx.Done():
		return ModelResult{}, ctx.Err()
	case <-m.first:
	}
	if err := emit(PerformancePhrase{Text: "Another thought.", Tone: "neutral", Cues: []PerformanceCue{}}); err != nil {
		return ModelResult{}, err
	}
	m.completed.Store(true)
	return ModelResult{Text: "Hello there. Another thought.", Response: json.RawMessage(`{"fixture":true}`)}, nil
}

type phraseTurnSpeech struct{ calls atomic.Int32 }

func (s *phraseTurnSpeech) Prepare(ctx context.Context, text, language, voice, id string) (any, error) {
	s.calls.Add(1)
	return map[string]any{"audio_base64": performanceFixtureWAV(), "duration_ms": float64(9000), "mime": "audio/wav", "cached": true}, ctx.Err()
}

func TestPhraseTurnDeliversValidatedDirectionBeforeModelCompletion(t *testing.T) {
	store := isolatedStore(t)
	model := &phraseTurnFixture{first: make(chan struct{})}
	speech := &phraseTurnSpeech{}
	service := New(store, model, speech, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cid, id := "phrase_session_0001", "phrase_turn_000001"
	// Match the normal accepted-turn preconditions without creating provider calls.
	if _, err := store.Session(ctx, cid, "syntelle", "en-US", "Fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginTurn(ctx, cid, id, "Hello"); err != nil {
		t.Fatal(err)
	}
	parts := 0
	completed := false
	service.runTurn(ctx, cid, id, "Hello", "en-US", "en-US-Ava:DragonHDLatestNeural", "Fixture", func(event Event) {
		if event.Type == "error" {
			t.Error(event.Text)
		}
		if event.Type == "speech_part" {
			parts++
			if parts == 1 {
				if model.completed.Load() {
					t.Error("first phrase waited for full model completion")
				}
				raw, _ := json.Marshal(event.Data)
				var part struct {
					Speech struct {
						Performance PerformanceValidation `json:"performance"`
					} `json:"speech"`
				}
				if json.Unmarshal(raw, &part) != nil || part.Speech.Performance.Intent.Version != "performance-intent/2" {
					t.Error("phrase direction envelope lost")
				}
				found := false
				for _, cue := range part.Speech.Performance.Resolved {
					if cue.Kind == "brows" && cue.Amount != nil && *cue.Amount == .5 {
						found = true
					}
				}
				if !found {
					t.Error("numeric eyebrow direction lost")
				}
				close(model.first)
			}
		}
		if event.Type == "speech_complete" {
			completed = true
		}
	})
	if !completed || parts != 2 || speech.calls.Load() != 2 {
		t.Fatal("phrase delivery incomplete", completed, parts, speech.calls.Load())
	}
}

func TestPhraseTurnCancellationJoinsTheModelAndRestoresDurableState(t *testing.T) {
	store := isolatedStore(t)
	model := &phraseTurnFixture{first: make(chan struct{})}
	service := New(store, model, &phraseTurnSpeech{}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cid, id := "phrase_cancel_session", "phrase_cancel_turn"
	if _, err := store.Session(ctx, cid, "syntelle", "en-US", "Fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginTurn(ctx, cid, id, "Hello"); err != nil {
		t.Fatal(err)
	}
	service.runTurn(ctx, cid, id, "Hello", "en-US", "en-US-Ava:DragonHDLatestNeural", "Fixture", func(event Event) {
		if event.Type == "speech_part" {
			cancel()
		}
	})
	check, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	turns, err := store.Turns(check, cid, 1)
	if err != nil || len(turns) != 1 || turns[0].Status != "interrupted" || model.completed.Load() {
		t.Fatal("canceled phrase was not joined and saved", turns, err)
	}
}
