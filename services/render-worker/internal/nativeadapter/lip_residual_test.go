package nativeadapter

import (
	"syntelle/render-worker/internal/nativeanimation"
	"testing"
)

func TestLipResidualPauseKeepsLastRenderedState(t *testing.T) {
	speech, err := nativeanimation.BuildSpeech(make([]float32, 24000), 24000, []nativeanimation.NativeViseme{{ID: 6, OffsetMS: 0}, {ID: 4, OffsetMS: 500}}, 1, nativeanimation.Config{ContextualArticulation: true})
	if err != nil {
		t.Fatal(err)
	}
	s := scene{speech: speech}
	last := s.sampleLipResidual(.25, true, false)
	if s.sampleLipResidual(.75, true, true) != last {
		t.Fatal("pause resampled a different source position")
	}
	if s.sampleLipResidual(.75, true, false) == last {
		t.Fatal("resume failed to sample new context")
	}
	if s.sampleLipResidual(.8, false, false) != ([2]float64{}) {
		t.Fatal("inactive/laughter residual retained")
	}
}
