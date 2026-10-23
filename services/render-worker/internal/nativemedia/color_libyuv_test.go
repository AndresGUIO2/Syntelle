//go:build libyuv && cgo

package nativemedia

import (
	"bytes"
	"image"
	"testing"
)

func TestLibyuvRejectsInvalidNativeBuffers(t *testing.T) {
	for _, name := range []string{"source", "luma", "cb", "cr", "source-stride", "chroma-stride", "native-stride"} {
		t.Run(name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, 4, 4))
			dst := image.NewYCbCr(src.Rect, image.YCbCrSubsampleRatio420)
			switch name {
			case "source":
				src.Pix = src.Pix[:len(src.Pix)-1]
			case "luma":
				dst.Y = dst.Y[:len(dst.Y)-1]
			case "cb":
				dst.Cb = dst.Cb[:len(dst.Cb)-1]
			case "cr":
				dst.Cr = dst.Cr[:len(dst.Cr)-1]
			case "source-stride":
				src.Stride = -1
			case "chroma-stride":
				dst.CStride = 1
			case "native-stride":
				src.Stride = 1 << 32
			}
			if err := limitedI420(dst, src); err == nil {
				t.Fatal("invalid buffer entered native conversion")
			}
		})
	}
}

func TestLibyuvPreservesSourceAndPlanePadding(t *testing.T) {
	src := &image.RGBA{Pix: bytes.Repeat([]byte{91}, 21*4), Stride: 21, Rect: image.Rect(0, 0, 4, 4)}
	dst := &image.YCbCr{Y: bytes.Repeat([]byte{173}, 7*4), Cb: bytes.Repeat([]byte{173}, 5*2),
		Cr: bytes.Repeat([]byte{173}, 5*2), YStride: 7, CStride: 5,
		SubsampleRatio: image.YCbCrSubsampleRatio420, Rect: src.Rect}
	before := bytes.Clone(src.Pix)
	if err := limitedI420(dst, src); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src.Pix, before) {
		t.Fatal("conversion modified the source")
	}
	for _, plane := range []struct {
		data                  []byte
		stride, width, height int
	}{
		{dst.Y, dst.YStride, 4, 4}, {dst.Cb, dst.CStride, 2, 2}, {dst.Cr, dst.CStride, 2, 2},
	} {
		for y := 0; y < plane.height; y++ {
			for _, value := range plane.data[y*plane.stride+plane.width : (y+1)*plane.stride] {
				if value != 173 {
					t.Fatal("conversion modified plane padding")
				}
			}
		}
	}
	if err := limitedI420(image.NewYCbCr(image.Rectangle{}, image.YCbCrSubsampleRatio420), image.NewRGBA(image.Rectangle{})); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkExperimentalI420(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 720, 960))
	dst := image.NewYCbCr(src.Rect, image.YCbCrSubsampleRatio420)
	for i := range src.Pix {
		src.Pix[i] = byte(i*53 + 71)
	}
	for _, mode := range []string{"scalar", "libyuv"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if mode == "scalar" {
					err = scalarI420(dst, src)
				} else {
					err = limitedI420(dst, src)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
