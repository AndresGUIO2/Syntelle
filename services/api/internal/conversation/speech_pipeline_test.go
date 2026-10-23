package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
)

type earlyModelFixture struct {
	speechStarted <-chan struct{}
	fail          bool
}

func (m earlyModelFixture) Settings() any         { return map[string]bool{"fixture": true} }
func (m earlyModelFixture) EarlySpeechText() bool { return true }
func (m earlyModelFixture) Complete(ctx context.Context, _ string, _ []Message, delta func(string)) (ModelResult, error) {
	delta("The opening phrase is ready. The complete remaining explanation follows.")
	select {
	case <-m.speechStarted:
	case <-ctx.Done():
		return ModelResult{}, ctx.Err()
	}
	if m.fail {
		return ModelResult{}, errors.New("fixture incomplete model stream")
	}
	return ModelResult{Text: "The opening phrase is ready. The complete remaining explanation follows.", Usage: json.RawMessage("{}")}, nil
}

type progressiveFixture struct {
	started chan struct{}
	release chan struct{}
	stopped chan struct{}
	calls   atomic.Int32
}

func (p *progressiveFixture) Prepare(context.Context, string, string, string, string) (any, error) {
	return nil, errors.New("whole reply path must not run")
}
func (p *progressiveFixture) PrepareParts(ctx context.Context, _ string, _, _, _ string, emit func(SpeechPart) error) (any, error) {
	p.calls.Add(1)
	defer close(p.stopped)
	close(p.started)
	if err := emit(SpeechPart{Text: "The opening phrase is ready.", Speech: map[string]any{"fixture": true}, Index: 0}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.release:
	}
	if err := emit(SpeechPart{Text: "The complete remaining explanation follows.", Speech: map[string]any{"fixture": true}, Index: 1, Final: true, OffsetMS: 1000}); err != nil {
		return nil, err
	}
	return map[string]any{"all_parts": true}, nil
}

func TestProgressiveReplyOverlapsModelAndPublishesBeforeRemainder(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "cancel"}[cancel], func(t *testing.T) {
			store := isolatedStore(t)
			speech := &progressiveFixture{started: make(chan struct{}), release: make(chan struct{}), stopped: make(chan struct{})}
			service := New(store, earlyModelFixture{speechStarted: speech.started}, speech, nil)
			cid, tid := "progressive-session-001", "progressive-turn-001"
			f := openSocketFixture(t, service, cid)
			f.send(Event{Type: "submit", ConversationID: cid, TurnID: tid, Text: "Explain this."})
			for {
				e := f.read()
				if e.Type == "error" {
					t.Fatal(e.Text)
				}
				if e.Type == "speech_part" {
					break
				}
			}
			// The provider's second phrase is still blocked: playback does not wait.
			if cancel {
				f.send(Event{Type: "cancel", ConversationID: cid, TurnID: tid})
				for {
					if f.read().Type == "cancelled" {
						break
					}
				}
				select {
				case <-speech.stopped:
				case <-f.ctx.Done():
					t.Fatal("speech worker leaked")
				}
			} else {
				close(speech.release)
				parts := 1
				for {
					e := f.read()
					if e.Type == "error" || e.Type == "prepared_speech" {
						t.Fatal("failed or duplicated whole audio", e.Type, e.Text)
					}
					if e.Type == "speech_part" {
						parts++
					}
					if e.Type == "speech_complete" {
						break
					}
				}
				if parts != 2 {
					t.Fatal("lost phrase", parts)
				}
				var status, text string
				if err := store.Pool.QueryRow(f.ctx, "SELECT status,spoken->>'text' FROM syntelle_conversation.turns WHERE id=$1", tid).Scan(&status, &text); err != nil || status != "ready" || text != "The opening phrase is ready. The complete remaining explanation follows." {
					t.Fatal(status, text, err)
				}
			}
			if speech.calls.Load() != 1 {
				t.Fatal("duplicate synthesis")
			}
		})
	}
}

func TestIncompleteModelNeverPublishesSpeculativeAudio(t *testing.T) {
	store := isolatedStore(t)
	speech := &progressiveFixture{started: make(chan struct{}), release: make(chan struct{}), stopped: make(chan struct{})}
	service := New(store, earlyModelFixture{speechStarted: speech.started, fail: true}, speech, nil)
	f := openSocketFixture(t, service, "incomplete-preview-session")
	f.send(Event{Type: "submit", ConversationID: "incomplete-preview-session", TurnID: "incomplete-preview-turn", Text: "Fixture"})
	for {
		e := f.read()
		if e.Type == "speech_part" || e.Type == "prepared_speech" {
			t.Fatal("unvalidated audio escaped")
		}
		if e.Type == "error" {
			break
		}
	}
	select {
	case <-speech.stopped:
	case <-f.ctx.Done():
		t.Fatal("preview worker leaked")
	}
}
