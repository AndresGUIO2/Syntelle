package nativeanimation

import (
	"math"
	"testing"
)

func TestExpressivePosturePreservesEnvelopeAndHistoricalProfile(t *testing.T) {
	_, c, _ := loadReference(t)
	previous, expressive := NewEngine(c), NewEngine(func() Config { c.Expressive = true; return c }())
	different := false
	for ms := 0.; ms < 60000; ms += 100 {
		old := previous.Sample(ms, SpeechState{}, DefaultSettings())
		frame := expressive.Sample(ms, SpeechState{}, DefaultSettings())
		if math.Abs(frame.Head.Yaw) > c.Bounds.Yaw+1e-9 || math.Abs(frame.Head.Pitch) > c.Bounds.Pitch+1e-9 || math.Abs(frame.Head.Roll) > c.Bounds.Roll+1e-9 {
			t.Fatalf("outside prepared envelope: %+v", frame.Head)
		}
		different = different || math.Abs(old.Head.Yaw-frame.Head.Yaw) > .2
	}
	if !different {
		t.Fatal("expressive posture did not visibly change trajectory")
	}
}

func TestExpressiveSpeakingRetainsMorePosture(t *testing.T) {
	old, newer := newSpeaking(42), newSpeaking(42)
	idle := attention{Head: Head{Yaw: 2}}
	old.sample(0, true, idle, 42, false)
	newer.sample(0, true, idle, 42, true)
	a := old.sample(500, true, idle, 42, false)
	b := newer.sample(500, true, idle, 42, true)
	if b.Head.Yaw <= a.Head.Yaw {
		t.Fatal("speaking posture still suppressed")
	}
}
