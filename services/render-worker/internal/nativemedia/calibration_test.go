package nativemedia

import (
	"context"
	"image"
	"testing"
	"time"
)

func TestMeasureTimingRequiresExplicitOfflineHost(t *testing.T) {
	t.Parallel()
	h := &Host{}
	if err := h.MeasureTiming("fixture_native"); err == nil {
		t.Fatal("production-default host admitted a calibration")
	}
	s := queuedSession()
	s.ctx = context.Background()
	s.mediaTime = time.Second
	h = &Host{config: Config{OfflineCalibration: true}, sessions: map[string]*session{"fixture_native": s}}
	if err := h.MeasureTiming("fixture_native"); err != nil {
		t.Fatal(err)
	}
	start, end := s.calibration()
	if start != 1250*time.Millisecond || end != 1500*time.Millisecond {
		t.Fatalf("calibration did not share the media epoch: %v to %v", start, end)
	}
	for _, at := range []time.Duration{start - 20*time.Millisecond, start, end} {
		frame := image.NewRGBA(image.Rect(0, 0, 64, 64))
		pcm := make([]int16, audioSamples)
		timingMarker(frame, at, start, end)
		timingTone(pcm, at, start, end)
		visible := frame.RGBAAt(0, 0).R == 255 && frame.RGBAAt(0, 0).B == 255
		audible := false
		for _, sample := range pcm {
			if sample != 0 {
				audible = true
				break
			}
		}
		if visible != (at == start) || audible != visible {
			t.Fatalf("audio/video calibration windows diverged at %v: visible=%v audible=%v", at, visible, audible)
		}
		if visible && frame.RGBAAt(32, 0).R != 0 {
			t.Fatal("marker exceeded its 32 pixel boundary")
		}
	}
}

func TestTimingToneDoesNotOverwritePCMOutsideItsWindow(t *testing.T) {
	t.Parallel()
	pcm := make([]int16, audioSamples)
	for i := range pcm {
		pcm[i] = 32000
	}
	start := 10 * time.Millisecond
	timingTone(pcm, 0, start, 15*time.Millisecond)
	for i := 0; i < sampleRate/100; i++ {
		if pcm[i] != 32000 {
			t.Fatal("tone changed earlier PCM")
		}
	}
	for i := sampleRate * 15 / 1000; i < len(pcm); i++ {
		if pcm[i] != 32000 {
			t.Fatal("tone changed later PCM")
		}
	}
	changed := false
	for _, v := range pcm {
		if v < 32000 {
			changed = true
		}
	}
	if !changed {
		t.Fatal("bounded mixed tone was absent")
	}
}
