//go:build integration && nvenc && nativegpu && nativecodecs && cgo && linux && amd64

package nativemedia

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"syntelle/render-worker/internal/nativerender"
	"testing"
)

func TestCUDAEncoderSurfaceEquivalence(t *testing.T) {
	if os.Getenv("SYNTELLE_GPU_CHECK") != "1" {
		t.Skip("explicit offline GPU qualification required")
	}
	t.Setenv("SYNTELLE_GPU_PIPELINE", "device")
	output := os.Getenv("SYNTELLE_SURFACE_OUTPUT")
	if output == "" {
		t.Fatal("new evidence directory required")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("evidence directory must be new")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	assets, err := nativerender.LoadCharacter(os.Getenv("SYNTELLE_NATIVE_ASSETS"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYNTELLE_RENDERER", "cpu")
	cpu, err := nativerender.New(assets)
	if err != nil {
		t.Fatal(err)
	}
	defer cpu.Close()
	t.Setenv("SYNTELLE_RENDERER", "gpu")
	gpu, err := nativerender.New(assets)
	if err != nil {
		t.Fatal(err)
	}
	defer gpu.Close()
	var states []struct {
		Name  string
		State nativerender.FrameState
	}
	raw, err := os.ReadFile(os.Getenv("SYNTELLE_FRAME_STATES"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &states); err != nil {
		t.Fatal(err)
	}
	reference := image.NewRGBA(image.Rect(0, 0, 1440, 960))
	borrowed := image.NewRGBA(reference.Rect)
	inspected := image.NewRGBA(reference.Rect)
	for i := range borrowed.Pix {
		borrowed.Pix[i] = 173
	}
	sentinel := bytes.Clone(borrowed.Pix)
	var report []map[string]any
	for _, entry := range states {
		if entry.Name != "mouth-Rest-A-0.5" && entry.Name != "eyes-half-blink" && entry.Name != "expression-smile" {
			continue
		}
		if err := cpu.Render(context.Background(), entry.State, reference); err != nil {
			t.Fatal(err)
		}
		direct, err := newNVENCVideo(1440, 960, 30, 2500000)
		if err != nil {
			t.Fatal(err)
		}
		v := direct.(*nvencVideo)
		upload, err := newNVENCVideo(1440, 960, 30, 2500000)
		if err != nil {
			t.Fatal(err)
		}
		surface, ok, err := v.Surface()
		if err != nil || !ok {
			t.Fatalf("surface: %v", err)
		}
		used, err := gpu.RenderFrame(context.Background(), entry.State, borrowed, &surface)
		if err != nil || !used {
			t.Fatalf("device render: %v", err)
		}
		if !bytes.Equal(borrowed.Pix, sentinel) {
			t.Fatal("device path wrote host RGBA")
		}
		if err := gpu.Readback(context.Background(), inspected); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(inspected.Pix, reference.Pix) {
			t.Fatal("raw device output differs")
		}
		planes, err := v.downloadSurface()
		if err != nil {
			t.Fatal(err)
		}
		expected := image.NewYCbCr(reference.Rect, image.YCbCrSubsampleRatio420)
		if err := limitedI420(expected, reference); err != nil {
			t.Fatal(err)
		}
		changed, maxDelta := 0, 0
		for _, p := range []struct {
			got, want                         []byte
			stride, wantStride, width, height int
		}{{planes.Y, expected.Y, planes.YStride, expected.YStride, 1440, 960}, {planes.Cb, expected.Cb, planes.CStride, expected.CStride, 720, 480}, {planes.Cr, expected.Cr, planes.CStride, expected.CStride, 720, 480}} {
			for y := 0; y < p.height; y++ {
				for x := 0; x < p.width; x++ {
					d := int(p.got[y*p.stride+x]) - int(p.want[y*p.wantStride+x])
					if d < 0 {
						d = -d
					}
					if d > 0 {
						changed++
					}
					maxDelta = max(maxDelta, d)
				}
			}
		}
		var a, b []byte
		for n := 0; n < 30; n++ {
			if n > 0 {
				surface, _, err = v.Surface()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := gpu.RenderFrame(context.Background(), entry.State, borrowed, &surface); err != nil {
					t.Fatal(err)
				}
			}
			data, err := v.EncodeSurface()
			if err != nil {
				t.Fatal(err)
			}
			a = append(a, data...)
			data, err = upload.Encode(reference)
			if err != nil {
				t.Fatal(err)
			}
			b = append(b, data...)
		}
		if err := direct.Close(); err != nil {
			t.Fatal(err)
		}
		if err := upload.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, entry.Name+"-device.h264"), a, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, entry.Name+"-upload.h264"), b, 0600); err != nil {
			t.Fatal(err)
		}
		report = append(report, map[string]any{"scene": entry.Name, "changed_i420_bytes": changed, "maximum_delta": maxDelta, "raw_rgba_exact": true, "host_frame_untouched": true, "encoded_bytes_exact": bytes.Equal(a, b)})
		if maxDelta > 1 {
			t.Errorf("GPU conversion delta %d", maxDelta)
		}
	}
	raw, err = json.MarshalIndent(map[string]any{"records": report, "paid_calls": 0}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "comparison.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
