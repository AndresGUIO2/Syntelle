//go:build integration && nativegpu && cgo && linux && amd64

package nativerender

import (
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMandibleCPUCUDAPacked720(t *testing.T) {
	if os.Getenv("SYNTELLE_GPU_CHECK") != "1" {
		t.Skip("explicit private GPU review")
	}
	a, err := Load(os.Getenv("SYNTELLE_NATIVE_ASSETS"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYNTELLE_GPU_PIPELINE", "device")
	t.Setenv("SYNTELLE_GPU_PRECISION", "fp32")
	t.Setenv("SYNTELLE_RENDERER", "cpu")
	cpu, err := NewSized(a, 540, 720)
	if err != nil {
		t.Fatal(err)
	}
	defer cpu.Close()
	t.Setenv("SYNTELLE_RENDERER", "gpu")
	gpu, err := NewSized(a, 540, 720)
	if err != nil {
		t.Fatal(err)
	}
	defer gpu.Close()
	data, err := os.ReadFile(os.Getenv("SYNTELLE_FRAME_STATES"))
	if err != nil {
		t.Fatal(err)
	}
	var states []struct {
		Name  string
		State FrameState
	}
	if err = json.Unmarshal(data, &states); err != nil {
		t.Fatal(err)
	}
	var report []map[string]any
	for _, entry := range states {
		one, two := image.NewRGBA(image.Rect(0, 0, 1080, 720)), image.NewRGBA(image.Rect(0, 0, 1080, 720))
		if err = cpu.Render(t.Context(), entry.State, one); err != nil {
			t.Fatal(err)
		}
		if err = gpu.Render(t.Context(), entry.State, two); err != nil {
			t.Fatal(err)
		}
		squared := 0.
		for i, v := range one.Pix {
			d := float64(int(v) - int(two.Pix[i]))
			squared += d * d
		}
		psnr := 10 * math.Log10(255*255/math.Max(1e-12, squared/float64(len(one.Pix))))
		report = append(report, map[string]any{"name": entry.Name, "psnr_db": psnr})
		if psnr < 45 {
			t.Errorf("%s PSNR %.3f", entry.Name, psnr)
		}
	}
	data, _ = json.MarshalIndent(report, "", "  ")
	if err = os.WriteFile(filepath.Join(os.Getenv("SYNTELLE_GPU_OUTPUT"), "packed720.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
