package nativerender

import (
	"errors"
	"math"
)

func (r *Renderer) makeBackground(id string, blur float64) (*Texture, error) {
	source := r.assets.Backgrounds[id]
	if source == nil {
		return nil, errors.New("unknown native background")
	}
	blur = clamp(blur, 0, 20)
	margin := int(math.Ceil(blur * 3))
	w, h := 720+margin*2, 960+margin*2
	scale := math.Max(float64(w)/float64(source.Width), float64(h)/float64(source.Height))
	resized := NewTexture(w, h, false)
	for y := range h {
		for x := range w {
			sx := (float64(x)+.5-float64(w)/2)/scale + float64(source.Width)/2 - .5
			sy := (float64(y)+.5-float64(h)/2)/scale + float64(source.Height)/2 - .5
			resized.Set(x, y, source.Sample(sx, sy))
		}
	}
	out := NewTexture(720, 960, false)
	if blur == 0 {
		copy(out.Pixels, resized.Pixels)
		return out, nil
	}
	// Background work occurs only on configuration changes. A separable kernel
	// keeps blur out of the per-frame character and media path.
	kernel := make([]float64, margin*2+1)
	total := 0.
	for i := range kernel {
		x := float64(i - margin)
		kernel[i] = math.Exp(-x * x / (2 * blur * blur))
		total += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= total
	}
	temp := NewTexture(720, h, false)
	for y := range h {
		for x := range 720 {
			var c Color
			for k, v := range kernel {
				at := (y*w + x + k) * 4
				for channel := range 3 {
					c[channel] += float64(resized.Pixels[at+channel]) / 255 * v
				}
			}
			c[3] = 1
			temp.Set(x, y, c)
		}
	}
	for y := range 960 {
		for x := range 720 {
			var c Color
			for k, v := range kernel {
				at := ((y+k)*720 + x) * 4
				for channel := range 3 {
					c[channel] += float64(temp.Pixels[at+channel]) / 255 * v
				}
			}
			c[3] = 1
			out.Set(x, y, c)
		}
	}
	return out, nil
}
