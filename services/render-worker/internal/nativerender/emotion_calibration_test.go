package nativerender

import (
	"os"
	"syntelle/render-contract"
	"testing"
)

func TestPreparedEyelidCalibrationSeparatesAngerAndDisgust(t *testing.T) {
	if _, err := os.Stat("../../../../.tools/emotion-readability-87/package-v3/manifest.json"); err != nil {
		t.Skip("cached private calibration required")
	}
	a, err := Load("../../../../.tools/emotion-readability-87/package-v3")
	if err != nil {
		t.Fatal(err)
	}
	r := &Renderer{assets: a}
	var state FrameState
	state.Emotion = rendercontract.EmotionWeights{}
	state.Emotion[6] = 1
	if v := r.laughSquint(state); v != .28 {
		t.Fatal("anger", v)
	}
	state.Emotion[6] = 0
	state.Emotion[9] = 1
	if v := r.laughSquint(state); v != .07 {
		t.Fatal("disgust", v)
	}
	a.Expression.EmotionRig.EyelidTension = nil
	if v := r.laughSquint(state); v != .14 {
		t.Fatal("legacy fallback", v)
	}
}
