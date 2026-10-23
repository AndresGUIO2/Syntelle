package nativerender

import (
	"bytes"
	"context"
	"image"
	"os"
	"testing"
)

// referenceCompose freezes the preceding two-stage bilinear composition. It
// deliberately retains the former indexing so tests cover the new texel views.
func referenceCompose(r *Renderer, dst *image.RGBA) {
	pixels := r.body.Pixels
	for y := range 960 {
		yaxis := r.composeY[y]
		for x := range 720 {
			i, bg := y*dst.Stride+x*4, (y*720+x)*4
			xaxis := r.composeX[x]
			if !yaxis.Valid || !xaxis.Valid {
				copy(dst.Pix[i:i+4], r.background.Pixels[bg:bg+4])
				continue
			}
			a, b, c, d := yaxis.A+xaxis.A, yaxis.A+xaxis.B, yaxis.B+xaxis.A, yaxis.B+xaxis.B
			if pixels[a+3]|pixels[b+3]|pixels[c+3]|pixels[d+3] == 0 {
				copy(dst.Pix[i:i+4], r.background.Pixels[bg:bg+4])
				continue
			}
			remaining := 0.
			if pixels[a+3]&pixels[b+3]&pixels[c+3]&pixels[d+3] != 255 {
				top := mix(float64(pixels[a+3]), float64(pixels[b+3]), xaxis.F)
				bottom := mix(float64(pixels[c+3]), float64(pixels[d+3]), xaxis.F)
				remaining = 1 - mix(top, bottom, yaxis.F)/255
			}
			for k := range 3 {
				top := mix(float64(pixels[a+k]), float64(pixels[b+k]), xaxis.F)
				bottom := mix(float64(pixels[c+k]), float64(pixels[d+k]), xaxis.F)
				v := mix(top, bottom, yaxis.F) + float64(r.background.Pixels[bg+k])*remaining
				dst.Pix[i+k] = byte(min(255., v+.5))
			}
			dst.Pix[i+3] = 255
		}
	}
}

func TestCompositionPreservesBilinearPremultipliedBytes(t *testing.T) {
	t.Parallel()
	r := rasterFixture()
	r.background = patchSolid(720, 960, [4]byte{37, 51, 65, 255})
	copy(r.body.Pixels, r.subjectTexture.Pixels)
	for x := range 720 {
		a := x * 31 / 720
		r.composeX[x] = sampleAxis{A: a * 4, B: min(a+1, 31) * 4, F: float64(x%13) / 13, Valid: x > 13 && x < 706}
	}
	for y := range 960 {
		a := y * 31 / 960
		r.composeY[y] = sampleAxis{A: a * 32 * 4, B: min(a+1, 31) * 32 * 4, F: float64(y%17) / 17, Valid: y > 7 && y < 948}
	}
	want, actual := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	referenceCompose(r, want)
	if err := r.compose(context.Background(), actual); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want.Pix, actual.Pix) {
		for i, v := range actual.Pix {
			if v != want.Pix[i] {
				t.Fatalf("composition byte %d: got %d, want %d", i, v, want.Pix[i])
			}
		}
	}
}

func BenchmarkPreparedComposition(b *testing.B) {
	directory := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	if directory == "" {
		b.Skip("set SYNTELLE_NATIVE_ASSETS to a prepared native package")
	}
	assets, err := Load(directory)
	if err != nil {
		b.Fatal(err)
	}
	r, err := New(assets)
	if err != nil {
		b.Fatal(err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, 720, 960))
	ctx := context.Background()
	if err := r.Render(ctx, FrameState{Head: [3]float64{1.1, -.4, .1}, Breath: 1.3, Background: "dark-lounge"}, frame); err != nil {
		b.Fatal(err)
	}
	for _, original := range []bool{true, false} {
		name := "bounded-texels"
		if original {
			name = "preceding-formula"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if original {
					referenceCompose(r, frame)
				} else if err := r.compose(ctx, frame); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
