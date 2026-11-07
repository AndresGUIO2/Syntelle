//go:build integration

package conversation

import (
	"fmt"
	"testing"
	"time"
)

func fastMicrophoneFrames(f socketFixture, cid, mic string, amplitude, dc int16, count int) {
	for range count {
		f.send(Event{Type: "audio", ConversationID: cid, TurnID: mic, Audio: fastPCM(amplitude, dc)})
		time.Sleep(20 * time.Millisecond)
	}
}

func TestFastSocketQuietFinalSurvivesContinuedNoisePackets(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 1)}
	r := quietRecognitionFixture{make(chan RecognitionEvent, 8)}
	cid, mic := "fast-quiet-session", "fast-quiet-microphone"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: mic})
	readUtteranceEvent(t, f, "listening")
	fastMicrophoneFrames(f, cid, mic, 3500, 0, 25)
	fastMicrophoneFrames(f, cid, mic, 0, 0, 35)
	started := time.Now()
	r.events <- RecognitionEvent{Type: "final", ID: "fast-quiet-result", Text: "Can you tell me a story?", DurationMS: 500}
	readUtteranceEvent(t, f, "transcript_partial")
	var submitted time.Duration
	// Continuous packets containing DC and stationary microphone noise must
	// not turn the final guard into a perpetually refreshed silence deadline.
	for range 30 {
		fastMicrophoneFrames(f, cid, mic, 200, 9000, 1)
		if submitted == 0 && model.calls.Load() > 0 {
			submitted = time.Since(started)
		}
	}
	if submitted == 0 || submitted > 500*time.Millisecond {
		t.Fatalf("confirmed quiet was held by noise packets: %v", submitted)
	}
	final := readUtteranceEvent(t, f, "transcript_final")
	if final.Text != "Can you tell me a story?" || model.calls.Load() != 1 {
		t.Fatal("confirmed final was lost or duplicated", final)
	}
	var finalToCommitMS float64
	err := store.Pool.QueryRow(f.ctx, `SELECT EXTRACT(EPOCH FROM (t.created_at-r.created_at))*1000
		FROM syntelle_conversation.turns t JOIN syntelle_conversation.recognition_results r ON r.result_id=t.id
		WHERE t.conversation_id=$1 AND t.id=$2`, cid, final.TurnID).Scan(&finalToCommitMS)
	if err != nil || finalToCommitMS < 0 || finalToCommitMS > 250 {
		t.Fatalf("normal confirmed-final to committed turn: %.1f ms, error %v", finalToCommitMS, err)
	}
	t.Logf("confirmed-final to committed turn %.1f ms; SDK fixture callback to model %v", finalToCommitMS, submitted)
}

func TestFastSocketAdvancingSDKProtectsLongContinuation(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 1)}
	r := quietRecognitionFixture{make(chan RecognitionEvent, 32)}
	cid, mic := "fast-continuation-session", "fast-continuation-microphone"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: mic})
	readUtteranceEvent(t, f, "listening")
	fastMicrophoneFrames(f, cid, mic, 3500, 0, 25)
	fastMicrophoneFrames(f, cid, mic, 0, 0, 35)
	r.events <- RecognitionEvent{Type: "final", ID: "fast-fragment-one", Text: "What did you do?", DurationMS: 500}
	readUtteranceEvent(t, f, "transcript_partial")
	r.events <- RecognitionEvent{Type: "start", OffsetMS: 1200}
	readUtteranceEvent(t, f, "speech_detected")
	for i := range 22 {
		r.events <- RecognitionEvent{Type: "partial", Text: fmt.Sprintf("In your daily routine %d", i), OffsetMS: 1200, DurationMS: float64((i + 1) * 200)}
		fastMicrophoneFrames(f, cid, mic, 3500, 0, 10)
		if model.calls.Load() != 0 {
			t.Fatal("a genuinely advancing 4.4-second continuation was cut", i)
		}
	}
	r.events <- RecognitionEvent{Type: "final", ID: "fast-fragment-two", Text: "In your daily routine.", OffsetMS: 1200, DurationMS: 4400}
	final := readUtteranceEvent(t, f, "transcript_final")
	if final.Text != "What did you do? In your daily routine." {
		t.Fatal("SDK-confirmed continuation changed", final)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if model.calls.Load() != 1 {
		t.Fatal("advancing continuation created duplicate turns")
	}
}

func TestFastSocketPCMTimeoutRetainsLateConfirmedWordsBeforePlayback(t *testing.T) {
	store := isolatedStore(t)
	model := &inputModelFixture{input: make(chan string, 2)}
	r := quietRecognitionFixture{make(chan RecognitionEvent, 8)}
	cid, mic := "fast-pcm-session", "fast-pcm-microphone"
	f := openSocketFixture(t, New(store, model, &speechFixture{}, r), cid)
	f.send(Event{Type: "mic_start", ConversationID: cid, TurnID: mic})
	readUtteranceEvent(t, f, "listening")
	fastMicrophoneFrames(f, cid, mic, 3500, 0, 25)
	fastMicrophoneFrames(f, cid, mic, 0, 0, 35)
	r.events <- RecognitionEvent{Type: "final", ID: "fast-pcm-fragment", Text: "A confirmed fragment.", DurationMS: 500}
	readUtteranceEvent(t, f, "transcript_partial")
	started := time.Now()
	var submitted time.Duration
	for range 50 {
		fastMicrophoneFrames(f, cid, mic, 3500, 0, 1)
		if submitted == 0 && model.calls.Load() > 0 {
			submitted = time.Since(started)
		}
	}
	if submitted < 650*time.Millisecond || submitted > time.Second {
		t.Fatal("PCM-only continuation did not use its bounded lease", submitted)
	}
	readUtteranceEvent(t, f, "transcript_final")
	readUtteranceEvent(t, f, "accepted")
	prepared := readUtteranceEvent(t, f, "prepared_speech")
	f.send(Event{Type: "playback_started", ConversationID: cid, TurnID: prepared.TurnID})
	// These words were captured before playback, inside the bounded unresolved
	// source interval. A delayed SDK final remains a separate eligible turn.
	r.events <- RecognitionEvent{Type: "final", ID: "fast-pcm-late-final", Text: "A late confirmed continuation.", OffsetMS: 1200, DurationMS: 650}
	final := readUtteranceEvent(t, f, "transcript_final")
	if final.Text != "A late confirmed continuation." {
		t.Fatal("late confirmed source words were discarded", final)
	}
	readUtteranceEvent(t, f, "accepted")
	readUtteranceEvent(t, f, "prepared_speech")
	if model.calls.Load() != 2 {
		t.Fatal("late final was not submitted exactly once")
	}
	t.Logf("PCM-only confirmed prefix released after %v; late source words preserved", submitted)
}
