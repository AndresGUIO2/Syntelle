package nativemedia

import (
	"errors"
	"image"
)

// Conversion uses the BT.601 studio-range matrix expected by OpenH264's
// default SPS. Go's image/color YCbCr matrix is full/JPEG range and cannot be
// handed to this encoder without setting matching H264 color metadata.
func validateI420(dst *image.YCbCr, src *image.RGBA) error {
	bounds := src.Bounds()
	if bounds.Min != (image.Point{}) || dst.Rect != bounds || bounds.Dx()%2 != 0 || bounds.Dy()%2 != 0 || dst.SubsampleRatio != image.YCbCrSubsampleRatio420 {
		return errors.New("expected matching even origin-zero I420 frame")
	}
	return nil
}

func scalarI420(dst *image.YCbCr, src *image.RGBA) error {
	if err := validateI420(dst, src); err != nil {
		return err
	}
	bounds := src.Bounds()
	for y := 0; y < bounds.Dy(); y += 2 {
		a, b := src.Pix[y*src.Stride:], src.Pix[(y+1)*src.Stride:]
		ya, yb := dst.Y[y*dst.YStride:], dst.Y[(y+1)*dst.YStride:]
		chroma := (y / 2) * dst.CStride
		for x := 0; x < bounds.Dx(); x += 2 {
			at := x * 4
			r0, g0, b0 := int(a[at]), int(a[at+1]), int(a[at+2])
			r1, g1, b1 := int(a[at+4]), int(a[at+5]), int(a[at+6])
			r2, g2, b2 := int(b[at]), int(b[at+1]), int(b[at+2])
			r3, g3, b3 := int(b[at+4]), int(b[at+5]), int(b[at+6])
			ya[x] = byte(16 + ((16829*r0 + 33039*g0 + 6416*b0 + 32768) >> 16))
			ya[x+1] = byte(16 + ((16829*r1 + 33039*g1 + 6416*b1 + 32768) >> 16))
			yb[x] = byte(16 + ((16829*r2 + 33039*g2 + 6416*b2 + 32768) >> 16))
			yb[x+1] = byte(16 + ((16829*r3 + 33039*g3 + 6416*b3 + 32768) >> 16))
			r, g, blue := r0+r1+r2+r3, g0+g1+g2+g3, b0+b1+b2+b3
			// Average chroma in the unrounded 2x2 RGB domain.
			dst.Cb[chroma+x/2] = byte(128 + ((-9714*r - 19071*g + 28784*blue + 131072) >> 18))
			dst.Cr[chroma+x/2] = byte(128 + ((28784*r - 24103*g - 4681*blue + 131072) >> 18))
		}
	}
	return nil
}
