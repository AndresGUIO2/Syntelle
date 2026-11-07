package nativeanimation

import (
	"math"
	"testing"
)

func TestContextualArticulationPreservesMelodiesNativeContext(t *testing.T) {
	s := &Speech{contextual: true, Duration: 1.2, NativeEvents: []NativeViseme{{21, 600}, {4, 640}, {14, 720}, {6, 800}, {19, 880}, {6, 960}, {6, 1000}, {15, 1080}}}
	for _, e := range s.NativeEvents {
		f := s.MouthAt(e.OffsetMS / 1000)
		if mouthPoseWeight(f, azurePoses[e.ID]) < .999 {
			t.Fatalf("late native target %v %+v", e, f)
		}
	}
	a, b := s.ResidualAt(.75, true), s.ResidualAt(.83, true)
	if a == b || a == ([2]float64{}) {
		t.Fatal("shared EI shape frozen", a, b)
	}
	for _, at := range []float64{.6, .62} {
		if s.ResidualAt(at, true) != ([2]float64{}) {
			t.Fatal("contact variation", at)
		}
	}
}
func TestContextualArticulationSustainedVowelDoesNotPump(t *testing.T) {
	s := &Speech{contextual: true, Duration: 1, NativeEvents: []NativeViseme{{6, 0}, {6, 200}, {6, 400}, {6, 600}}}
	a := s.ResidualAt(.25, true)
	if a != s.ResidualAt(.45, true) {
		t.Fatal("sustained vowel pumps")
	}
	if s.ResidualAt(.3, false) != ([2]float64{}) {
		t.Fatal("disabled variation")
	}
}
func TestExpressivePresenceSpeedAndActivity(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	c.PresenceVersion = ExpressivePresenceVersion
	p := presenceDirector{}
	var last Head
	travel := 0.
	for i := 0; i < 900; i++ {
		s.Position = float64(i) / 30
		h := p.sample(s.Position*1000, s, c, settings)
		for _, v := range [][3]float64{{h.Yaw, last.Yaw, 70}, {h.Pitch, last.Pitch, 45}, {h.Roll, last.Roll, 15}} {
			if math.Abs(v[0]-v[1])*30 > v[2]+.01 {
				t.Fatal("excess velocity", v)
			}
		}
		travel += headDistance(h, last)
		last = h
	}
	t.Log("30s angular travel", travel, "adjustments", p.adjustments)
	if travel < 30 || p.adjustments < 15 {
		t.Fatal("still too static", travel, p.adjustments)
	}
}
