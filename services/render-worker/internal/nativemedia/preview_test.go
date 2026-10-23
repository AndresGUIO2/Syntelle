package nativemedia

import "testing"

func TestNodPreviewForwardsCountAndRejectsInvalidCounts(t *testing.T) {
	s := queuedSession()
	if err := s.preview(conversationData(t, map[string]any{"kind": "nod", "repetitions": 7, "amount": .8})); err != nil {
		t.Fatal(err)
	}
	if got := s.frameInput(); got.PreviewRepetitions != 7 || got.PreviewAmount != .8 {
		t.Fatal(got)
	}
	for _, n := range []int{0, 9, -1} {
		if s.preview(conversationData(t, map[string]any{"kind": "nod", "repetitions": n})) == nil {
			t.Fatal("invalid count", n)
		}
	}
	s.interrupt()
	if s.frameInput().PreviewKind != "" {
		t.Fatal("interruption retained nod")
	}
}

func TestLaughterPreviewQueuesCachedPCMOnTheSpeechClock(t *testing.T) {
	s := queuedSession()
	audio := preparedFixture()
	audio.Preview = true
	if err := s.preview(conversationData(t, map[string]any{"kind": "laugh", "turn_id": "preview-fixture", "speech": audio})); err != nil {
		t.Fatal(err)
	}
	if len(s.queue) != 1 || !s.queue[0].preview || !s.complete || s.previewKind != "" || s.turn != "preview-fixture" {
		t.Fatal("laughter preview did not enter the audio lane")
	}
	if !s.clipState(s.queue[0], 0).Preview {
		t.Fatal("visual renderer lost preview identity")
	}
	s.interrupt()
	if len(s.queue) != 0 || s.turn != "" {
		t.Fatal("interruption retained preview audio")
	}
}

func TestPreviewDoesNotCreateSpeechAndInterruptionClearsIt(t *testing.T) {
	s := queuedSession()
	if err := s.preview(conversationData(t, map[string]string{"kind": "laugh"})); err != nil {
		t.Fatal(err)
	}
	f := s.frameInput()
	if f.PreviewKind != "laugh" || f.PreviewSequence != 1 || f.Speech.Active || f.Speech.Waiting || len(s.queue) != 0 {
		t.Fatal("preview became speech")
	}
	s.interrupt()
	if s.frameInput().PreviewKind != "" {
		t.Fatal("interruption retained preview")
	}
	s.presentation.Active = true
	if s.preview(conversationData(t, map[string]string{"kind": "smile"})) == nil {
		t.Fatal("preview displaced active speech")
	}
}
