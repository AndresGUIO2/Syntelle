package nativeadapter

import (
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativemedia"
	"testing"
	"time"
)

func TestProgressiveSpeechKeepsAnimationIdentityAndAudioPosition(t *testing.T) {
	s, clip := fixtureScene(t)
	clip.Streaming = true
	clip.TurnID = "progressive-fixture"
	clip.Position = 300 * time.Millisecond
	input := nativemedia.FrameInput{Settings: rendercontract.Defaults(), Speech: clip, MediaTime: time.Second}
	if _, err := s.sample(input); err != nil {
		t.Fatal(err)
	}
	session := s.animationSession
	// A new immutable analysis for this prefix must not restart blink, gesture,
	// attention or articulation clocks, or spend another performance allowance.
	next, err := s.PrepareSpeech(t.Context(), clip)
	if err != nil {
		t.Fatal(err)
	}
	input.Speech.Prepared = next
	input.Speech.Position = 320 * time.Millisecond
	input.MediaTime += 20 * time.Millisecond
	if _, err = s.sample(input); err != nil {
		t.Fatal(err)
	}
	if s.animationSession != session || s.clipPrepared != next || s.streamTurn != clip.TurnID {
		t.Fatal("prefix update restarted animation identity")
	}
	input.Speech.Streaming = false
	input.Speech.Prepared, err = s.PrepareSpeech(t.Context(), clip)
	if err != nil {
		t.Fatal(err)
	}
	input.MediaTime += 20 * time.Millisecond
	if _, err = s.sample(input); err != nil {
		t.Fatal(err)
	}
	if s.animationSession != session+1 || s.streamTurn != "" {
		t.Fatal("normal next phrase lost its original admission")
	}
}
