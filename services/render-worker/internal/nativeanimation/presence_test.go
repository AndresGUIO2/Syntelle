package nativeanimation

import (
	"encoding/json"
	"math"
	"testing"
)

func conversationalPresenceFixture() (Config, SpeechState, Settings) {
	c := Config{PresenceVersion: PresenceVersion, Conversational: true, ConversationalMotion: true, Seed: 17, Bounds: Bounds{Yaw: 3, Pitch: 4, Roll: 1.2}}
	s := SpeechState{Active: true, Session: 1, ReplyID: "reply", Speech: &Speech{Duration: 30, Plan: &Plan{natural: true}}}
	for i := range c.Eyes {
		c.Eyes[i] = GazeEye{Width: 100, Bounds: [2][2]float64{{5, 5}, {2, 2}}}
	}
	return c, s, DefaultSettings()
}

func headDistance(a, b Head) float64 {
	return math.Abs(a.Yaw-b.Yaw) + math.Abs(a.Pitch-b.Pitch) + math.Abs(a.Roll-b.Roll)
}

func TestPresenceWaitsForAcousticOpportunity(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	p := presenceDirector{}
	for i := 0; i < 24; i++ {
		s.Position = float64(i) / 30
		if h := p.sample(s.Position*1000, s, c, settings); h != (Head{}) {
			t.Fatal("timer invented a speaking movement", h)
		}
	}
	s.Speech.Plan.Cues = []Cue{{.8, "pause", .7}}
	var travel float64
	var last Head
	for i := 24; i < 54; i++ {
		s.Position = float64(i) / 30
		h := p.sample(s.Position*1000, s, c, settings)
		travel += headDistance(h, last)
		last = h
	}
	if travel < .1 || math.Abs(last.Yaw) > 1 || math.Abs(last.Pitch) > .5 || math.Abs(last.Roll) > .15 {
		t.Fatal("opportunity did not make one bounded visible adjustment", travel, last)
	}
}

func TestPresenceHoldsPostureDuringGestureWithoutRecentering(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	p := presenceDirector{}
	p.sample(0, s, c, settings)
	p.base = Head{Yaw: 1, Pitch: .5, Roll: .1}
	p.value = p.base
	s.Session++
	s.Speech.Plan.Gestures = []Gesture{{Time: .2, Duration: .42, Pitch: -1.2, Kind: "nod"}}
	for i := 0; i < 20; i++ {
		s.Position = float64(i) / 30
		h := p.sample(float64(i)*1000/30, s, c, settings)
		if math.Abs(h.Yaw-1) > 1e-9 || math.Abs(h.Roll-.1) > 1e-9 {
			t.Fatal("nod pulled unrelated axes toward center", h)
		}
		if h.Pitch < -.71 || h.Pitch > .51 {
			t.Fatal("nod left its posture-relative envelope", h)
		}
	}
}

func TestPresencePrefixAndUnderflowDoNotReplayOrAdvance(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	p := presenceDirector{}
	s.Speech.Plan.Cues = []Cue{{.4, "pause", .8}, {.7, "ending", .8}}
	for i := 0; i <= 18; i++ {
		s.Position = float64(i) / 30
		p.sample(float64(i)*1000/30, s, c, settings)
	}
	before := p.value
	for i := 19; i < 90; i++ {
		if got := p.sample(float64(i)*1000/30, s, c, settings); got != before {
			t.Fatal("underflow advanced speech-linked movement", before, got)
		}
	}
	// Immutable replacement changes complete analysis but not the source cursor.
	s.Speech = &Speech{Duration: 30, Plan: &Plan{natural: true, Cues: []Cue{{.2, "emphasis", .9}, {.4, "pause", .8}, {.7, "ending", .8}}}}
	p.sample(3000, s, c, settings)
	if p.adjustments != 1 || p.value != before {
		t.Fatal("prefix replacement replayed/revised an action", p.adjustments, p.value)
	}
	for i := 19; i < 45; i++ {
		s.Position = float64(i) / 30
		p.sample(3000+float64(i)*1000/30, s, c, settings)
	}
	if p.adjustments != 1 {
		t.Fatal("incomplete ending invented another movement", p.adjustments)
	}
}

func TestPresenceBoundsSpacingAndDeterminism(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "quiet"}[quiet], func(t *testing.T) {
			c, s, settings := conversationalPresenceFixture()
			if quiet {
				settings.EmotionControl = &EmotionSnapshot{EnergyTo: .2}
			}
			for at := .5; at < 60; at += .5 {
				s.Speech.Plan.Cues = append(s.Speech.Plan.Cues, Cue{at, "pause", .8})
			}
			a, b := presenceDirector{}, presenceDirector{}
			lastAction := -10000.
			for i := 0; i < 1800; i++ {
				s.Position = float64(i) / 30
				count := a.adjustments
				x := a.sample(s.Position*1000, s, c, settings)
				y := b.sample(s.Position*1000, s, c, settings)
				if x != y || math.Abs(x.Yaw) > c.Bounds.Yaw || math.Abs(x.Pitch) > c.Bounds.Pitch || math.Abs(x.Roll) > c.Bounds.Roll {
					t.Fatal("nonrepeatable or unsafe movement", x, y)
				}
				if count != a.adjustments {
					gap := 2000.
					if quiet {
						gap = 4000
					}
					if a.clock-lastAction < gap {
						t.Fatal("opportunities bypassed spacing", a.clock-lastAction)
					}
					lastAction = a.clock
				}
			}
		})
	}
}

