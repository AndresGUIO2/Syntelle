package nativemedia

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"syntelle/render-contract"
)

func conversationData(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func queuedSession() *session {
	return &session{settings: rendercontract.Defaults(), ignored: map[string]bool{}}
}
func preparedFixture() preparedSpeech {
	return preparedSpeech{Audio: testWAV(480), DurationMS: 20, Visemes: []Viseme{{ID: 1}, {ID: 0, OffsetMS: 20}}}
}

func TestSessionConversationOrdersDeduplicatesAndInterrupts(t *testing.T) {
	t.Parallel()
	s := queuedSession()
	accepted := conversationData(t, map[string]any{"type": "accepted", "turn_id": "turn_fixture"})
	if err := s.conversation(accepted); err != nil {
		t.Fatal(err)
	}
	state := s.frameInput()
	if !state.Speech.Waiting || state.Speech.TurnID != "turn_fixture" {
		t.Fatal("accepted turn did not enter waiting state")
	}
	part := conversationData(t, map[string]any{"type": "speech_part", "turn_id": "turn_fixture", "data": speechPart{Speech: preparedFixture(), Index: 0, Text: "original"}})
	if err := s.conversation(part); err != nil {
		t.Fatal(err)
	}
	if err := s.conversation(part); err != nil {
		t.Fatal(err)
	}
	if len(s.queue) != 1 {
		t.Fatal("duplicate delivery repeated the phrase")
	}
	outOfOrder := conversationData(t, map[string]any{"type": "speech_part", "turn_id": "turn_fixture", "data": speechPart{Speech: preparedFixture(), Index: 2}})
	if err := s.conversation(outOfOrder); err == nil {
		t.Fatal("out-of-order phrase accepted")
	}
	oldGeneration := s.frameInput().Generation
	s.interrupt()
	if len(s.queue) != 0 || s.frameInput().Generation <= oldGeneration || s.frameInput().Speech.Active {
		t.Fatal("interruption did not invalidate the old turn")
	}
	if err := s.conversation(part); err != nil {
		t.Fatal(err)
	}
	if err := s.conversation(accepted); err != nil {
		t.Fatal(err)
	}
	if len(s.queue) != 0 || s.frameInput().Speech.TurnID != "" {
		t.Fatal("late interrupted turn resumed")
	}
}

func TestSessionCompletionMatchesReceivedParts(t *testing.T) {
	t.Parallel()
	s := queuedSession()
	if err := s.conversation(conversationData(t, map[string]any{"type": "accepted", "turn_id": "turn_fixture"})); err != nil {
		t.Fatal(err)
	}
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_part", "turn_id": "turn_fixture", "data": speechPart{Speech: preparedFixture(), Index: 0}})); err != nil {
		t.Fatal(err)
	}
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_complete", "turn_id": "turn_fixture", "data": map[string]int{"parts": 2}})); err == nil {
		t.Fatal("incomplete turn marked complete")
	}
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_complete", "turn_id": "turn_fixture", "data": map[string]int{"parts": 1}})); err != nil {
		t.Fatal(err)
	}
	if !s.complete {
		t.Fatal("complete turn remained open")
	}
}

func TestSessionFrameUsesCurrentMediaBlockAndImmutablePCM(t *testing.T) {
	t.Parallel()
	s := queuedSession()
	samples := []int16{100, 200}
	s.mediaTime = time.Second
	s.lastTick = time.Now()
	s.presentation = SpeechState{Active: true, Position: 200 * time.Millisecond, Duration: time.Second, Samples: samples, Visemes: []Viseme{{ID: 1}}, SampleRate: sampleRate}
	frame := s.frameInput()
	if frame.MediaTime < time.Second || frame.MediaTime > 1020*time.Millisecond || frame.Speech.Position < 200*time.Millisecond || frame.Speech.Position > 220*time.Millisecond {
		t.Fatal("frame was not sampled on the media clock")
	}
	if len(frame.Speech.Samples) != 2 || frame.Speech.Samples[0] != 100 {
		t.Fatal("source PCM was changed")
	}
}

