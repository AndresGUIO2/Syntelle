package nativeanimation

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestConversationalBackgroundHasBriefAdjustmentsAndRealHolds(t *testing.T) {
	c := Config{Conversational: true, ConversationalMotion: true, Expressive: true, Seed: 17, Bounds: Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}}
	d := newAttention(c.Seed)
	last := Head{}
	moving, total, longest, run := 0, 0, 0, 0
	for ms := 0.; ms < 30000; ms += 1000. / 30 {
		h := d.sample(ms, SpeechState{Active: true, Session: 1}, c).Head
		if ms > 1000 {
			total++
			if math.Abs(h.Yaw-last.Yaw)+math.Abs(h.Pitch-last.Pitch)+math.Abs(h.Roll-last.Roll) > .003 {
				moving++
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
		last = h
	}
	if float64(moving)/float64(total) > .4 {
		t.Fatalf("background kept drifting rather than holding: %d/%d frames", moving, total)
	}
	if longest > 28 {
		t.Fatalf("background adjustment lasted %.2f seconds", float64(longest)/30)
	}
	if moving < 35 {
		t.Fatal("background froze instead of using occasional adjustments")
	}
	t.Logf("moving frames %d/%d (%.1f%%); longest adjustment %.3fs", moving, total, 100*float64(moving)/float64(total), float64(longest)/30)
}

func TestBriefPostureDoesNotSweepBetweenOppositeSideTargets(t *testing.T) {
	for _, seed := range []uint32{1, 17, 1234} {
		t.Run(string(rune('A'+seed%26)), func(t *testing.T) {
			p := briefPosture{}
			for ms := 0.; ms < 120000; ms += 1000. / 30 {
				h := p.sample(ms, true, seed, Bounds{Yaw: 8, Pitch: 4, Roll: 1.2})
				if math.Abs(h.Yaw) > 3.6+1e-9 || math.Abs(h.Pitch) > 1.7+1e-9 || math.Abs(h.Roll) > .45+1e-9 {
					t.Fatal("posture escaped its reserved envelope", h)
				}
				if p.from.Yaw*p.target.Yaw < 0 && math.Abs(p.from.Yaw) > .6 && math.Abs(p.target.Yaw) > .6 {
					t.Fatal("wide opposite-side background sweep", p.from, p.target)
				}
			}
		})
	}
}

func TestBriefPostureContinuesAcrossShortPhrases(t *testing.T) {
	c := Config{Conversational: true, ConversationalMotion: true, Seed: 17, Bounds: Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}}
	a, b := newAttention(c.Seed), newAttention(c.Seed)
	for ms := 0.; ms < 20000; ms += 1000. / 30 {
		x := a.sample(ms, SpeechState{Active: true, Session: 1}, c)
		y := b.sample(ms, SpeechState{Active: true, Session: uint64(ms/400) + 1}, c)
		if x != y {
			t.Fatal("phrase boundary restarted background posture", ms, x, y)
		}
	}
}

func TestSpeakingPresenceIsActiveButKeepsQuietDeliveryAndIdleDistinct(t *testing.T) {
	activity := func(speaking, quiet bool) (int, float64) {
		p := briefPosture{}
		moving := 0
		travel := 0.
		var previous Head
		for ms := 0.; ms < 30000; ms += 1000. / 30 {
			h := p.sample(ms, speaking, 17, Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}, quiet)
			if ms > 1000 {
				delta := math.Abs(h.Yaw-previous.Yaw) + math.Abs(h.Pitch-previous.Pitch) + math.Abs(h.Roll-previous.Roll)
				travel += delta
				if delta > .003 {
					moving++
				}
			}
			previous = h
		}
		return moving, travel
	}
	normal, normalTravel := activity(true, false)
	quiet, _ := activity(true, true)
	idle, idleTravel := activity(false, false)
	// More frequent small idle adjustments can occupy more sampled frames than
	// brief, stronger speech accents. Preserve their energy distinction through
	// total angular travel rather than treating frame activity as a quota.
	if normal <= quiet || normalTravel <= idleTravel || normal < 180 {
		t.Fatal("speaking presence did not distinguish active, quiet and idle", normal, quiet, idle, normalTravel, idleTravel)
	}
	t.Logf("moving steps active=%d quiet=%d idle=%d; angular travel active=%.6f idle=%.6f degrees", normal, quiet, idle, normalTravel, idleTravel)
}

func TestSpeakingPostureTraceRemainsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		quiet bool
		want  string
	}{
		{"normal", false, "b6a363b07fde90ed55d746b497702b02a3fdea34f2db946f36378a9ae31c19b2"},
		{"quiet", true, "14b69e6fdf86e4f3ddf79585abbbcd90d8e87ab45f9c10fb54a8ddc278a448c9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := briefPosture{}
			hash := sha256.New()
			var encoded [8]byte
			for ms := 0.; ms < 60000; ms += 1000. / 30 {
				h := p.sample(ms, true, 2762064193, Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}, tc.quiet)
				for _, value := range []float64{h.Yaw, h.Pitch, h.Roll} {
					binary.LittleEndian.PutUint64(encoded[:], math.Float64bits(value))
					hash.Write(encoded[:])
				}
			}
			got := fmt.Sprintf("%x", hash.Sum(nil))
			t.Log("speaking trace SHA256", got)
			if got != tc.want {
				t.Fatal("idle retuning changed speaking posture", got, tc.want)
			}
		})
	}
}

func TestSpeakingPresenceUsesBriefTransitionsAndPurposeSpecificHolds(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		p := briefPosture{}
		p.sample(0, true, 17, Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}, quiet)
		for ms := 1000.; ms < 30000; ms += 1000 {
			p.sample(ms, true, 17, Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}, quiet)
			if p.duration < 280 || p.duration > 500 {
				t.Fatal("speaking transition slowed", p.duration)
			}
			low, high := 800., 1600.
			if quiet {
				low, high = 1600, 2800
			}
			// Idle retains its historical extension; speaking holds use their profile.
			if p.hold < low || p.hold > high {
				t.Fatal("speaking hold escaped its profile", quiet, p.hold)
			}
		}
	}
}

func TestIdlePostureAvoidsLongStaticRunsWithoutIncreasingMotionRange(t *testing.T) {
	t.Parallel()
	p := briefPosture{}
	const seed = uint32(2762064193)
	var previous Head
	moving, run, longest := 0, 0, 0
	for ms := 0.; ms < 60000; ms += 1000. / 30 {
		h := p.sample(ms, false, seed, Bounds{Yaw: 8, Pitch: 4, Roll: 1.2})
		if math.Abs(h.Yaw) > 1.8+1e-9 || math.Abs(h.Pitch) > .65+1e-9 || math.Abs(h.Roll) > .18+1e-9 {
			t.Fatal("idle escaped its existing posture range", h)
		}
		if p.duration != 0 && (p.duration < 550 || p.duration > 900) {
			t.Fatal("idle adjustment speed changed", p.duration)
		}
		if ms > 0 {
			if math.Abs(h.Yaw-previous.Yaw)+math.Abs(h.Pitch-previous.Pitch)+math.Abs(h.Roll-previous.Roll) > .003 {
				moving++
				run = 0
			} else {
				run++
				longest = max(longest, run)
			}
		}
		previous = h
	}
	t.Logf("idle moving steps=%d/1799; longest nearly-static run=%.3fs", moving, float64(longest)/30)
	if float64(longest)/30 > 6.5 {
		t.Fatal("idle retained the prolonged unchanged pose", float64(longest)/30)
	}
	if moving < 300 || moving > 600 {
		t.Fatal("idle should use occasional adjustments with genuine holds", moving)
	}
}

func TestIdlePosturePreservesPauseAndReducedMotion(t *testing.T) {
	t.Parallel()
	c := Config{Conversational: true, ConversationalMotion: true, Seed: 2762064193, Bounds: Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}}
	for i := range c.Eyes {
		c.Eyes[i] = GazeEye{Width: 100, Bounds: [2][2]float64{{5, 5}, {2, 2}}}
	}
	engine := NewEngine(c)
	settings := Settings{Movement: true}
	engine.Sample(0, SpeechState{}, settings)
	before := engine.Sample(2500, SpeechState{}, settings)
	settings.Paused = true
	if paused := engine.Sample(60000, SpeechState{}, settings); paused != before {
		t.Fatal("paused idle changed its visible frame")
	}
	settings.Paused = false
	settings.ReducedMotion = true
	for _, ms := range []float64{65000, 70000, 75000} {
		frame := engine.Sample(ms, SpeechState{}, settings)
		if frame.Head != (Head{}) || frame.Breath != 0 || frame.Gaze != ([2][2]float64{}) {
			t.Fatal("idle retuning bypassed reduced motion", frame)
		}
	}
}
