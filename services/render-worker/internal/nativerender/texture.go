// Package nativerender renders the prepared character directly into reusable CPU
// buffers. Textures are source-bound immutable inputs; render targets are owned
// by a single session.
package nativerender

import "math"

type Color [4]float64

type Texture struct {
	Width, Height int
	Pixels        []byte
	Premultiplied bool
}

func NewTexture(width, height int, premultiplied bool) *Texture {
	return &Texture{Width: width, Height: height, Pixels: make([]byte, width*height*4), Premultiplied: premultiplied}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func mix(a, b, t float64) float64     { return a + (b-a)*t }
func byteColor(v float64) byte {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return byte(v*255 + .5)
}

func (t *Texture) Set(x, y int, c Color) {
	i := (y*t.Width + x) * 4
	t.Pixels[i], t.Pixels[i+1], t.Pixels[i+2], t.Pixels[i+3] = byteColor(c[0]), byteColor(c[1]), byteColor(c[2]), byteColor(c[3])
}

// Sample uses the same clamp-to-edge, bilinear texel convention as the existing
// shader. Coordinates refer to texel centers (the first center is 0,0).
func (t *Texture) Sample(x, y float64) Color {
	if x < 0 {
		x = 0
	} else if x > float64(t.Width-1) {
		x = float64(t.Width - 1)
	}
	if y < 0 {
		y = 0
	} else if y > float64(t.Height-1) {
		y = float64(t.Height - 1)
	}
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, t.Width-1), min(y0+1, t.Height-1)
	fx, fy := x-float64(x0), y-float64(y0)
	a, b, c, d := (y0*t.Width+x0)*4, (y0*t.Width+x1)*4, (y1*t.Width+x0)*4, (y1*t.Width+x1)*4
	var result Color
	for k := range 4 {
		top := float64(t.Pixels[a+k]) + (float64(t.Pixels[b+k])-float64(t.Pixels[a+k]))*fx
		bottom := float64(t.Pixels[c+k]) + (float64(t.Pixels[d+k])-float64(t.Pixels[c+k]))*fx
		result[k] = (top + (bottom-top)*fy) / 255
	}
	return result
}

func (t *Texture) Premultiply() *Texture {
	out := NewTexture(t.Width, t.Height, true)
	copy(out.Pixels, t.Pixels)
	if !t.Premultiplied {
		for i := 0; i < len(out.Pixels); i += 4 {
			a := uint32(out.Pixels[i+3])
			for k := range 3 {
				out.Pixels[i+k] = byte((uint32(out.Pixels[i+k])*a + 127) / 255)
			}
		}
	}
	return out
}
