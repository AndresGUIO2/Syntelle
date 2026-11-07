//go:build integration && nativegpu && cgo && linux && amd64

package nativerender

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestContextualArticulationRenderedParityAndCache(t *testing.T) {
	path := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	if path == "" {
		t.Skip("explicit cached package required")
	}
	assets, err := LoadCharacter(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYNTELLE_RENDERER", "cpu")
	cpu, err := NewSized(assets, 540, 720)
	if err != nil {
		t.Fatal(err)
	}
	defer cpu.Close()
	t.Setenv("SYNTELLE_RENDERER", "gpu")
	gpu, err := NewSized(assets, 540, 720)
	if err != nil {
		t.Fatal(err)
	}
	defer gpu.Close()
	states := []FrameState{{From: "EI", To: "Rest"}, {From: "EI", To: "Rest", LipResidual: [2]float64{.03, -.025}}, {From: "EI", To: "Rest", LipResidual: [2]float64{-.02, .03}}, {From: "MBP", To: "FV", Amount: .5}, {From: "A", To: "EI", Amount: .4, Head: [3]float64{8, 4, 1.2}, LipResidual: [2]float64{.02, .02}}, {From: "A", To: "EI", Amount: .7, Head: [3]float64{-8, -4, -1.2}, LipResidual: [2]float64{-.02, -.02}}}
	var previous []byte
	for i, state := range states {
		a, b := image.NewRGBA(image.Rect(0, 0, 1080, 720)), image.NewRGBA(image.Rect(0, 0, 1080, 720))
		if err := cpu.Render(t.Context(), state, a); err != nil {
			t.Fatal(err)
		}
		if err := gpu.Render(t.Context(), state, b); err != nil {
			t.Fatal(err)
		}
		if i == 1 && bytes.Equal(previous, a.Pix) {
			t.Fatal("residual mouth frame incorrectly reused")
		}
		previous = append(previous[:0], a.Pix...)
		squared := 0.
		for j, v := range a.Pix {
			d := float64(int(v) - int(b.Pix[j]))
			squared += d * d
		}
		psnr := 10 * math.Log10(255*255/math.Max(1e-12, squared/float64(len(a.Pix))))
		if psnr < 45 {
			t.Fatal("CPU/CUDA mismatch", i, psnr)
		}
		t.Log("state", i, "PSNR", psnr)
		if dir := os.Getenv("SYNTELLE_GPU_OUTPUT"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, string(rune('0'+i))+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(f, b); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
