package nativemedia

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// The scalar reference retains its exact historical coefficients. Optimized
// converters are checked against the independent display contract separately.
func TestScalarI420PreservesKnownDisplayColors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		rgb       color.RGBA
		y, cb, cr byte
	}{
		{name: "black", rgb: color.RGBA{0, 0, 0, 255}, y: 16, cb: 128, cr: 128},
		{name: "white", rgb: color.RGBA{255, 255, 255, 255}, y: 235, cb: 128, cr: 128},
		{name: "neutral gray", rgb: color.RGBA{128, 128, 128, 255}, y: 126, cb: 128, cr: 128},
		{name: "red", rgb: color.RGBA{255, 0, 0, 255}, y: 81, cb: 90, cr: 240},
		{name: "skin", rgb: color.RGBA{218, 176, 154, 255}, y: 176, cb: 112, cr: 148},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, 2, 2))
			for y := 0; y < 2; y++ {
				for x := 0; x < 2; x++ {
					src.SetRGBA(x, y, tt.rgb)
				}
			}
			dst := image.NewYCbCr(src.Bounds(), image.YCbCrSubsampleRatio420)
			if err := scalarI420(dst, src); err != nil {
				t.Fatal(err)
			}
			if dst.Y[0] != tt.y || dst.Cb[0] != tt.cb || dst.Cr[0] != tt.cr {
				t.Fatalf("studio matrix got YCbCr=%d,%d,%d want %d,%d,%d", dst.Y[0], dst.Cb[0], dst.Cr[0], tt.y, tt.cb, tt.cr)
			}
			// Independent limited-range display inverse catches the former contrast
			// expansion of full-range encoder input (black->0, white->255).
			y, cb, cr := float64(dst.Y[0])-16, float64(dst.Cb[0])-128, float64(dst.Cr[0])-128
			rgb := [3]float64{1.16438356*y + 1.59602678*cr, 1.16438356*y - .39176229*cb - .81296765*cr, 1.16438356*y + 2.01723214*cb}
			want := [3]byte{tt.rgb.R, tt.rgb.G, tt.rgb.B}
			for i, value := range rgb {
				if math.Abs(math.Max(0, math.Min(255, value))-float64(want[i])) > 1.5 {
					t.Fatalf("display channel %d changed: %.3f vs %d", i, value, want[i])
				}
			}
		})
	}
}

func TestScalarI420AveragesChromaWithoutChangingLuma(t *testing.T) {
	t.Parallel()
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	src.SetRGBA(0, 1, color.RGBA{0, 0, 255, 255})
	src.SetRGBA(1, 1, color.RGBA{255, 255, 255, 255})
	dst := image.NewYCbCr(src.Bounds(), image.YCbCrSubsampleRatio420)
	if err := scalarI420(dst, src); err != nil {
		t.Fatal(err)
	}
	if dst.Cb[0] != 128 || dst.Cr[0] != 128 || dst.Y[0] != 81 || dst.Y[1] != 145 || dst.Y[2] != 41 || dst.Y[3] != 235 {
		t.Fatalf("unexpected independent luma/chroma: %v %v %v", dst.Y, dst.Cb, dst.Cr)
	}
}

// The selected backend must preserve BT.601 studio range and stay within two
// quantization levels of a floating-point matrix, including 2x2 chroma means.
// This bound is explicit; it does not replace the scalar's exact-byte tests.
func TestLimitedI420MatchesStudioMatrix(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"color-grid", "mixed-chroma"} {
		t.Run(name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, 256, 256))
			for y := 0; y < 256; y++ {
				for x := 0; x < 256; x++ {
					px, py := x/2, y/2
					if name == "mixed-chroma" {
						px, py = x, y
					}
					src.SetRGBA(x, y, color.RGBA{byte(px * 17), byte(py * 29), byte(px*53 + py*71), 255})
				}
			}
			dst := image.NewYCbCr(src.Rect, image.YCbCrSubsampleRatio420)
			if err := limitedI420(dst, src); err != nil {
				t.Fatal(err)
			}
			for y := 0; y < 256; y += 2 {
				for x := 0; x < 256; x += 2 {
					var r, g, b float64
					for dy := 0; dy < 2; dy++ {
						for dx := 0; dx < 2; dx++ {
							p := src.RGBAAt(x+dx, y+dy)
							luma := 16 + 219.0/255*(.299*float64(p.R)+.587*float64(p.G)+.114*float64(p.B))
							got := dst.Y[(y+dy)*dst.YStride+x+dx]
							if got < 16 || got > 235 || math.Abs(float64(got)-luma) > 2 {
								t.Fatalf("luma at %d,%d: got %d want %.4f", x+dx, y+dy, got, luma)
							}
							r, g, b = r+float64(p.R)/4, g+float64(p.G)/4, b+float64(p.B)/4
						}
					}
					want := [2]float64{128 + 224.0/255*(-.168735892*r-.331264108*g+.5*b),
						128 + 224.0/255*(.5*r-.418687589*g-.081312411*b)}
					at := y/2*dst.CStride + x/2
					for channel, got := range []byte{dst.Cb[at], dst.Cr[at]} {
						if got < 16 || got > 240 || math.Abs(float64(got)-want[channel]) > 2 {
							t.Fatalf("chroma %d at %d,%d: got %d want %.4f", channel, x, y, got, want[channel])
						}
					}
				}
			}
		})
	}
}

func TestLimitedI420PreservesNeutralEndpoints(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		input, y byte
	}{
		{"black", 0, 16}, {"white", 255, 235},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, 2, 2))
			for y := 0; y < 2; y++ {
				for x := 0; x < 2; x++ {
					src.SetRGBA(x, y, color.RGBA{tt.input, tt.input, tt.input, 255})
				}
			}
			dst := image.NewYCbCr(src.Rect, image.YCbCrSubsampleRatio420)
			if err := limitedI420(dst, src); err != nil {
				t.Fatal(err)
			}
			if dst.Y[0] != tt.y || dst.Cb[0] != 128 || dst.Cr[0] != 128 {
				t.Fatalf("studio endpoint: %v %v %v", dst.Y, dst.Cb, dst.Cr)
			}
		})
	}
}
