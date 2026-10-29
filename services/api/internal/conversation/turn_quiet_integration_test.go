package conversation

import (
	"context"
	"testing"
	"time"
)

type quietRecognitionFixture struct{ events chan RecognitionEvent }

func (r quietRecognitionFixture) Run(ctx context.Context, _ string, pcm <-chan []byte, emit func(RecognitionEvent)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-pcm:
			if !ok {
				return nil
			}
		case e := <-r.events:
			emit(e)
		}
	}
}
func quietFrames(f socketFixture, cid, mic string, amplitude int16, n int) {
	for range n {
		f.send(Event{Type: "audio", ConversationID: cid, TurnID: mic, Audio: microphoneFrame(amplitude)})
		time.Sleep(20 * time.Millisecond)
	}
}

func TestQuietMicrophoneFinalStartsWithoutSecondLongSilence(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 1)}
	r := quietRecognitionFixture{make(chan RecognitionEvent, 8)}
	cid, mic := "quiet-latency-session", "quiet-latency-microphone"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: mic})
	readUtteranceEvent(t, f, "listening")
	quietFrames(f, cid, mic, 2000, 25)
	quietFrames(f, cid, mic, 0, 35)
	start := time.Now()
	r.events <- RecognitionEvent{Type: "final", ID: "quiet-result-01", Text: "What time is it?", DurationMS: 500}
	final := readUtteranceEvent(t, f, "transcript_final")
	elapsed := time.Since(start)
	t.Logf("confirmed final to turn: %v", elapsed)
	if elapsed > 900*time.Millisecond || elapsed < 200*time.Millisecond {
		t.Fatalf("confirmed quiet delay: %v", elapsed)
	}
	if final.Text != "What time is it?" {
		t.Fatal(final)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if model.calls.Load() != 1 {
		t.Fatal("opening was not submitted once")
	}
}

func TestQuietMicrophoneContinuationOutlivesWatchdogWithoutSDKCallbacks(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 1)}
	r := quietRecognitionFixture{make(chan RecognitionEvent, 8)}
	cid, mic := "quiet-continuation-session", "quiet-continuation-microphone"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: mic})
	readUtteranceEvent(t, f, "listening")
	quietFrames(f, cid, mic, 2000, 25)
	quietFrames(f, cid, mic, 0, 35)
	r.events <- RecognitionEvent{Type: "final", ID: "quiet-fragment-01", Text: "What did you do?", DurationMS: 500}
	readUtteranceEvent(t, f, "transcript_partial")
	// The microphone continues, but the provider sends no start/interim event.
	quietFrames(f, cid, mic, 2000, 220)
	if model.calls.Load() != 0 {
		t.Fatal("ongoing microphone speech was cut by a deadline")
	}
	quietFrames(f, cid, mic, 0, 60)
	if model.calls.Load() != 0 {
		t.Fatal("unconfirmed acoustic continuation was lost")
	}
	r.events <- RecognitionEvent{Type: "final", ID: "quiet-fragment-02", Text: "In your daily routine.", OffsetMS: 1200, DurationMS: 4400}
	final := readUtteranceEvent(t, f, "transcript_final")
	if final.Text != "What did you do? In your daily routine." {
		t.Fatal(final)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if model.calls.Load() != 1 {
		t.Fatal("continuation created duplicate turns")
	}
}
