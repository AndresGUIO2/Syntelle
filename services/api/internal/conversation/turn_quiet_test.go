package conversation

import (
	"encoding/binary"
	"testing"
	"time"
)

func microphoneFrame(amplitude int16) []byte {
	b := make([]byte, 640)
	for i := 0; i < len(b); i += 2 {
		binary.LittleEndian.PutUint16(b[i:], uint16(amplitude))
	}
	return b
}

func TestQuietTurnUsesAlreadyObservedSilence(t *testing.T) {
	q := turnQuiet{}
	now := time.Unix(1, 0)
	for range 25 {
		now = now.Add(20 * time.Millisecond)
		q.audio(microphoneFrame(2000), now)
	}
	for range 35 {
		now = now.Add(20 * time.Millisecond)
		q.audio(microphoneFrame(0), now)
	}
	if delay := q.delay("What time is it?", now); delay > 500*time.Millisecond || delay < 250*time.Millisecond {
		t.Fatalf("quiet opening delay = %v", delay)
	}
	if delay := q.delay("I was thinking about", now); delay < utterancePause {
		t.Fatalf("incomplete thought released early: %v", delay)
	}
}

func TestQuietTurnProtectsContinuingPCMWithoutRecognition(t *testing.T) {
	q := turnQuiet{}
	now := time.Unix(1, 0)
	for range 250 {
		now = now.Add(20 * time.Millisecond)
		q.audio(microphoneFrame(2000), now)
		if q.seen && q.delay("A confirmed fragment.", now) < time.Second {
			t.Fatal("active audio allowed a quiet deadline")
		}
	}
	if !q.recentVoice(now) {
		t.Fatal("local voice did not protect the watchdog")
	}
	for range 60 {
		now = now.Add(20 * time.Millisecond)
		q.audio(microphoneFrame(0), now)
	}
	if q.recentVoice(now) {
		t.Fatal("silence kept the voice latch active")
	}
}

func TestQuietTurnConservativeFallbackAndNoise(t *testing.T) {
	q := turnQuiet{}
	now := time.Unix(1, 0)
	if q.delay("Ready.", now) != utterancePause {
		t.Fatal("missing audio shortened the deadline")
	}
	for range 100 {
		now = now.Add(20 * time.Millisecond)
		q.audio(microphoneFrame(50), now)
	}
	if q.seen {
		t.Fatal("low background noise classified as speech")
	}
	q.audio(microphoneFrame(3000), now.Add(20*time.Millisecond))
	if !q.recentVoice(now.Add(20 * time.Millisecond)) {
		t.Fatal("first candidate frame did not postpone the deadline")
	}
	q.audio(microphoneFrame(0), now.Add(40*time.Millisecond))
	if q.seen {
		t.Fatal("one click classified as sustained speech")
	}
}
