package nativerender

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedForegroundIgnoresBackgroundAndPreservesCharacter(t *testing.T) {
	directory := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	if directory == "" {
		t.Skip("set SYNTELLE_NATIVE_ASSETS for the bounded prepared-character check")
	}
	a, err := LoadCharacter(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Backgrounds) != 0 {
		t.Fatal("live character loader loaded background resources")
	}
	r, err := New(a)
	if err != nil {
		t.Fatal(err)
	}
	states := []FrameState{
		{From: "Rest", To: "A", Background: "not-a-server-background", Blur: 20},
		{From: "EI", To: "FV", Amount: .35, Closure: .5, Head: [3]float64{2, -.5, .1}},
		{From: "A", To: "U", Amount: .65, Closure: 1, Head: [3]float64{-2, .5, -.1}},
	}
	for i, state := range states {
		packed, changed := image.NewRGBA(image.Rect(0, 0, 1440, 960)), image.NewRGBA(image.Rect(0, 0, 1440, 960))
		if err = r.Render(context.Background(), state, packed); err != nil {
			t.Fatal(err)
		}
		state.Background = "different-background"
		state.Blur = 0
		if err = r.Render(context.Background(), state, changed); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(packed.Pix, changed.Pix) || r.background != nil {
			t.Fatal("background settings affected the transmitted character")
		}
		if output := os.Getenv("SYNTELLE_PACKED_FIXTURES"); output != "" {
			if err = os.MkdirAll(output, 0700); err != nil {
				t.Fatal(err)
			}
			file, err := os.Create(filepath.Join(output, []string{"neutral.png", "speech-blink.png", "turn-blink.png"}[i]))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(file, packed)
			closeErr := file.Close()
			if err != nil || closeErr != nil {
				t.Fatal(errors.Join(err, closeErr))
			}
		}
	}
}

func Test720ForegroundReducesInternalAndTransportRaster(t *testing.T) {
	directory := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	if directory == "" {
		t.Skip("set SYNTELLE_NATIVE_ASSETS for the prepared-character check")
	}
	t.Setenv("SYNTELLE_RENDERER", "cpu")
	assets, err := LoadCharacter(directory)
	if err != nil {
		t.Fatal(err)
	}
	full, err := NewSized(assets, 720, 960)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	small, err := NewSized(assets, 540, 720)
	if err != nil {
		t.Fatal(err)
	}
	defer small.Close()
	if small.body.Width >= full.body.Width || small.body.Height >= full.body.Height {
		t.Fatal("internal body raster was not reduced")
	}
	frame := image.NewRGBA(image.Rect(0, 0, 1080, 720))
	if err := small.Render(context.Background(), FrameState{From: "Rest", To: "A"}, frame); err != nil {
		t.Fatal(err)
	}
	if len(frame.Pix) != 1080*720*4 {
		t.Fatal("incorrect packed frame size")
	}
	for y := 0; y < 720; y += 37 {
		for x := 0; x < 540; x += 29 {
			i := (y*1080 + x + 540) * 4
			if frame.Pix[i] != frame.Pix[i+1] || frame.Pix[i] != frame.Pix[i+2] || frame.Pix[i+3] != 255 {
				t.Fatal("720-high alpha half is not opaque grayscale")
			}
		}
	}
}

func TestForegroundTransportReconstructsPremultipliedEdges(t *testing.T) {
	t.Parallel()
	r := rasterFixture()
	copy(r.body.Pixels, r.subjectTexture.Pixels)
	for x := range 720 {
		a := x * 31 / 720
		r.composeX[x] = sampleAxis{A: a * 4, B: min(a+1, 31) * 4, F: float64(x%13) / 13, Valid: x > 13 && x < 706}
	}
	for y := range 960 {
		a := y * 31 / 960
		r.composeY[y] = sampleAxis{A: a * 32 * 4, B: min(a+1, 31) * 32 * 4, F: float64(y%17) / 17, Valid: y > 7 && y < 948}
	}
	packed := image.NewRGBA(image.Rect(0, 0, 1440, 960))
	if err := r.packForeground(context.Background(), packed); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		color [4]byte
	}{
		{"dark", [4]byte{3, 7, 11, 255}}, {"light", [4]byte{247, 251, 255, 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r.background = patchSolid(720, 960, tc.color)
			want := image.NewRGBA(image.Rect(0, 0, 720, 960))
			referenceCompose(r, want)
			for y := range 960 {
				for x := range 720 {
					i, m := (y*1440+x)*4, (y*1440+x+720)*4
					alpha := packed.Pix[m]
					if packed.Pix[m+1] != alpha || packed.Pix[m+2] != alpha || packed.Pix[m+3] != 255 {
						t.Fatal("mask is not opaque grayscale")
					}
					for c := range 3 {
						got := int(packed.Pix[i+c]) + int(float64(tc.color[c])*(1-float64(alpha)/255)+.5)
						delta := got - int(want.Pix[(y*720+x)*4+c])
						if delta < -1 || delta > 1 {
							t.Fatalf("pixel %d,%d channel %d difference %d", x, y, c, delta)
						}
					}
				}
			}
		})
	}
}

func TestForegroundTransportCancelsBeforeWriting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := rasterFixture()
	if err := r.packForeground(ctx, image.NewRGBA(image.Rect(0, 0, 1440, 960))); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
