package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func TestVariedPresenceKeepsBrowAccentsExplicit(t *testing.T) {
	for _, version := range []string{"conversational-presence/3", "conversational-presence/4"} {
		for _, kind := range []string{"emphasis", "brows"} {
			t.Run(version+"-"+kind, func(t *testing.T) {
				c, s, settings := conversationalPresenceFixture()
				c.PresenceVersion = version
				c.Expressions = true
				amount := .6
				s.Speech.Plan.Performance = &Performance{Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "explain"}, Resolved: []PerformanceCue{{Kind: kind, Time: .4, Duration: .72, Amount: &amount, Source: "acoustic"}}}
				e := NewEngine(c)
				e.Sample(0, SpeechState{}, settings)
				s.Position = .6016
				f := e.Sample(601.6, s, settings)
				if version == "conversational-presence/4" && kind == "emphasis" {
					if f.Brows != 0 {
						t.Fatal("automatic head beat still raises brows", f.Brows)
					}
				} else if f.Brows < .2 {
					t.Fatal("explicit or legacy accent lost", f.Brows)
				}
			})
		}
	}
}

func TestVariedPresenceUsesAcousticAccentAsPostureRatherThanNod(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	c.PresenceVersion = "conversational-presence/4"
	amount := .8
	s.Speech.Plan.Performance = &Performance{Intent: &PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion, Purpose: "explain"}, Resolved: []PerformanceCue{{Kind: "emphasis", Source: "acoustic", Time: .4, Duration: .72, Amount: &amount}}}
	s.Speech.Plan.Gestures = []Gesture{{Time: .4, Duration: .72, Pitch: -.7, Kind: "emphasis"}}
	s.Speech.Plan.Cues = []Cue{{Time: .4, Kind: "emphasis", Strength: .8}}
	p := presenceDirector{}
	for i := 0; i < 40; i++ {
		s.Position = float64(i) / 30
		p.sample(s.Position*1000, s, c, settings)
		if p.offset != (Head{}) {
			t.Fatal("acoustic cue still injects return-to-center pitch beat", p.offset)
		}
	}
	if p.adjustments == 0 {
		t.Fatal("acoustic emphasis lost posture opportunity")
	}
}

func TestVariedPresenceHasVisiblePostureTravelWithoutContinuousMotion(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle", true: "speaking"}[active], func(t *testing.T) {
			travel := [2]float64{}
			moving := [2]int{}
			for v, version := range []string{"conversational-presence/3", "conversational-presence/4"} {
				c, s, settings := conversationalPresenceFixture()
				c.PresenceVersion = version
				s.Active = active
				p := presenceDirector{}
				var last Head
				for i := 0; i < 900; i++ {
					s.Position = float64(i) / 30
					h := p.sample(s.Position*1000, s, c, settings)
					d := headDistance(h, last)
					travel[v] += d
					if d > .002 {
						moving[v]++
					}
					if math.Abs(h.Yaw) > c.Bounds.Yaw || math.Abs(h.Pitch) > c.Bounds.Pitch || math.Abs(h.Roll) > c.Bounds.Roll || d > .5 {
						t.Fatal("outside safe envelope or abrupt", h, last)
					}
					last = h
				}
			}
			t.Logf("posture travel before/after %.3f/%.3f degrees; moving frames %d/%d", travel[0], travel[1], moving[0], moving[1])
			if travel[1] < travel[0]*1.5 || moving[1] < 250 || moving[1] > 650 {
				t.Fatal("candidate remains too small/static or moves continuously", travel, moving)
			}
		})
	}
}

func TestVariedPresencePreservesSemanticTimingAndUnderflow(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	c.PresenceVersion = VariedPresenceVersion
	g := Gesture{Time: .2, Duration: .42, Pitch: -1.2, Kind: "nod"}
	s.Speech.Plan.Gestures = []Gesture{g}
	p := presenceDirector{}
	for i := 0; i < 20; i++ {
		s.Position = float64(i) / 30
		h := p.sample(s.Position*1000, s, c, settings)
		want := naturalGestureAt(g, (s.Position-g.Time)/g.Duration)
		if headDistance(h, want) > 1e-9 {
			t.Fatal("semantic nod retimed", s.Position, h, want)
		}
	}
	for i := 20; i < 55; i++ {
		s.Position = float64(i) / 30
		p.sample(s.Position*1000, s, c, settings)
	}
	before := p.value
	s.Waiting = true
	s.Active = false
	if got := p.sample(8000, s, c, settings); got != before {
		t.Fatal("underflow advanced presence", before, got)
	}
	s.Waiting = false
	s.Active = true
	s.Session++
	s.Position = 0
	p.duration = p.position
	if got := p.sample(8000, s, c, settings); headDistance(got, before) > 1e-9 {
		t.Fatal("phrase boundary snapped", before, got)
	}
}

func TestVariedPresencePreservesFixedHeadArticulationAndAccessibility(t *testing.T) {
	for _, version := range []string{PresenceVersion, VariedPresenceVersion} {
		c, s, settings := conversationalPresenceFixture()
		c.PresenceVersion = version
		s.Speech.Timeline = []TimelineEvent{{0, "A"}, {.3, "MBP"}, {.5, "FV"}, {.8, "O"}}
		settings.Movement = false
		e := NewEngine(c)
		for i := 0; i < 25; i++ {
			s.Position = float64(i) / 30
			f := e.Sample(s.Position*1000, s, settings)
			if f.Head != (Head{}) {
				t.Fatal("movement disable ignored")
			}
			want := MouthAt(s.Speech.Timeline, s.Position)
			if want.From != want.To && f.Mouth != want {
				t.Fatal("articulation retimed", version, s.Position, f.Mouth, want)
			}
		}
		settings.Movement = true
		settings.ReducedMotion = true
		if f := e.Sample(2000, s, settings); f.Head != (Head{}) {
			t.Fatal("reduced motion ignored")
		}
		settings.ReducedMotion = false
		e.Sample(3000, s, settings)
		settings.Paused = true
		before := e.last
		if f := e.Sample(9000, s, settings); f != before {
			t.Fatal("pause changed presentation")
		}
	}
}
