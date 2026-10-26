package nativemedia

import (
	"context"
	"image"
	"testing"
	"time"

	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
)

type emotionScene struct{}

func (emotionScene) Render(context.Context, FrameInput, *image.RGBA) error { return nil }
func (emotionScene) EmotionsAvailable() bool                               { return true }

func TestEmotionActivationWaitsForPCMAndRespectsUserOverrides(t *testing.T) {
	for _, mode := range []string{"active", "paused", "preview", "disabled", "reduced", "edited", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			s := queuedSession()
			s.settings.EmotionRegulation = rendercontract.EmotionRegulation{Enabled: false, Temperament: "calm"}
			if mode != "legacy" {
				s.scene = emotionScene{}
			}
			s.turn, s.generation = "turn_fixture", 7
			s.paused = mode == "paused"
			s.settings.AutomaticExpressions = mode != "disabled"
			s.settings.ReducedMotion = mode == "reduced"
			if mode == "edited" {
				s.emotionSuppressedTurn = s.turn
			}
			target := rendercontract.EmotionTarget{State: "sad", Intensity: .7, Energy: .3}
			s.queue = []*clip{{phraseIndex: 2, preview: mode == "preview", samples: make([]int16, audioSamples*2),
				prepared: &nativeanimation.PreparedSpeech{Performance: &nativeanimation.Performance{
					Intent: &nativeanimation.PerformanceIntent{Version: "performance-intent/3", Emotion: &target},
				}}}}
			block := s.nextAudioBlock(time.Now(), make([]int16, audioSamples))
			if mode != "active" {
				if len(block.activations) != 0 {
					t.Fatal("unplayed or suppressed emotion entered context", block.activations)
				}
				return
			}
			if len(block.activations) != 1 || block.activations[0].Generation != 7 || block.activations[0].PhraseIndex != 2 || block.activations[0].Emotion != target {
				t.Fatal("activation does not identify admitted PCM", block.activations)
			}
			if len(s.nextAudioBlock(time.Now(), make([]int16, audioSamples)).activations) != 0 {
				t.Fatal("a continued phrase activated twice")
			}
		})
	}
}
