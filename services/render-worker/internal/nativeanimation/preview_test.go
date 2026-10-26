package nativeanimation

import (
	"fmt"
	"math"
	"testing"

	"syntelle/render-contract"
)

func TestNodPreviewIsQuickVisibleAndScalesWithIntensity(t *testing.T) {
	c := Config{Bounds: Bounds{3, 1.2, .3}}
	peak := .14
	full := PreviewFrameAtIntensity(Frame{}, "nod", peak, 1, c)
	half := PreviewFrameAtIntensity(Frame{}, "nod", peak, .5, c)
	if math.Abs(full.Head.Pitch) < c.Bounds.Pitch*.94 || math.Abs(half.Head.Pitch/full.Head.Pitch-.5) > 1e-9 {
		t.Fatal("preview did not use its prepared range or requested intensity", full, half)
	}
	if PreviewFrameAtIntensity(Frame{}, "nod", .5, 1, c) != (Frame{}) {
		t.Fatal("nod did not finish promptly")
	}
	for _, value := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		if PreviewFrameAtIntensity(Frame{}, "nod", peak, value, c) != (Frame{}) {
			t.Fatal("invalid intensity admitted")
		}
	}
}

func TestRepeatedNodsUseOneCompleteCurveAndRestoreBaseline(t *testing.T) {
	c := Config{Bounds: Bounds{3, 4, .3}}
	for _, n := range []int{1, 2, 4, 5, 7, 8} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			for i := 0; i < n; i++ {
				peak := (float64(i) + .34) * rendercontract.NaturalNodCycleSeconds
				frame := PreviewFrameRepeated(Frame{}, "nod", peak, 1, n, c)
				if math.Abs(frame.Head.Pitch+3.8) > 1e-8 {
					t.Fatal("incomplete nod", i, frame)
				}
				if got := PreviewFrameRepeated(Frame{}, "nod", float64(i+1)*rendercontract.NaturalNodCycleSeconds, 1, n, c); math.Abs(got.Head.Pitch) > 1e-8 {
					t.Fatal("nod did not return", i, got)
				}
			}
			base := Frame{Smile: .7, Brows: .4, Head: Head{Pitch: .3}}
			if got := PreviewFrameRepeated(base, "nod", float64(n)*rendercontract.NaturalNodCycleSeconds+.01, 1, n, c); got != base {
				t.Fatal("baseline not restored", got)
			}
		})
	}
}

func TestVisualPreviewIsBoundedAndReturnsToLatestBaseline(t *testing.T) {
	c := Config{Expressions: true, SpeakingSmile: true, LaughAvailable: true, Bounds: Bounds{3, 1.2, .3}}
	base := Frame{Smile: .25, Brows: .1}
	for _, kind := range []string{"smile", "laugh", "brows", "nod", "double_nod", "shake", "tilt"} {
		if !ValidPreview(kind) {
			t.Fatal(kind)
		}
		if PreviewFrame(base, kind, 0, c) != base || PreviewFrame(base, kind, 2, c) != base {
			t.Fatal("preview changed baseline outside its window", kind)
		}
		sample := .5
		if kind == "nod" {
			sample = .14
		}
		if PreviewFrame(base, kind, sample, c) == base {
			t.Fatal("preview did not move", kind)
		}
	}
	if PreviewFrame(base, "laugh", .5, Config{}) != base {
		t.Fatal("unprepared laughter admitted")
	}
	if ValidPreview("[Laugh]") {
		t.Fatal("free-form command admitted")
	}
}

func TestShakeAndTiltUsePreparedRangeAndRestoreBaseline(t *testing.T) {
	c := Config{Bounds: Bounds{3, 4, 1.2}}
	for _, n := range []int{1, 4, 8} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			for i := 0; i < n; i++ {
				for _, side := range []struct{ phase, yaw float64 }{{.24, 2.85}, {.70, -2.85 * .82}} {
					at := (float64(i) + side.phase) * rendercontract.NaturalShakeCycleSeconds
					full := PreviewFrameRepeated(Frame{}, "shake", at, 1, n, c)
					half := PreviewFrameRepeated(Frame{}, "shake", at, .5, n, c)
					if math.Abs(full.Head.Yaw-side.yaw) > 1e-8 || math.Abs(half.Head.Yaw-side.yaw*.5) > 1e-8 {
						t.Fatal("incomplete shake", i, full, half)
					}
				}
			}
			base := Frame{Head: Head{Yaw: .3}, Smile: .7}
			if got := PreviewFrameRepeated(base, "shake", float64(n)*rendercontract.NaturalShakeCycleSeconds+.01, 1, n, c); got != base {
				t.Fatal("baseline did not return", got)
			}
		})
	}
	full := PreviewFrameAtIntensity(Frame{}, "tilt", 1.15*.42, 1, c)
	half := PreviewFrameAtIntensity(Frame{}, "tilt", 1.15*.42, .5, c)
	if math.Abs(full.Head.Roll-1.14) > 1e-8 || math.Abs(half.Head.Roll-.57) > 1e-8 {
		t.Fatal("tilt not visible or scalable", full, half)
	}
}

func TestShakeReturnsWithin650Milliseconds(t *testing.T) {
	c := Config{Bounds: Bounds{3, 4, 1.2}}
	first := PreviewFrameAtIntensity(Frame{}, "shake", .14, 1, c)
	second := PreviewFrameAtIntensity(Frame{}, "shake", .41, 1, c)
	if first.Head.Yaw < 2.6 || second.Head.Yaw > -2.1 {
		t.Fatal("faster shake lost its opposing excursions", first.Head, second.Head)
	}
	base := Frame{Head: Head{Yaw: .3}, Smile: .2}
	if got := PreviewFrameAtIntensity(base, "shake", .65, 1, c); got != base {
		t.Fatal("shake still takes longer than its new playback window", got)
	}
}
