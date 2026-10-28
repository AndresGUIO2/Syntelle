package nativerender

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReferenceContactShadowOnlyDimsCoveredEnamel(t *testing.T) {
	shadow := &referenceContactShadow{Version: "aperture-contact-shadow/1", Strength: .3, Width: 7}
	for _, tc := range []struct {
		name          string
		distance, gap float64
		unchanged     bool
	}{
		{"reference", 0, 60, true}, {"opening beyond reference", 1, 65, true},
		{"partly closing", 2, 20, false}, {"nearly closed", 0, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := referenceToothExposure(tc.distance, tc.gap, 60, shadow)
			if (tc.unchanged && got != 1) || (!tc.unchanged && (got >= 1 || got < math.Pow(.7, 1/2.2))) {
				t.Fatal(got)
			}
		})
	}
	if referenceToothExposure(0, 1, 60, nil) != 1 {
		t.Fatal("historical reference changed")
	}
	if referenceToothExposure(0, 10, 60, shadow) >= referenceToothExposure(10, 10, 60, shadow) {
		t.Fatal("shadow does not follow lip distance")
	}
	if referenceToothExposure(1, 10, 60, shadow) >= referenceToothExposure(1, 40, 60, shadow) {
		t.Fatal("shadow does not follow closure")
	}
}

func referenceTestAssets(t *testing.T) *Assets {
	t.Helper()
	dir := os.Getenv("SYNTELLE_REFERENCE_TEST_PACKAGE")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "..", ".tools", "reference-laughter-76", "package")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skip("local reference laughter package unavailable")
	}
	a, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.Expression.ReferenceLaughter == nil {
		t.Fatal("fixture has no independent reference expression")
	}
	return a
}

func TestReferenceLaughterOwnsWholeFaceAndRestoresManualSmile(t *testing.T) {
	a := referenceTestAssets(t)
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	base := FrameState{From: "Rest", To: "A", Smile: 1, RestingSmile: true, Background: "dark-lounge"}
	before, after, peak, other := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	if err = r.Render(context.Background(), base, before); err != nil {
		t.Fatal(err)
	}
	upper, lower := sha256.Sum256(a.Oral.upper.Pixels), sha256.Sum256(a.Oral.lower.Pixels)
	state := base
	state.Laugh = 1
	state.Amount = .5
	state.RestingSmile = false
	if err = r.Render(context.Background(), state, peak); err != nil {
		t.Fatal(err)
	}
	state.Smile = 0
	if err = r.Render(context.Background(), state, other); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(peak.Pix, other.Pix) {
		t.Fatal("peak laughter still depends on manual smile")
	}
	if bytes.Equal(peak.Pix, before.Pix) {
		t.Fatal("reference face did not visibly replace the resting expression")
	}
	state.Closure = 1
	if err = r.Render(context.Background(), state, other); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(peak.Pix, other.Pix) {
		t.Fatal("full reference expression disabled blinking")
	}
	state.Closure = 0
	state.Gaze[0][0] = 1
	state.Gaze[1][0] = 1
	if err = r.Render(context.Background(), state, other); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(peak.Pix, other.Pix) {
		t.Fatal("full reference expression disabled gaze")
	}
	if err = r.Render(context.Background(), base, after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Pix, after.Pix) {
		t.Fatal("manual resting smile did not restore exactly after laughter")
	}
	if upper != sha256.Sum256(a.Oral.upper.Pixels) || lower != sha256.Sum256(a.Oral.lower.Pixels) {
		t.Fatal("laughter mutated canonical dental pixels")
	}
}

func TestReferenceLaughterLowAmountRemainsContinuous(t *testing.T) {
	a := referenceTestAssets(t)
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	baseline := FrameState{From: "Rest", To: "A", Smile: 1, RestingSmile: true, Background: "dark-lounge"}
	before, after := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	if err = r.Render(context.Background(), baseline, before); err != nil {
		t.Fatal(err)
	}
	state := baseline
	state.Laugh = .001
	if err = r.Render(context.Background(), state, after); err != nil {
		t.Fatal(err)
	}
	// Render quantization makes an amount below one prepared step exactly zero.
	if !bytes.Equal(before.Pix, after.Pix) {
		t.Fatal("substep laughter abruptly erased the manual baseline")
	}
}

// A full amused face during an acoustic gap must retain a source-supported
// closed smile, rather than compressing the open laugh's lips into a strip.
func TestFullLaughterGapKeepsClosedSmileContour(t *testing.T) {
	root := os.Getenv("SYNTELLE_LAUGHTER_ASSETS")
	if root == "" {
		t.Skip("cached laughter companion")
	}
	a, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, smile := range []float64{0, .5, 1} {
		t.Run(fmt.Sprint(smile), func(t *testing.T) {
			state := FrameState{From: "Rest", To: "A", Smile: smile, Laugh: 1, JawOpen: 0}
			frame := image.NewRGBA(image.Rect(0, 0, 720, 960))
			if err := r.Render(t.Context(), state, frame); err != nil {
				t.Fatal(err)
			}
			got := r.referenceState(state)
			for i := range got.inner {
				if got.inner[i] != r.oral.shadeState.inner[i] || got.outer[i] != r.oral.outer[i] {
					t.Fatalf("quiet laughter crushed the open reference instead of retaining the calibrated closed smile: point %d", i)
				}
			}
			previous := -1.
			var gaps []float64
			for _, jaw := range []float64{0, .25, .5, .75, 1} {
				state.JawOpen = jaw
				if err := r.Render(t.Context(), state, frame); err != nil {
					t.Fatal(err)
				}
				got = r.referenceState(state)
				gap := referenceColumnGap((got.inner[0][0]+got.inner[10][0])*.5, got.inner[:])
				gaps = append(gaps, gap)
				if gap <= previous {
					t.Fatalf("laughter opening is not responsive and monotonic: %v", gaps)
				}
				previous = gap
				if got.upper != r.oral.shadeState.upper {
					t.Fatal("upper material drifted with laughter opening")
				}
				for triangleIndex, triangle := range got.triangles {
					fromArea := patchEdge(triangle.From[0], triangle.From[1], triangle.From[2])
					toArea := patchEdge(triangle.To[0], triangle.To[1], triangle.To[2])
					if fromArea <= 0 || toArea <= 0 || triangle.Area <= 0 {
						t.Fatalf("oral material mapping folds at jaw %g triangle %d areas %g/%g/%g", jaw, triangleIndex, fromArea, toArea, triangle.Area)
					}
					for pointIndex, point := range triangle.P {
						from, to := referenceCoordinates(point[0]-.5, point[1]-.5, got.triangles)
						if math.Hypot(from[0]-triangle.From[pointIndex][0], from[1]-triangle.From[pointIndex][1]) > .001 || math.Hypot(to[0]-triangle.To[pointIndex][0], to[1]-triangle.To[pointIndex][1]) > .001 {
							t.Fatal("lip material does not share final contour correspondence", jaw, triangleIndex, pointIndex)
						}
					}
				}
			}
			t.Log("center aperture px at jaw 0/.25/.5/.75/1:", gaps)
			if math.Abs(gaps[4]-a.Motion.Mandible.ReferenceGap) > 1 {
				t.Fatal("full laughter lost its source-calibrated aperture", gaps[4], a.Motion.Mandible.ReferenceGap)
			}
		})
	}
}
