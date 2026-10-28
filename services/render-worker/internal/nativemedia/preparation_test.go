package nativemedia

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"syntelle/render-contract"
	"testing"
	"testing/synctest"
	"time"

	"syntelle/render-worker/internal/nativeanimation"
)

type preparationScene struct {
	prepare func(context.Context, SpeechState) (*nativeanimation.PreparedSpeech, error)
}

func (s preparationScene) PrepareSpeech(ctx context.Context, input SpeechState) (*nativeanimation.PreparedSpeech, error) {
	return s.prepare(ctx, input)
}
func (preparationScene) Render(context.Context, FrameInput, *image.RGBA) error { return nil }

func preparationSession(t *testing.T, scene preparationScene) *session {
	t.Helper()
	s := queuedSession()
	s.ctx, s.cancel = context.WithCancel(t.Context())
	s.preparationWake = make(chan struct{}, 1)
	s.scene = scene
	s.wg.Add(1)
	go s.preparationLoop()
	t.Cleanup(func() { s.cancel(); s.wg.Wait() })
	return s
}

func queuePreparation(t *testing.T, s *session, turn, text string, index int, fixture preparedSpeech) {
	t.Helper()
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_part", "turn_id": turn, "data": speechPart{Speech: fixture, Index: index, Text: text}})); err != nil {
		t.Fatal(err)
	}
}

func TestPreparationGatesAudioAndCompletionWithOwnedClip(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})
		started := make(chan SpeechState, 1)
		result := &nativeanimation.PreparedSpeech{}
		s := preparationSession(t, preparationScene{prepare: func(ctx context.Context, input SpeechState) (*nativeanimation.PreparedSpeech, error) {
			started <- input
			select {
			case <-gate:
				return result, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}})
		if err := s.conversation(conversationData(t, map[string]any{"type": "accepted", "turn_id": "turn_owned"})); err != nil {
			t.Fatal(err)
		}
		raw := conversationData(t, map[string]any{"type": "speech_part", "turn_id": "turn_owned", "data": speechPart{Speech: preparedFixture(), Text: "owned", Index: 0}})
		if err := s.conversation(raw); err != nil {
			t.Fatal(err)
		}
		// The control envelope can be recycled immediately after Dispatch returns.
		for i := range raw {
			raw[i] = 'x'
		}
		if err := s.conversation(conversationData(t, map[string]any{"type": "speech_complete", "turn_id": "turn_owned", "data": map[string]int{"parts": 1}})); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		input := <-started
		if input.Samples[123] != 123 || input.Text != "owned" || input.Visemes[0].ID != 1 {
			t.Fatal("preparation retained reused control-envelope storage")
		}
		pcm := make([]int16, audioSamples)
		for range 3 {
			block := s.nextAudioBlock(time.Now(), pcm)
			if len(block.events) != 0 || s.frameInput().Speech.Active || !s.frameInput().Speech.Waiting {
				t.Fatal("pending analysis started audio or ended a completed-but-unready turn")
			}
			for _, sample := range pcm {
				if sample != 0 {
					t.Fatal("unprepared PCM escaped the audio gate")
				}
			}
		}
		close(gate)
		synctest.Wait()
		block := s.nextAudioBlock(time.Now(), pcm)
		state := s.frameInput().Speech
		if pcm[123] != 123 || !state.Active || state.Position != 0 || state.Prepared != result || len(block.events) != 1 || block.events[0].kind != "playback_started" {
			t.Fatal("first PCM and prepared animation did not start from the same clip position")
		}
		block = s.nextAudioBlock(time.Now(), pcm)
		if len(block.events) != 1 || block.events[0].kind != "playback_ended" || s.queuedBytes != 0 {
			t.Fatal("completed prepared clip did not end and release its PCM reservation")
		}
	})
}

