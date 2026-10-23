package nativerender

import (
	"bytes"
	"context"
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestRestingSmileMatchesPreviousRegisteredRenderer(t *testing.T) {
	directory := filepath.Join("..", "..", "..", "..", ".tools", "expression-controls-75", "runtime-package")
	if _, err := os.Stat(directory); err != nil {
		t.Skip("local correction package unavailable")
	}
	assets, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	current, err := newRenderer(assets, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	// Use the original branch of the renderer as a reference, retaining the
	// original registered material, field, neutral mouth, eyes and projection.
	previousAssets, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	previousAssets.Expression.Joint = nil
	previousAssets.SkinFrames = nil
	previousAssets.LaughFrames = nil
	previous, err := newRenderer(previousAssets, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	got, want := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	for _, amount := range []float64{0, .5, 1, .5, 0} {
		state := FrameState{From: "Rest", To: "A", Smile: amount, RestingSmile: true, Background: "dark-lounge"}
		if err := current.Render(context.Background(), state, got); err != nil {
			t.Fatal(err)
		}
		if err := previous.Render(context.Background(), state, want); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("resting smile differs from the previous renderer at %.2f", amount)
		}
	}
}
