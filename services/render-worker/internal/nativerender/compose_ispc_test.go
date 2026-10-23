//go:build ispc && cgo && linux && amd64

package nativerender

import (
	"bytes"
	"context"
	"errors"
	"image"
	"testing"
)

func compositionFixture() *Renderer {
	r := &Renderer{body: NewTexture(128, 192, true), background: NewTexture(720, 960, true)}
	for i := 0; i < len(r.body.Pixels); i += 4 {
		a := byte((i / 4) * 17)
		r.body.Pixels[i], r.body.Pixels[i+1], r.body.Pixels[i+2], r.body.Pixels[i+3] = a/2, a/3, a/4, a
	}
	for i := range r.background.Pixels {
		r.background.Pixels[i] = byte(i*53 + 71)
	}
	for i := range r.composeX {
		x := i % 128
		r.composeX[i] = sampleAxis{A: x * 4, B: min(x+1, 127) * 4, F: float64(i%17) / 17, Valid: i > 4}
	}
	for i := range r.composeY {
		y := i % 192
		r.composeY[i] = sampleAxis{A: y * 128 * 4, B: min(y+1, 191) * 128 * 4, F: float64(i%13) / 13, Valid: i > 7}
	}
	return r
}

func TestISPCCompositionPreservesScalarPixelsAndCancellation(t *testing.T) {
	r := compositionFixture()
	want, got := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	if err := r.composeScalar(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if err := r.compose(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want.Pix, got.Pix) {
		for i := range want.Pix {
			if want.Pix[i] != got.Pix[i] {
				t.Fatalf("composition byte %d: scalar=%d ISPC=%d", i, want.Pix[i], got.Pix[i])
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.compose(ctx, got); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestISPCCompositionPreservesOutputLayouts(t *testing.T) {
	for _, layout := range []struct {
		name   string
		stride int
		offset int
	}{
		{"padded", 720*4 + 4, 0},
		{"byte-stride", 720*4 + 1, 0},
		{"unaligned", 720 * 4, 1},
	} {
		t.Run(layout.name, func(t *testing.T) {
			r := compositionFixture()
			makeFrame := func() *image.RGBA {
				storage := bytes.Repeat([]byte{173}, layout.offset+layout.stride*960)
				return &image.RGBA{Pix: storage[layout.offset:], Stride: layout.stride, Rect: image.Rect(0, 0, 720, 960)}
			}
			want, got := makeFrame(), makeFrame()
			if err := r.composeScalar(context.Background(), want); err != nil {
				t.Fatal(err)
			}
			if err := r.compose(context.Background(), got); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want.Pix, got.Pix) {
				t.Fatal("composition changed output pixels or row padding")
			}
		})
	}
}

func BenchmarkExperimentalComposition(b *testing.B) {
	r := compositionFixture()
	dst := image.NewRGBA(image.Rect(0, 0, 720, 960))
	ctx := context.Background()
	for _, mode := range []string{"scalar", "ispc"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if mode == "scalar" {
					err = r.composeScalar(ctx, dst)
				} else {
					err = r.compose(ctx, dst)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
