//go:build integration && nativegpu && cgo && linux && amd64

package nativerender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"syntelle/render-worker/internal/nativeframe"
	"testing"
	"time"
)

func TestCUDARecordedFramesAndFallback(t *testing.T) {
	if os.Getenv("SYNTELLE_GPU_CHECK") != "1" {
		t.Skip("explicit offline GPU qualification required")
	}
	t.Setenv("SYNTELLE_GPU_PIPELINE", "device")
	directory, statesPath, output := os.Getenv("SYNTELLE_NATIVE_ASSETS"), os.Getenv("SYNTELLE_FRAME_STATES"), os.Getenv("SYNTELLE_GPU_OUTPUT")
	if directory == "" || statesPath == "" || output == "" {
		t.Fatal("private assets, recorded states and new output required")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("output must be new")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(statesPath)
	if err != nil {
		t.Fatal(err)
	}
	var states []struct {
		Name  string
		State FrameState
	}
	if err := json.Unmarshal(data, &states); err != nil {
		t.Fatal(err)
	}
	if len(states) != 19 {
		t.Fatalf("expected 19 recorded states, got %d", len(states))
	}
	assets, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	var report []map[string]any
	for _, width := range []int{720, 1440} {
		cpu, err := newRenderer(assets, "cpu")
		if err != nil {
			t.Fatal(err)
		}
		gpu, err := newRenderer(assets, "gpu")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := cpu.Close(); err != nil {
				t.Error(err)
			}
			if err := gpu.Close(); err != nil {
				t.Error(err)
			}
		})
		for _, entry := range states {
			if entry.Name == "" || filepath.Base(entry.Name) != entry.Name {
				t.Fatal("invalid state name")
			}
			a, b := image.NewRGBA(image.Rect(0, 0, width, 960)), image.NewRGBA(image.Rect(0, 0, width, 960))
			start := time.Now()
			if err := cpu.Render(context.Background(), entry.State, a); err != nil {
				t.Fatal(err)
			}
			cpuTime := time.Since(start)
			start = time.Now()
			if err := gpu.Render(context.Background(), entry.State, b); err != nil {
				t.Fatal(err)
			}
			gpuTime := time.Since(start)
			var face, all float64
			maxDelta, changed := 0, 0
			for y := 0; y < 960; y++ {
				for x := 0; x < width; x++ {
					for c := 0; c < 4; c++ {
						i := y*a.Stride + x*4 + c
						d := int(a.Pix[i]) - int(b.Pix[i])
						if d < 0 {
							d = -d
						}
						maxDelta = max(maxDelta, d)
						if d > 0 {
							changed++
						}
						all += float64(d * d)
						if x >= 180 && x < 540 && y >= 180 && y < 680 && c < 3 {
							face += float64(d * d)
						}
					}
				}
			}
			psnr := func(mse float64, n int) float64 { return 10 * math.Log10(255*255/math.Max(mse/float64(n), 1e-12)) }
			faceDB, allDB := psnr(face, 360*500*3), psnr(all, width*960*4)
			for _, item := range []struct {
				mode  string
				frame *image.RGBA
			}{{"cpu", a}, {"gpu", b}} {
				f, err := os.Create(filepath.Join(output, entry.Name+"-"+item.mode+"-"+fmtWidth(width)+".png"))
				if err != nil {
					t.Fatal(err)
				}
				encodeErr := png.Encode(f, item.frame)
				closeErr := f.Close()
				if err := errors.Join(encodeErr, closeErr); err != nil {
					t.Fatal(err)
				}
			}
			report = append(report, map[string]any{"name": entry.Name, "width": width, "face_psnr_db": faceDB, "all_rgba_psnr_db": allDB, "maximum_byte_delta": maxDelta, "changed_bytes": changed, "cpu_ms": float64(cpuTime) / float64(time.Millisecond), "gpu_ms": float64(gpuTime) / float64(time.Millisecond)})
			if faceDB < 45 || allDB < 45 {
				t.Errorf("%s/%d quality failed face %.3f all %.3f", entry.Name, width, faceDB, allDB)
			}
		}
		// Simulate loss after real CUDA output. Auto mode must completely replace
		// a partially written GPU result with the current CPU state.
		gpu.gpu = &failedGPU{underlying: gpu.gpu}
		gpu.gpuRequired = false
		s := states[3].State
		s.Closure = .47
		s.Amount = .29
		a, b := image.NewRGBA(image.Rect(0, 0, width, 960)), image.NewRGBA(image.Rect(0, 0, width, 960))
		if err := cpu.Render(context.Background(), s, a); err != nil {
			t.Fatal(err)
		}
		if err := gpu.Render(context.Background(), s, b); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.Pix, b.Pix) || gpu.gpu != nil {
			t.Fatal("GPU failure did not reproduce the exact current CPU frame")
		}
		// The full-device path never prepared CPU patches. A failed device call
		// must rebuild them, rather than accepting stale CPU cache metadata.
		full, err := newRenderer(assets, "gpu")
		if err != nil {
			t.Fatal(err)
		}
		if err := full.Render(context.Background(), states[1].State, b); err != nil {
			t.Fatal(err)
		}
		full.gpu = &failedFullGPU{failedGPU{underlying: full.gpu}}
		full.gpuRequired = false
		if err := full.Render(context.Background(), s, b); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.Pix, b.Pix) || full.gpu != nil {
			t.Fatal("device fallback retained stale patches")
		}
		if err := full.Close(); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := json.MarshalIndent(map[string]any{"states": report, "paid_calls": 0, "fallback_exact": true}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "comparison.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
}

func fmtWidth(w int) string {
	if w == 720 {
		return "720"
	}
	return "1440"
}

type failedGPU struct{ underlying gpuRenderer }

func (g *failedGPU) Render(_ context.Context, _ *Renderer, _ FrameState, dst *image.RGBA, _ bool, _ bool, _ bool) error {
	for i := range dst.Pix {
		dst.Pix[i] = 173
	}
	return errors.New("injected GPU device loss")
}
func (g *failedGPU) Close() error { return g.underlying.Close() }

type failedFullGPU struct{ failedGPU }

func (g *failedFullGPU) RenderAll(ctx context.Context, r *Renderer, s FrameState, dst *image.RGBA, _ *nativeframe.Surface) error {
	return g.Render(ctx, r, s, dst, false, false, false)
}
func (g *failedFullGPU) Readback(*image.RGBA) error { return errors.New("injected device loss") }
