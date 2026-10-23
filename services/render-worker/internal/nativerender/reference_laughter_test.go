package nativerender

import (
	"bytes"
	"context"
	"crypto/sha256"
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