func TestSessionConcurrentStateAndInterruption(t *testing.T) {
	s := queuedSession()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s.interrupt()
				s.frameInput()
			}
		}()
	}
	wg.Wait()
	if s.frameInput().Generation != 300 {
		t.Fatal("interruption generation was lost")
	}
}

func TestSessionAudioPreservesAdjacentPCMAndPause(t *testing.T) {
	t.Parallel()
	s := queuedSession()
	s.turn = "turn_fixture"
	s.complete = true
	first := &clip{samples: make([]int16, 200), duration: time.Second * 200 / sampleRate}
	second := &clip{samples: make([]int16, 400), duration: time.Second * 400 / sampleRate}
	for i := range first.samples {
		first.samples[i] = 100
	}
	for i := range second.samples {
		second.samples[i] = 200
	}
	s.queue = []*clip{first, second}
	s.queuedBytes = 1200
	pcm := make([]int16, audioSamples)
	s.nextAudioBlock(time.Now(), pcm)
	if pcm[199] != 100 || pcm[200] != 200 || pcm[479] != 200 {
		t.Fatal("phrase boundary inserted silence or changed source samples")
	}
	if s.position != 280 {
		t.Fatal("source sample cursor did not cross the phrase boundary")
	}
	if err := s.configure([]byte(`{"paused":true}`)); err != nil {
		t.Fatal(err)
	}
	clear(pcm)
	s.nextAudioBlock(time.Now(), pcm)
	if s.position != 280 {
		t.Fatal("pause advanced source PCM")
	}
	for _, sample := range pcm {
		if sample != 0 {
			t.Fatal("pause sent speech instead of silence")
		}
	}
	position := s.frameInput().Speech.Position
	if position != time.Second*280/sampleRate {
		t.Fatal("paused visual speech clock advanced")
	}
	if err := s.configure([]byte(`{"paused":false}`)); err != nil {
		t.Fatal(err)
	}
	clear(pcm)
	s.nextAudioBlock(time.Now(), pcm)
	if pcm[119] != 200 || pcm[120] != 0 {
		t.Fatal("only the final codec tail should be silence padded")
	}
}

func TestSessionConfigurePauseFreezesAcknowledgedEmittedCursor(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name                      string
		firstLength, secondLength int
		wantPosition              time.Duration
		wantText                  string
	}{
		{name: "inside current phrase", firstLength: 4800, wantPosition: 20 * time.Millisecond, wantText: "first"},
		{name: "completed phrase boundary", firstLength: 480, secondLength: 480, wantPosition: 20 * time.Millisecond, wantText: "first"},
		{name: "crossed phrase boundary", firstLength: 200, secondLength: 4800, wantPosition: time.Second * 280 / sampleRate, wantText: "second"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := queuedSession()
			s.turn = "turn_fixture"
			s.complete = true
			first := &clip{samples: make([]int16, tt.firstLength), duration: time.Duration(tt.firstLength) * time.Second / sampleRate, text: "first"}
			s.queue = []*clip{first}
			s.queuedBytes = len(first.samples) * 2
			if tt.secondLength > 0 {
				second := &clip{samples: make([]int16, tt.secondLength), duration: time.Duration(tt.secondLength) * time.Second / sampleRate, text: "second"}
				s.queue = append(s.queue, second)
				s.queuedBytes += len(second.samples) * 2
			}
			pcm := make([]int16, audioSamples)
			s.nextAudioBlock(time.Now(), pcm)
			if err := s.configure([]byte(`{"paused":true}`)); err != nil {
				t.Fatal(err)
			}
			before := s.frameInput().Speech
			if before.Position != tt.wantPosition || before.Text != tt.wantText {
				t.Fatalf("pause acknowledgement used stale block start or phrase: %+v", before)
			}
			for range 11 {
				s.nextAudioBlock(time.Now(), pcm)
			}
			after := s.frameInput().Speech
			if after.Position != before.Position || after.Text != before.Text || after.Duration != before.Duration || after.Active != before.Active {
				t.Fatalf("paused presentation changed across 220 ms of silence: before=%+v after=%+v", before, after)
			}
		})
	}
}
