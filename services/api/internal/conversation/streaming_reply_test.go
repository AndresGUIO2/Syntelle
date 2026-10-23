package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

const openingFixture = "The first stable sentence is ready."
const completeFixture = openingFixture + " The rest of this reply follows."

type growingReplyFixture struct {
	release <-chan struct{}
	fail    bool
}

func (m growingReplyFixture) Settings() any { return map[string]bool{"fixture": true} }
func (m growingReplyFixture) Complete(ctx context.Context, _ string, _ []Message, delta func(string)) (ModelResult, error) {
	return m.CompleteWithOpening(ctx, "", nil, delta, func(string) {})
}
func (m growingReplyFixture) CompleteWithOpening(ctx context.Context, _ string, _ []Message, delta func(string), opening func(string)) (ModelResult, error) {
	delta(openingFixture + " ")
	opening(openingFixture)
	select {
	case <-m.release:
	case <-ctx.Done():
		return ModelResult{Text: openingFixture}, ctx.Err()
	}
	if m.fail {
		return ModelResult{Text: openingFixture}, errors.New("offline incomplete model")
	}
	delta("The rest of this reply follows.")
	return ModelResult{Text: completeFixture, Usage: json.RawMessage(`{}`)}, nil
}

type audibleOpeningFixture struct{ calls atomic.Int32 }

func (s *audibleOpeningFixture) Prepare(context.Context, string, string, string, string) (any, error) {
	return nil, errors.New("whole recording must not run")
}
func (s *audibleOpeningFixture) PrepareOpening(context.Context, string, string, string, string) error {
	return errors.New("preparation-only path must not run")
}
func (s *audibleOpeningFixture) PrepareOpeningPart(_ context.Context, text, _, _, _ string) (SpeechPart, error) {
	s.calls.Add(1)
	return SpeechPart{Text: text, Speech: map[string]any{"audio_base64": make([]byte, 300000), "duration_ms": 7250}}, nil
}
func (s *audibleOpeningFixture) PrepareParts(_ context.Context, _ string, _, _, _ string, emit func(SpeechPart) error) (any, error) {
	for i, text := range []string{openingFixture + " ", "The rest of this reply follows."} {
		if err := emit(SpeechPart{Text: text, Index: i, Final: i == 1, Speech: map[string]any{"fixture": true}}); err != nil {
			return nil, err
		}
	}
	return map[string]any{"all_parts": true}, nil
}

func TestOpeningIsDurableAndAudibleBeforeModelCompletion(t *testing.T) {
	for _, mode := range []string{"complete", "incomplete", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			store := isolatedStore(t)
			release := make(chan struct{})
			voice := &audibleOpeningFixture{}
			cid, tid := "audible-opening-session", "audible-opening-turn"
			f := openSocketFixture(t, New(store, growingReplyFixture{release: release, fail: mode == "incomplete"}, voice, nil), cid)
			f.c.SetReadLimit(8 << 20)
			f.send(Event{Type: "submit", ConversationID: cid, TurnID: tid, Text: "Tell me a complete story."})
			for {
				e := f.read()
				if e.Type == "error" || e.Type == "reply_complete" {
					t.Fatal("waited for completed model", e)
				}
				if e.Type == "speech_part" {
					break
				}
			}
			var pending bool
			if err := store.Pool.QueryRow(f.ctx, `SELECT (diagnostics->>'model_pending')::boolean FROM syntelle_conversation.operations WHERE kind='speech-delivery' AND turn_id=$1`, tid).Scan(&pending); err != nil || !pending {
				t.Fatal("opening not committed before delivery", err)
			}
			if mode == "cancel" {
				f.send(Event{Type: "cancel", ConversationID: cid, TurnID: tid, Reason: "owner stopped reply"})
				readUtteranceEvent(t, f, "cancelled")
				return
			}
			close(release)
			parts := 1
			for {
				e := f.read()
				if e.Type == "speech_part" {
					parts++
				}
				if e.Type == "error" {
					if mode != "incomplete" {
						t.Fatal(e)
					}
					break
				}
				if e.Type == "speech_complete" {
					if mode != "complete" {
						t.Fatal("failed model completed speech")
					}
					data := e.Data.(map[string]any)
					if data["parts"] != float64(2) {
						t.Fatal("incorrect completion count", data)
					}
					break
				}
			}
			want := 2
			if mode == "incomplete" {
				want = 1
			}
			if parts != want || voice.calls.Load() != 1 {
				t.Fatal("lost/duplicated opening", parts, voice.calls.Load())
			}
		})
	}
}

