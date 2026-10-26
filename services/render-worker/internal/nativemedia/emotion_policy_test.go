package nativemedia

import (
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"testing"
	"time"
)

func TestEmotionPolicyCommitFollowsSentAudioAndConfigurationEpoch(t *testing.T) {
	for _, mode := range []string{"sent", "unsent", "cancelled", "edited", "paused"} {
		t.Run(mode, func(t *testing.T) {
			s := queuedSession()
			s.scene = emotionScene{}
			s.turn = "turn_fixture"
			s.emotionInstance = "fixture_instance"
			s.settings = rendercontract.Defaults()
			v := rendercontract.EmotionTarget{State: "angry", Intensity: 1, Energy: 1}
			s.queue = []*clip{{samples: make([]int16, audioSamples*2), prepared: &nativeanimation.PreparedSpeech{Performance: &nativeanimation.Performance{Intent: &nativeanimation.PerformanceIntent{Version: "performance-intent/3", Emotion: &v}}}}}
			s.paused = mode == "paused"
			block := s.nextAudioBlock(time.Now(), make([]int16, audioSamples))
			if s.emotionController.Sequence != 0 {
				t.Fatal("staging mutated committed state")
			}
			if mode == "cancelled" {
				s.generation++
			}
			if mode == "edited" {
				s.emotionEpoch++
			}
			decisions := s.commitEmotions(block, mode != "unsent")
			if mode == "unsent" || mode == "cancelled" || mode == "edited" {
				if len(decisions) != 0 || s.emotionController.Sequence != 0 {
					t.Fatal("unplayed target committed")
				}
				return
			}
			if mode == "paused" {
				if s.emotionClock != 0 {
					t.Fatal("paused clock advanced")
				}
				return
			}
			if len(decisions) != 2 || decisions[1].Effective.Intensity != .18 || decisions[1].Proposed.Intensity != 1 || decisions[1].Instance != "fixture_instance" {
				t.Fatal(decisions)
			}
		})
	}
}