func TestPreparationDiscardsInterruptedResultAndKeepsReservation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})
		old, current := &nativeanimation.PreparedSpeech{}, &nativeanimation.PreparedSpeech{}
		calls := 0
		s := preparationSession(t, preparationScene{prepare: func(_ context.Context, _ SpeechState) (*nativeanimation.PreparedSpeech, error) {
			calls++
			if calls == 1 {
				// Deliberately return a late successful result after interruption.
				<-gate
				return old, nil
			}
			return current, nil
		}})
		if err := s.conversation(conversationData(t, map[string]any{"type": "accepted", "turn_id": "turn_old"})); err != nil {
			t.Fatal(err)
		}
		queuePreparation(t, s, "turn_old", "old", 0, preparedFixture())
		synctest.Wait()
		s.interrupt()
		if s.queuedBytes != 960 || len(s.queue) != 0 {
			t.Fatal("interruption lost the in-flight PCM reservation")
		}
		if err := s.conversation(conversationData(t, map[string]any{"type": "accepted", "turn_id": "turn_new"})); err != nil {
			t.Fatal(err)
		}
		fixture := preparedFixture()
		binary.LittleEndian.PutUint16(fixture.Audio[44+123*2:], 1234)
		queuePreparation(t, s, "turn_new", "new", 0, fixture)
		close(gate)
		synctest.Wait()
		pcm := make([]int16, audioSamples)
		s.nextAudioBlock(time.Now(), pcm)
		state := s.frameInput().Speech
		if calls != 2 || pcm[123] != 1234 || state.Prepared != current || state.Text != "new" || s.queuedBytes != 0 {
			t.Fatal("obsolete analysis was published or its reservation affected the new generation")
		}
	})
}

func TestPreparationBackpressureAndShutdownAreBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := preparationSession(t, preparationScene{prepare: func(ctx context.Context, _ SpeechState) (*nativeanimation.PreparedSpeech, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}})
		if err := s.conversation(conversationData(t, map[string]any{"type": "accepted", "turn_id": "turn_bounded"})); err != nil {
			t.Fatal(err)
		}
		for index := range maxQueuedParts {
			queuePreparation(t, s, "turn_bounded", "bounded", index, preparedFixture())
		}
		synctest.Wait()
		extra := conversationData(t, map[string]any{"type": "speech_part", "turn_id": "turn_bounded", "data": speechPart{Speech: preparedFixture(), Index: maxQueuedParts}})
		if err := s.conversation(extra); err == nil || len(s.queue) != maxQueuedParts {
			t.Fatal("preparing clips bypassed the bounded part queue")
		}
		s.cancel()
		synctest.Wait()
		s.wg.Wait()
	})
}

func TestPreparationCapacityIncludesPlayingPCM(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		queuedParts int
		pcmBytes    int
	}{
		{name: "playing part counts toward part limit", queuedParts: maxQueuedParts - 1},
		{name: "in-flight bytes count toward PCM limit", pcmBytes: maxQueuedPCMBytes - 959},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := queuedSession()
			s.turn = "turn_capacity"
			s.playing = &clip{samples: []int16{1}}
			s.queue = make([]*clip, tc.queuedParts)
			s.nextIndex = tc.queuedParts
			s.queuedBytes = tc.pcmBytes
			raw := conversationData(t, map[string]any{"type": "speech_part", "turn_id": s.turn, "data": speechPart{Speech: preparedFixture(), Index: s.nextIndex}})
			if err := s.conversation(raw); err == nil {
				t.Fatal("reserved playing/preparing capacity was bypassed")
			}
		})
	}
}

func TestPreparationLanePreservesLaughterBinding(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		captured := make(chan SpeechState, 1)
		s := preparationSession(t, preparationScene{prepare: func(_ context.Context, input SpeechState) (*nativeanimation.PreparedSpeech, error) {
			captured <- input
			return &nativeanimation.PreparedSpeech{}, nil
		}})
		fixture := preparedFixture()
		sum := sha256.Sum256(fixture.Audio[44:])
		fixture.LaughterSpan = &rendercontract.LaughterSpan{Version: "laughter-span/1", Count: 480, PCMHash: fmt.Sprintf("%x", sum), RecordingHash: fmt.Sprintf("%x", sum)}
		if err := s.conversation(conversationData(t, map[string]any{"type": "accepted", "turn_id": "turn_bound"})); err != nil {
			t.Fatal(err)
		}
		queuePreparation(t, s, "turn_bound", "original", 0, fixture)
		synctest.Wait()
		input := <-captured
		if input.LaughterSpan == nil || *input.LaughterSpan != *fixture.LaughterSpan {
			t.Fatal("actual preparation lane dropped original laughter span")
		}
	})
}
