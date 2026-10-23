//go:build integration && nativecodecs && cgo

package nativemedia

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This explicit offline test accepts only a bounded local PNG corpus. It has no
// provider, database or network dependency and uses the production H264 settings.
func TestI420RecordedFrameFixtures(t *testing.T) {
	input, output := os.Getenv("SYNTELLE_COLOR_FIXTURES"), os.Getenv("SYNTELLE_COLOR_OUTPUT")
	if input == "" || output == "" {
		t.Fatal("explicit local fixture and new output directories required")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("fixture output must not already exist")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(input, "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 || len(files) > 96 {
		t.Fatal("bounded PNG fixture set required")
	}
	var records []map[string]any
	for _, path := range files {
		if strings.HasSuffix(path, "-mouth.png") || strings.HasSuffix(path, "-eyes.png") {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".png")
		t.Run(name, func(t *testing.T) {
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			picture, err := png.Decode(file)
			closeErr := file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			if picture.Bounds().Dx() != 720 || picture.Bounds().Dy() != 960 {
				t.Fatal("expected fixed 720x960 scene")
			}
			src := image.NewRGBA(image.Rect(0, 0, 720, 960))
			draw.Draw(src, src.Rect, picture, picture.Bounds().Min, draw.Src)
			for _, converter := range []struct {
				name    string
				convert func(*image.YCbCr, *image.RGBA) error
			}{{"scalar", scalarI420}, {"selected", limitedI420}} {
				dst := image.NewYCbCr(src.Rect, image.YCbCrSubsampleRatio420)
				if err := converter.convert(dst, src); err != nil {
					t.Fatal(err)
				}
				data := make([]byte, 0, len(dst.Y)+len(dst.Cb)+len(dst.Cr))
				data = append(data, dst.Y...)
				data = append(data, dst.Cb...)
				data = append(data, dst.Cr...)
				if err := os.WriteFile(filepath.Join(output, name+"-"+converter.name+".i420"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			encoder, err := (nativeFactory{}).Video(720, 960, 30, 2500000)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := encoder.Close(); err != nil {
					t.Error(err)
				}
			}()
			encoded, err := os.Create(filepath.Join(output, name+".h264"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := encoded.Close(); err != nil {
					t.Error(err)
				}
			}()
			for frame := 0; frame < 30; frame++ {
				data, err := encoder.Encode(src)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := encoded.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			hash := sha256.Sum256(src.Pix)
			records = append(records, map[string]any{"name": name, "rgba_sha256": hex.EncodeToString(hash[:]), "encoded_frames": 30})
		})
	}
	if len(records) != 19 {
		t.Fatalf("got %d fixed scenes; want 19", len(records))
	}
	data, err := json.MarshalIndent(map[string]any{"frames": records, "size": []int{720, 960}, "fps": 30, "bitrate": 2500000, "paid_calls": 0}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
