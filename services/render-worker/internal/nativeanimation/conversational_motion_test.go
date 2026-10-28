package nativeanimation

import (
	"math"
	"syntelle/render-contract"
	"testing"
)

func TestConversationalPostureIsVisibleBoundedAndContinuous(t *testing.T) {
	c := Config{Conversational: true, ConversationalMotion: true, Expressive: true, Seed: 17, Bounds: Bounds{Yaw: 8, Pitch: 4, Roll: 1.2}}
	d := newAttention(c.Seed)
	last := Head{}
	extent := Head{}
	for ms := 0.; ms < 60000; ms += 1000. / 30 {
		f := d.sample(ms, SpeechState{Active: true, Session: 1}, c).Head
		if math.Abs(f.Yaw) > 8 || math.Abs(f.Pitch) > 4 || math.Abs(f.Roll) > 1.2 {
			t.Fatal("escaped prepared bounds", f)
		}
		if ms > 0 && (math.Abs(f.Yaw-last.Yaw) > .65 || math.Abs(f.Pitch-last.Pitch) > .65) {
			t.Fatal("posture jumped", ms, last, f)
		}
		extent.Yaw = math.Max(extent.Yaw, math.Abs(f.Yaw))
		extent.Pitch = math.Max(extent.Pitch, math.Abs(f.Pitch))
		last = f
	}
	if extent.Yaw < 3 || extent.Pitch < .8 {
		t.Fatal("speaking posture remained too quiet", extent)
	}
}

func TestConversationalMotionAdmitsLongReplyAndCarriesPhraseTail(t *testing.T) {
	c := Config{ConversationalMotion: true, Bounds: Bounds{Yaw: 6, Pitch: 4, Roll: 1.2}}
	e := NewEngine(c)
	amount := .8
	makePerformance := func(offset float64) *Performance {
		return &Performance{Motion: &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: "reply", Offset: offset}, Intent: &PerformanceIntent{Version: "performance-intent/3", Text: "A point.", Cues: []PerformanceAnchor{{Kind: "emphasis", Anchor: "point"}}}, Resolved: []PerformanceCue{{Kind: "emphasis", Time: .1, Duration: .72, Amount: &amount}}}
	}
	for i := 0; i < 12; i++ {
		if got := e.AdmitPerformance(makePerformance(float64(i)*3), "A point.", float64(i)*3000); len(got.Resolved) != 1 {
			t.Fatal("long reply stopped moving", i)
		}
	}
	p := &PreparedSpeech{directed: true, continuous: true, bounds: c.Bounds, duration: .4}
	s := BindSpeech(p, makePerformance(0))
	if len(s.Plan.Gestures) != 1 {
		t.Fatal("gesture dropped at short phrase boundary")
	}
	d := newAttention(7)
	c.Conversational = true
	d.sample(100, SpeechState{Active: true, Session: 1, Position: .35, Speech: s}, c)
	next := makePerformance(.4)
	next.Resolved = nil
	got := d.sample(150, SpeechState{Active: true, Session: 2, Position: .05, Speech: BindSpeech(p, next)}, c)
	if got.Head.Pitch >= -.05 {
		t.Fatal("previous phrase tail disappeared", got.Head)
	}
}

func TestLaughterInterruptionRecoversAndPauseFreezes(t *testing.T) {
	e := NewEngine(Config{OralRecovery: true})
	e.last = Frame{Laugh: .7, Mouth: Mouth{"Rest", "A", .6}}
	e.initialized = true
	settings := DefaultSettings()
	settings.Movement = false
	settings.Blinking = false
	first := e.Sample(10, SpeechState{}, settings)
	if first.Laugh != .7 {
		t.Fatal("interruption jumped", first)
	}
	half := e.Sample(130, SpeechState{}, settings)
	if half.Laugh <= 0 || half.Laugh >= first.Laugh {
		t.Fatal("no bounded recovery", half)
	}
	settings.Paused = true
	if e.Sample(130, SpeechState{}, settings) != half {
		t.Fatal("pause advanced recovery")
	}
	settings.Paused = false
	if e.Sample(250, SpeechState{}, settings).Laugh != 0 {
		t.Fatal("recovery did not finish")
	}
}
