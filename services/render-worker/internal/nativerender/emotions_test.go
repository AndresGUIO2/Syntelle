package nativerender

import (
	"bytes"
	"context"
	"crypto/sha256"
	"image"
	"os"
	"path/filepath"
	"testing"

	"syntelle/render-contract"
)

func TestEmotionNeutralEquivalenceAndProtectedMaterials(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", ".tools")
	directory := filepath.Join(root, "emotions-82", "package-v11")
	if _, err := os.Stat(filepath.Join(directory, "manifest.json")); err != nil {
		t.Skip("cached private emotion package unavailable")
	}
	currentAssets, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	previousAssets, err := Load(filepath.Join(root, "speech-lighting-81", "package-v2"))
	if err != nil {
		t.Fatal(err)
	}
	current, err := newRenderer(currentAssets, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	previous, err := newRenderer(previousAssets, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer previous.Close()
	got, want := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	upper, lower := sha256.Sum256(currentAssets.Oral.upper.Pixels), sha256.Sum256(currentAssets.Oral.lower.Pixels)
	for _, state := range []FrameState{
		{From: "Rest", To: "A", RestingSmile: true},
		{From: "Rest", To: "A", RestingSmile: true, Smile: 1, Brows: .7},
		{From: "FV", To: "A", Amount: .2, Smile: .5, Brows: 1},
		{From: "MBP", To: "O", Amount: .3, Smile: .4},
	} {
		if err := current.Render(context.Background(), state, got); err != nil {
			t.Fatal(err)
		}
		if err := previous.Render(context.Background(), state, want); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatal("neutral emotion changed historical rendering", state)
		}
	}
	for index := range rendercontract.EmotionStates {
		for _, pose := range []string{"MBP", "FV", "A"} {
			state := FrameState{From: pose, To: "O", Amount: .1}
			state.Emotion[index] = 1
			if err := current.Render(context.Background(), state, got); err != nil {
				t.Fatal(err)
			}
		}
	}
	if sha256.Sum256(currentAssets.Oral.upper.Pixels) != upper || sha256.Sum256(currentAssets.Oral.lower.Pixels) != lower {
		t.Fatal("emotional articulation modified canonical tooth materials")
	}
}

func TestEmotionalSmileEditMaskFollowsItsLipTexture(t *testing.T) {
	assets, _ := patchOralFixture(t)
	pose := assets.poses["Rest"]
	// One source-owned column moves right with the lip mesh. Testing both
	// paths reproduces the old clipped corner and proves ownership travels.
	pose.mask = patchSolid(4, 4, [4]byte{0, 0, 0, 255})
	for y := range 4 {
		pose.mask.Pixels[y*4*4] = 255
	}
	mesh := assets.pairs["Rest-A"].Mesh
	for _, moving := range []bool{false, true} {
		t.Run(map[bool]string{false: "historical", true: "deformed"}[moving], func(t *testing.T) {
			r := NewOralRenderer(assets)
			s := oralShadeState{joint: true, deformedCoverage: moving, from: pose, to: pose}
			for _, ids := range mesh.Triangles {
				var points [3][2]float64
				for n, id := range ids {
					points[n] = [2]float64{mesh.Rest[id][0] + 1, mesh.Rest[id][1]}
				}
				r.drawTriangle(mesh, ids, points, &s)
			}
			alpha := r.output.Pixels[(1*4+1)*4+3]
			if moving && alpha != 255 {
				t.Fatal("moving lip was cut by stationary edit coverage", alpha)
			}
			if !moving && alpha != 0 {
				t.Fatal("historical comparator no longer reproduces clipping", alpha)
			}
			if r.output.Pixels[(1*4+2)*4+3] != 0 {
				t.Fatal("ownership leaked outside the transported mask")
			}
		})
	}
}
