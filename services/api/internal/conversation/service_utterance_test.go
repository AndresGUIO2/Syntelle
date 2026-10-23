package conversation

import (
	"context"
	"testing"
	"time"
)

type controlledRecognitionFixture struct{ events chan RecognitionEvent }

func (r controlledRecognitionFixture) Run(ctx context.Context, _ string, _ <-chan []byte, emit func(RecognitionEvent)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-r.events:
			emit(event)
		}
	}
}

func readUtteranceEvent(t *testing.T, f socketFixture, kind string) Event {
	t.Helper()
	for {
		event := f.read()
		if event.Type == "error" || event.Type == "microphone_error" {
			t.Fatal(event.Text)
		}
		if event.Type == kind {
			return event
		}
		if event.Type == "accepted" || event.Type == "transcript_final" {
			t.Fatalf("Unexpected premature/duplicate turn: %s", event.Type)
		}
	}
}

func TestRecognitionSegmentsProduceOneTurnAfterContinuationFinishes(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 2)}
	voice := &speechFixture{}
	r := controlledRecognitionFixture{events: make(chan RecognitionEvent, 16)}
	f := openSocketFixture(t, New(store, model, voice, r), "utterance-session-01")
	f.send(Event{Type: "mic_start", ConversationID: "utterance-session-01", TurnID: "utterance-microphone-01"})
	readUtteranceEvent(t, f, "listening")
	first := RecognitionEvent{Type: "final", ID: "native-phrase-0001", Text: "What did you do?"}
	r.events <- first
	if event := readUtteranceEvent(t, f, "transcript_partial"); event.Text != first.Text {
		t.Fatal(event)
	}
	r.events <- RecognitionEvent{Type: "start"}
	readUtteranceEvent(t, f, "speech_detected")
	// A real continuation can take longer than the quiet deadline. It must not
	// release a model request while the SDK is still recognizing that segment.
	<-time.After(utterancePause + 100*time.Millisecond)
	if model.calls.Load() != 0 {
		t.Fatal("Reply started while a continuation was being recognized")
	}
	r.events <- RecognitionEvent{Type: "partial", Text: "In your daily"}
	if event := readUtteranceEvent(t, f, "transcript_partial"); event.Text != "What did you do? In your daily" {
		t.Fatal(event)
	}
	second := RecognitionEvent{Type: "final", ID: "native-phrase-0002", Text: "In your daily routine."}
	r.events <- second
	r.events <- first
	r.events <- second
	want := "What did you do? In your daily routine."
	final := readUtteranceEvent(t, f, "transcript_final")
	if final.TurnID != first.ID || final.Text != want {
		t.Fatal(final)
	}
	accepted := readUtteranceEvent(t, f, "accepted")
	if accepted.TurnID != first.ID || accepted.Text != want {
		t.Fatal(accepted)
	}
	readUtteranceEvent(t, f, "prepared_speech")
	if got := <-model.input; got != want || model.calls.Load() != 1 || voice.calls.Load() != 1 {
		t.Fatal("Fragments produced multiple/incomplete requests", got)
	}
	turns, err := store.Turns(f.ctx, "utterance-session-01", 21)
	var results int
	countErr := store.Pool.QueryRow(f.ctx, "SELECT count(*) FROM syntelle_conversation.recognition_results").Scan(&results)
	if err != nil || len(turns) != 1 || turns[0].User != want || countErr != nil || results != 2 {
		t.Fatal("Merged turn or original recognition evidence was lost", err, countErr, results)
	}
	// A later deliberate utterance must remain a separate turn.
	f.send(Event{Type: "playback_ended", ConversationID: "utterance-session-01", TurnID: first.ID})
	r.events <- RecognitionEvent{Type: "final", ID: "native-phrase-0003", Text: "A different question."}
	if event := readUtteranceEvent(t, f, "transcript_final"); event.Text != "A different question." {
		t.Fatal(event)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if got := <-model.input; got != "A different question." || model.calls.Load() != 2 {
		t.Fatal("A later turn was incorrectly merged", got)
	}
}

func TestPendingRecognitionIsDiscardedOnStopOrFailure(t *testing.T) {
	for _, mode := range []string{"mic_stop", "recognition_error", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			store := isolatedStore(t)
			model := &modelFixture{}
			r := controlledRecognitionFixture{events: make(chan RecognitionEvent, 8)}
			service := New(store, model, &speechFixture{}, r)
			f := openSocketFixture(t, service, "discard-session-01")
			f.send(Event{Type: "mic_start", ConversationID: "discard-session-01", TurnID: "discard-microphone-01"})
			readUtteranceEvent(t, f, "listening")
			r.events <- RecognitionEvent{Type: "final", ID: "discard-phrase-01", Text: "Do not send this unfinished turn."}
			readUtteranceEvent(t, f, "transcript_partial")
			switch mode {
			case "mic_stop":
				f.send(Event{Type: "mic_stop", ConversationID: "discard-session-01", TurnID: "discard-microphone-01"})
			case "recognition_error":
				r.events <- RecognitionEvent{Type: "error", Text: "Offline recognition failure"}
				if event := f.read(); event.Type != "microphone_error" {
					t.Fatal(event)
				}
			case "disconnect":
				f.c.CloseNow()
			}
			<-time.After(utterancePause + 100*time.Millisecond)
			turns, err := store.Turns(f.ctx, "discard-session-01", 21)
			if err != nil || len(turns) != 0 || model.calls.Load() != 0 {
				t.Fatal("A cancelled pending turn reached the model", err)
			}
		})
	}
}

func TestNoMatchReleasesOnlyPreviouslyConfirmedSpeech(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 1)}
	r := controlledRecognitionFixture{events: make(chan RecognitionEvent, 8)}
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), "nomatch-session-01")
	f.send(Event{Type: "mic_start", ConversationID: "nomatch-session-01", TurnID: "nomatch-microphone-01"})
	readUtteranceEvent(t, f, "listening")
	r.events <- RecognitionEvent{Type: "final", ID: "nomatch-phrase-01", Text: "A confirmed question."}
	r.events <- RecognitionEvent{Type: "partial", Text: "An unconfirmed guess"}
	r.events <- RecognitionEvent{Type: "end"}
	if event := readUtteranceEvent(t, f, "transcript_final"); event.Text != "A confirmed question." {
		t.Fatal(event)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if got := <-model.input; got != "A confirmed question." {
		t.Fatal("An interim recognition guess reached the model", got)
	}
}