func TestPresenceInterruptionAndPhraseBoundaryAreContinuous(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	s.Speech.Plan.Gestures = []Gesture{{Time: .1, Duration: .58, Yaw: 2.8, Kind: "shake"}}
	p := presenceDirector{}
	p.sample(0, s, c, settings)
	s.Position = .24
	before := p.sample(240, s, c, settings)
	s.Active = false
	if h := p.sample(240, s, c, settings); headDistance(h, before) > 1e-9 {
		t.Fatal("interruption jumped", h, before)
	}
	after := p.sample(600, s, c, settings)
	if headDistance(after, p.base) > 1e-9 {
		t.Fatal("interruption did not settle", after)
	}
	s.Active, s.Session, s.Position = true, 2, 0
	s.Speech.Plan.Gestures = nil
	if h := p.sample(600, s, c, settings); headDistance(h, after) > 1e-9 {
		t.Fatal("phrase onset jumped", h, after)
	}
}

func TestPresenceEnergyDoesNotRescaleCurrentPoseAndPauseFreezes(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	c.Emotions = true
	e := NewEngine(c)
	e.Sample(0, s, settings)
	e.attention.presence.base = Head{Yaw: 1, Pitch: .3}
	e.attention.presence.value = e.attention.presence.base
	s.Position = 1
	before := e.Sample(1000, s, settings)
	weights := [10]float64{}
	weights[2] = 1
	settings.EmotionControl = &EmotionSnapshot{To: weights, EnergyTo: 1}
	if h := e.Sample(1000, s, settings).Head; h != before.Head {
		t.Fatal("emotion rescaled held pose", h, before.Head)
	}
	settings.Paused = true
	before = e.last
	if got := e.Sample(30000, s, settings); got != before {
		t.Fatal("pause changed frame")
	}
	settings.Paused, settings.ReducedMotion = false, true
	if h := e.Sample(31000, s, settings).Head; h != (Head{}) {
		t.Fatal("reduced motion ignored", h)
	}
}

func TestPresenceKeepsIdleAndUninterruptedSpeechVisiblyActive(t *testing.T) {
	for _, speaking := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle", true: "speech without pauses"}[speaking], func(t *testing.T) {
			c, s, settings := conversationalPresenceFixture()
			s.Active = speaking
			p := presenceDirector{}
			var last Head
			moving, quiet, longest := 0, 0, 0
			for i := 0; i < 900; i++ {
				s.Position = float64(i) / 30
				h := p.sample(float64(i)*1000/30, s, c, settings)
				if headDistance(h, last) > .002 {
					moving++
					quiet = 0
				} else {
					quiet++
					longest = max(longest, quiet)
				}
				if headDistance(h, last) > .25 {
					t.Fatal("background jerk", h, last)
				}
				last = h
			}
			if moving < 110 || moving > 550 || longest > 130 {
				t.Fatal("presence is static or constantly moving", moving, longest)
			}
			t.Logf("visible steps=%d/900 longest quiet=%.3fs", moving, float64(longest)/30)
		})
	}
}

func TestPresenceFitsGesturesAndSettlesAnOverlappingAdjustment(t *testing.T) {
	c, s, settings := conversationalPresenceFixture()
	p := presenceDirector{}
	p.sample(0, s, c, settings)
	p.base = Head{Yaw: 2.9, Pitch: 3.7, Roll: 1.1}
	p.value = p.base
	s.Session = 2
	s.Speech.Plan.Gestures = []Gesture{{Time: 0, Duration: .58, Yaw: 2.85, Pitch: -3.8, Roll: 1.14, Kind: "shake"}}
	for i := 0; i < 18; i++ {
		s.Position = float64(i) / 30
		h := p.sample(float64(i)*1000/30, s, c, settings)
		if math.Abs(h.Yaw) > c.Bounds.Yaw || math.Abs(h.Pitch) > c.Bounds.Pitch || math.Abs(h.Roll) > c.Bounds.Roll {
			t.Fatal("gesture clipped outside geometry", h)
		}
	}
	p = presenceDirector{}
	s.Session = 1
	s.Position = 0
	s.Speech.Plan.Gestures = nil
	p.sample(0, s, c, settings)
	p.from, p.target, p.moveAt, p.moveDuration = Head{}, Head{Yaw: 1, Pitch: .5}, 0, 600
	s.Position = .3
	before := p.sample(300, s, c, settings)
	s.Session = 2
	s.Position = 0
	s.Speech.Duration = .3
	// The actual completed preceding phrase duration is300ms.
	p.duration = .3
	s.Speech.Plan.Gestures = []Gesture{{Time: 0, Duration: .42, Pitch: -1, Kind: "nod"}}
	after := p.sample(300, s, c, settings)
	if headDistance(before, after) > 1e-9 || !p.settling {
		t.Fatal("semantic entry snapped an active posture", before, after)
	}
}

func TestPresenceSelectorIsOptionalAndRejectsUnknownVersions(t *testing.T) {
	ref, _, _ := loadReference(t)
	var motion map[string]any
	if err := json.Unmarshal(ref.Bundle.Motion, &motion); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"", PresenceVersion, "unsupported"} {
		t.Run("selector-"+version, func(t *testing.T) {
			motion["conversational_motion"] = map[string]any{"version": "conversational-motion/1", "presence_version": version}
			raw, err := json.Marshal(motion)
			if err != nil {
				t.Fatal(err)
			}
			config, err := LoadBundle(raw, ref.Bundle.Gaze, ref.Bundle.Expressions, ref.Bundle.Oral)
			if version == "unsupported" {
				if err == nil {
					t.Fatal("unknown selector accepted")
				}
				return
			}
			if err != nil || config.PresenceVersion != version {
				t.Fatal("optional selector did not round trip", err, config.PresenceVersion)
			}
		})
	}
}
