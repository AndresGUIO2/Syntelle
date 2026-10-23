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

func TestConfirmedFollowUpSurvivesUnresolvedInterimRecognition(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 2)}
	r := controlledRecognitionFixture{events: make(chan RecognitionEvent, 8)}
	cid := "followup-session-01"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: "followup-microphone-01"})
	readUtteranceEvent(t, f, "listening")
	r.events <- RecognitionEvent{Type: "final", ID: "followup-first-01", Text: "Hi, how are you?"}
	readUtteranceEvent(t, f, "transcript_final")
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if got := <-model.input; got != "Hi, how are you?" {
		t.Fatal("first turn was lost", got)
	}
	f.send(Event{Type: "playback_ended", ConversationID: cid, TurnID: "followup-first-01"})
	r.events <- RecognitionEvent{Type: "final", ID: "followup-second-01", Text: "Can you tell me a story?"}
	if event := readUtteranceEvent(t, f, "transcript_partial"); event.Text != "Can you tell me a story?" {
		t.Fatal(event)
	}
	// A subsequent start/interim event can lack both a final and a no-match
	// callback. It must not hold the already confirmed second turn forever.
	r.events <- RecognitionEvent{Type: "start"}
	readUtteranceEvent(t, f, "speech_detected")
	r.events <- RecognitionEvent{Type: "partial", Text: "unconfirmed background speech"}
	readUtteranceEvent(t, f, "transcript_partial")
	final := readUtteranceEvent(t, f, "transcript_final")
	if final.TurnID != "followup-second-01" || final.Text != "Can you tell me a story?" {
		t.Fatal("confirmed follow-up was not released", final)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if got := <-model.input; got != "Can you tell me a story?" || model.calls.Load() != 2 {
		t.Fatal("follow-up did not reach the model exactly once", got, model.calls.Load())
	}
}

func TestFollowUpAfterPlaybackIsNotDiscardedByEarlierAudioWindow(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 2)}
	r := controlledRecognitionFixture{events: make(chan RecognitionEvent, 4)}
	cid := "playback-followup-session-01"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "submit", ConversationID: cid, TurnID: "playback-first-turn-01", Text: "Say hello."})
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	<-model.input
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: "playback-followup-mic-01"})
	readUtteranceEvent(t, f, "listening")
	f.send(Event{Type: "playback_started", ConversationID: cid, TurnID: "playback-first-turn-01"})
	for range 5 {
		f.send(Event{Type: "audio", ConversationID: cid, TurnID: "playback-followup-mic-01", Audio: make([]byte, 6400)})
	}
	f.send(Event{Type: "playback_ended", ConversationID: cid, TurnID: "playback-first-turn-01"})
	// Let the ordered socket commands close the playback window before the
	// independent recognition callback delivers its delayed final result.
	time.Sleep(50 * time.Millisecond)
	r.events <- RecognitionEvent{Type: "final", ID: "playback-followup-result-01", Text: "Please explain something else.", OffsetMS: 400, DurationMS: 800}
	if event := readUtteranceEvent(t, f, "transcript_final"); event.Text != "Please explain something else." {
		t.Fatal("confirmed follow-up was discarded by previous playback", event)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if got := <-model.input; got != "Please explain something else." {
		t.Fatal("follow-up did not reach the model", got)
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
