package nativerender

import (
	"context"
	"image"
)

// packForeground transports a premultiplied character and its full-size
// grayscale coverage side by side in one encoded frame. Both halves therefore
// share decoding, RTP timing, loss and reconnection; no background is sampled.
// The original opaque compositor remains available for offline comparisons.
func (r *Renderer) packForeground(ctx context.Context, dst *image.RGBA) error {
	width, height := r.displaySize()
	for y := range height {
		if y%32 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		ya := r.composeY[y]
		for x := range width {
			i := y*dst.Stride + x*4
			mask := i + width*4
			var sample [4]byte
			xa := r.composeX[x]
			if xa.Valid && ya.Valid {
				a, b, c, d := ya.A+xa.A, ya.A+xa.B, ya.B+xa.A, ya.B+xa.B
				pixels := r.body.Pixels
				pa, pb, pc, pd := pixels[a:a+4], pixels[b:b+4], pixels[c:c+4], pixels[d:d+4]
				if pa[3]|pb[3]|pc[3]|pd[3] != 0 {
					for k := range 3 {
						top := mix(float64(pa[k]), float64(pb[k]), xa.F)
						bottom := mix(float64(pc[k]), float64(pd[k]), xa.F)
						sample[k] = byte(mix(top, bottom, ya.F) + .5)
					}
					if pa[3]&pb[3]&pc[3]&pd[3] == 255 {
						sample[3] = 255
					} else {
						top := mix(float64(pa[3]), float64(pb[3]), xa.F)
						bottom := mix(float64(pc[3]), float64(pd[3]), xa.F)
						sample[3] = byte(mix(top, bottom, ya.F) + .5)
					}
				}
			}
			copy(dst.Pix[i:i+3], sample[:3])
			dst.Pix[i+3] = 255
			dst.Pix[mask], dst.Pix[mask+1], dst.Pix[mask+2], dst.Pix[mask+3] = sample[3], sample[3], sample[3], 255
		}
	}
	return nil
}

// packColorOnly is restricted to the offline diagnostic receiver.
func (r *Renderer) packColorOnly(ctx context.Context, dst *image.RGBA) error {
	width, height := r.displaySize()
	packed := image.NewRGBA(image.Rect(0, 0, width*2, height))
	if err := r.packForeground(ctx, packed); err != nil {
		return err
	}
	for y := range height {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+width*4], packed.Pix[y*packed.Stride:y*packed.Stride+width*4])
	}
	return nil
}