func TestPlaybackEchoAndOriginalInterruptionCause(t *testing.T) {
	spoken := "Once, high in the craggy peaks, there lived an old clockmaker."
	if !matchesPlayback("HIGH in the craggy peaks", spoken) || matchesPlayback("Stop", spoken) || matchesPlayback("Tell me something different", spoken) {
		t.Fatal("incorrect echo classification")
	}
	store := isolatedStore(t)
	cid, tid := "reason-session-01", "reason-turn-01"
	if _, err := store.Session(context.Background(), cid, "syntelle", "en-US", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginTurn(context.Background(), cid, tid, "fixture"); err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{"owner stopped reply", "disconnect"} {
		if err := store.Interrupt(context.Background(), cid, tid, reason); err != nil {
			t.Fatal(err)
		}
	}
	var reason string
	if err := store.Pool.QueryRow(context.Background(), `SELECT interruption->>'reason' FROM syntelle_conversation.turns WHERE id=$1`, tid).Scan(&reason); err != nil || !strings.Contains(reason, "owner stopped") {
		t.Fatal("disconnect overwrote original cause", reason, err)
	}
}

type echoModelFixture struct{ calls atomic.Int32 }

func (m *echoModelFixture) Settings() any { return map[string]bool{"fixture": true} }
func (m *echoModelFixture) Complete(_ context.Context, _ string, _ []Message, delta func(string)) (ModelResult, error) {
	m.calls.Add(1)
	text := "This is a prerecorded fixture reply."
	delta(text)
	return ModelResult{Text: text}, nil
}

func TestMicrophoneNoiseDoesNotCancelSpeechAndVerbatimEchoIsIgnored(t *testing.T) {
	store := isolatedStore(t)
	model := &echoModelFixture{}
	r := controlledRecognitionFixture{events: make(chan RecognitionEvent, 8)}
	cid, tid := "noise-session-001", "noise-reply-001"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "submit", ConversationID: cid, TurnID: tid, Text: "Read a complete reply."})
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: "noise-microphone-001"})
	readUtteranceEvent(t, f, "listening")
	f.send(Event{Type: "playback_started", ConversationID: cid, TurnID: tid})
	f.send(Event{Type: "activity", ConversationID: cid, TurnID: tid})
	r.events <- RecognitionEvent{Type: "partial", Text: ""}
	readUtteranceEvent(t, f, "transcript_partial")
	r.events <- RecognitionEvent{Type: "start"}
	readUtteranceEvent(t, f, "speech_detected")
	r.events <- RecognitionEvent{Type: "final", ID: "noise-echo-result-001", Text: "This is a prerecorded fixture reply."}
	r.events <- RecognitionEvent{Type: "partial", Text: "A new question"}
	if e := readUtteranceEvent(t, f, "transcript_partial"); e.Text != "A new question" {
		t.Fatal("echo became a user utterance", e)
	}
	if model.calls.Load() != 1 {
		t.Fatal("noise or echo made a new request")
	}
	r.events <- RecognitionEvent{Type: "final", ID: "noise-real-result-001", Text: "Please explain something else."}
	readUtteranceEvent(t, f, "transcript_final")
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if model.calls.Load() != 2 {
		t.Fatal("confirmed interruption was lost")
	}
}
