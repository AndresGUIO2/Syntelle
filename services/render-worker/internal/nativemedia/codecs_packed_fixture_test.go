//go:build integration && nativecodecs && cgo

package nativemedia

import (
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPackedNVENCRecordedQuality(t *testing.T) {
	if os.Getenv("SYNTELLE_GPU_CHECK") != "1" {
		t.Skip("explicit offline hardware check required")
	}
	input, output := os.Getenv("SYNTELLE_PACKED_INPUT"), os.Getenv("SYNTELLE_PACKED_OUTPUT")
	if input == "" || output == "" {
		t.Fatal("explicit packed fixtures and new output required")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("output must be new")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	for _, name := range []string{"mouth-Rest-A-0.5", "eyes-half-blink", "expression-smile"} {
		file, err := os.Open(filepath.Join(input, name+"-cpu-1440.png"))
		if err != nil {
			t.Fatal(err)
		}
		picture, err := png.Decode(file)
		closed := file.Close()
		if err := errors.Join(err, closed); err != nil {
			t.Fatal(err)
		}
		if picture.Bounds() != image.Rect(0, 0, 1440, 960) {
			t.Fatal("expected packed frame")
		}
		source := image.NewRGBA(picture.Bounds())
		draw.Draw(source, source.Rect, picture, source.Rect.Min, draw.Src)
		for _, mode := range []string{"openh264", "nvenc"} {
			encoder, err := (nativeFactory{videoMode: mode}).Video(1440, 960, 30, 2500000)
			if err != nil {
				t.Fatal(err)
			}
			var clip []byte
			for frame := 0; frame < 30; frame++ {
				unit, err := encoder.Encode(source)
				if err != nil {
					t.Fatal(err)
				}
				if frame == 0 {
					sps := nativeNAL(unit, 7)
					if len(sps) < 4 || sps[1] != 66 || sps[3] != 40 {
						t.Fatalf("packed SDP profile mismatch: %x", sps)
					}
					full, colors, _ := inspectSPSColor(t, sps)
					if full || colors != h264Color {
						t.Fatal("packed color contract mismatch")
					}
				}
				if len(unit) == 0 {
					t.Fatal("encoder delayed a frame")
				}
				clip = append(clip, unit...)
			}
			if err := encoder.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(output, name+"-"+mode+".h264"), clip, 0600); err != nil {
				t.Fatal(err)
			}
			records = append(records, map[string]any{"scene": name, "encoder": mode, "bytes": len(clip), "frames": 30})
		}
	}
	data, err := json.MarshalIndent(map[string]any{"records": records, "paid_calls": 0}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "encoding.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
