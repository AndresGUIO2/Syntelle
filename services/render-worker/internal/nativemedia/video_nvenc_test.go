//go:build nvenc && nativecodecs && cgo && linux && amd64

package nativemedia

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Explicit opt-in never skips missing hardware: this is also the qualification
// command for a Linux NVIDIA runtime. It uses prepared frames, no providers.
func TestNVENCOfflineComparison(t *testing.T) {
	if os.Getenv("SYNTELLE_NVENC_CHECK") != "1" {
		t.Skip("explicit GPU fixture opt-in required")
	}
	input, output := os.Getenv("SYNTELLE_COLOR_FIXTURES"), os.Getenv("SYNTELLE_COLOR_OUTPUT")
	if input == "" || output == "" {
		t.Fatal("local fixtures and a new output directory required")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("output must be new")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	for _, name := range []string{"mouth-Rest-A-0.5", "eyes-half-blink", "head-positive"} {
		file, err := os.Open(filepath.Join(input, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		picture, err := png.Decode(file)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatal(errors.Join(err, closeErr))
		}
		src := image.NewRGBA(picture.Bounds())
		draw.Draw(src, src.Rect, picture, picture.Bounds().Min, draw.Src)
		for _, mode := range []string{"openh264", "nvenc", "switch"} {
			var encoder VideoEncoder
			if mode == "switch" {
				encoder, err = selectVideoEncoder("auto", func() (VideoEncoder, error) {
					v, err := newNVENCVideo(720, 960, 30, 2500000)
					if err != nil {
						return nil, err
					}
					return &loseGPUAfterTen{VideoEncoder: v}, nil
				}, func() (VideoEncoder, error) { return newOpenH264Video(720, 960, 30, 2500000) })
			} else {
				encoder, err = (nativeFactory{videoMode: mode}).Video(720, 960, 30, 2500000)
			}
			if err != nil {
				t.Fatal(err)
			}
			var clip []byte
			var elapsed time.Duration
			var first, retained []byte
			for i := range 30 {
				if i == 15 {
					if err := encoder.ForceKeyFrame(); err != nil {
						t.Fatal(err)
					}
				}
				start := time.Now()
				unit, err := encoder.Encode(src)
				elapsed += time.Since(start)
				if err != nil {
					t.Fatal(err)
				}
				if len(unit) == 0 {
					t.Fatal("encoder buffered a frame")
				}
				if i == 0 || i == 15 || (mode == "switch" && i == 10) {
					if len(nativeNAL(unit, 5)) == 0 || len(nativeNAL(unit, 7)) == 0 || len(nativeNAL(unit, 8)) == 0 {
						t.Fatal("IDR missing parameter sets")
					}
					if sps := nativeNAL(unit, 7); sps[1] != 66 || sps[3] != 31 {
						t.Fatalf("SDP baseline/3.1 mismatch: %x", sps[:4])
					}
					full, colors, _ := inspectSPSColor(t, nativeNAL(unit, 7))
					if full || colors != h264Color {
						t.Fatalf("wrong color contract: %v %v", full, colors)
					}
				}
				if i == 0 {
					first, retained = bytes.Clone(unit), unit
				}
				clip = append(clip, unit...)
			}
			if err := encoder.Close(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, retained) {
				t.Fatal("encoder mutated retained access unit")
			}
			if err := os.WriteFile(filepath.Join(output, name+"-"+mode+".h264"), clip, 0600); err != nil {
				t.Fatal(err)
			}
			records = append(records, map[string]any{"scene": name, "encoder": mode, "frames": 30, "bytes": len(clip), "encode_ms_per_frame": float64(elapsed) / float64(time.Millisecond) / 30})
		}
	}
	data, err := json.MarshalIndent(map[string]any{"records": records, "paid_calls": 0, "width": 720, "height": 960, "fps": 30, "bitrate": 2500000}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "encoding.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

type loseGPUAfterTen struct {
	VideoEncoder
	count int
}

func (v *loseGPUAfterTen) Encode(frame *image.RGBA) ([]byte, error) {
	v.count++
	if v.count == 11 {
		return nil, errors.New("offline injected device loss")
	}
	return v.VideoEncoder.Encode(frame)
}
