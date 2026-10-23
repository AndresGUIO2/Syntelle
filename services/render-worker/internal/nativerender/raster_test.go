package nativerender

import (
	"bytes"
	"math"
	"testing"
)

func rasterFixture() *Renderer {
	const size = 32
	subject := NewTexture(size, size, true)
	for y := range size {
		for x := range size {
			a := 255
			if x < 5 || x > 26 {
				a = 32 + 28*min(x, size-1-x)
			}
			at := (y*size + x) * 4
			subject.Pixels[at] = byte(x * 7 * a / 255)
			subject.Pixels[at+1] = byte(y * 7 * a / 255)
			subject.Pixels[at+2] = byte((x + y) * 3 * a / 255)
			subject.Pixels[at+3] = byte(a)
		}
	}
	r := &Renderer{assets: &Assets{Motion: Motion{Canvas: Size{size, size}}}, body: NewTexture(size, size, true), subjectTexture: subject, depth: make([]float64, size*size), right: size, bottom: size}
	for i := range r.depth {
		r.depth[i] = math.Inf(1)
	}
	return r
}

func TestRasterSamplingPreservesCoverageAndPremultipliedEdges(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		vertices []rasterVertex
	}{
		{"shared-diagonal", []rasterVertex{{X: .5, Y: .5, Z: .5, U: 0, V: 0, Q: 1}, {X: 31.5, Y: .5, Z: .5, U: 1, V: 0, Q: 1}, {X: 31.5, Y: 31.5, Z: .5, U: 1, V: 1, Q: 1}, {X: .5, Y: 31.5, Z: .5, U: 0, V: 1, Q: 1}}},
		{"fractional-perspective", []rasterVertex{{X: -3.124, Y: 1.149, Z: .42, U: 0, V: 0, Q: .85}, {X: 29.319, Y: -2.993, Z: .57, U: 1.1, V: 0, Q: 1.1}, {X: 34.017, Y: 28.613, Z: .63, U: 1, V: 1, Q: 1.03}, {X: 2.187, Y: 33.514, Z: .51, U: 0, V: 1, Q: .93}}},
		{"near-pixel-center", []rasterVertex{{X: .5 - 1e-13, Y: .5, Z: .5, U: 0, V: 0, Q: 1}, {X: 31.5, Y: .5, Z: .5, U: 1, V: 0, Q: 1}, {X: 31.5, Y: 31.5 + 1e-13, Z: .5, U: 1, V: 1, Q: 1}, {X: .5, Y: 31.5, Z: .5, U: 0, V: 1, Q: 1}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, reference := rasterFixture(), rasterFixture()
			actual.vertices, reference.vertices = test.vertices, test.vertices
			for i := range actual.vertices {
				actual.vertices[i].Head = 1
			}
			state := FrameState{}
			for _, depthStep := range []float64{0, 1e-10 - 1e-13, 2e-13} {
				for i := range actual.vertices {
					actual.vertices[i].Z += depthStep
					actual.vertices[i].U += .01
				}
				for _, ids := range [][3]uint16{{0, 1, 2}, {0, 2, 3}, {2, 1, 0}} {
					for _, boundary := range []bool{false, true} {
						actual.triangle(ids[0], ids[1], ids[2], boundary, false, &state)
						referenceTriangle(reference, ids[0], ids[1], ids[2], boundary, false, state)
					}
				}
			}
			for i, value := range actual.body.Pixels {
				difference := int(value) - int(reference.body.Pixels[i])
				if difference < -1 || difference > 1 || (i%4 == 3 && value != reference.body.Pixels[i]) {
					t.Fatalf("pixel %d channel %d: got %d, reference %d", i/4, i%4, value, reference.body.Pixels[i])
				}
			}
			if bytes.Count(actual.body.Pixels, []byte{0}) == len(actual.body.Pixels) {
				t.Fatal("fixture rendered no coverage")
			}
		})
	}
}

// referenceTriangle freezes the pre-optimization coverage/interpolation formula.
// It is an independent oracle for top-left ownership and premultiplied fringes.
func referenceTriangle(r *Renderer, ia, ib, ic uint16, boundary, reference bool, s FrameState) {
	a, b, c := r.vertices[ia], r.vertices[ib], r.vertices[ic]
	area := edge(a, b, c.X, c.Y)
	if area == 0 {
		return
	}
	if area < 0 {
		b, c = c, b
		area = -area
	}
	x0, x1 := max(r.left, int(math.Ceil(math.Min(a.X, math.Min(b.X, c.X))-.5))), min(r.right-1, int(math.Floor(math.Max(a.X, math.Max(b.X, c.X))-.5)))
	y0, y1 := max(r.top, int(math.Ceil(math.Min(a.Y, math.Min(b.Y, c.Y))-.5))), min(r.bottom-1, int(math.Floor(math.Max(a.Y, math.Max(b.Y, c.Y))-.5)))
	inv := 1 / area
	tl0, tl1, tl2 := topLeft(b, c), topLeft(c, a), topLeft(a, b)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			px, py := float64(x)+.5, float64(y)+.5
			e0, e1, e2 := edge(b, c, px, py), edge(c, a, px, py), edge(a, b, px, py)
			if e0 < 0 || e1 < 0 || e2 < 0 || (e0 == 0 && !tl0) || (e1 == 0 && !tl1) || (e2 == 0 && !tl2) {
				continue
			}
			wa, wb, wc := e0*inv, e1*inv, e2*inv
			z := wa*a.Z + wb*b.Z + wc*c.Z
			index := y*r.body.Width + x
			if z > r.depth[index]+1e-10 {
				continue
			}
			inverseQ := 1 / (wa*a.Q + wb*b.Q + wc*c.Q)
			u, v := (wa*a.U+wb*b.U+wc*c.U)*inverseQ, (wa*a.V+wb*b.V+wc*c.V)*inverseQ
			if (reference && (s.Breath == 0 || a.Torso == 0 && b.Torso == 0 && c.Torso == 0)) || (a.Head == 0 && b.Head == 0 && c.Head == 0 && s.Breath == 0) {
				u, v = px/float64(r.body.Width), py/float64(r.body.Height)
			}
			color := r.fragment(u, v, &s)
			if boundary {
				if color[3] >= .995 || color[3] <= .001 {
					continue
				}
				at := index * 4
				for k := range 4 {
					color[k] += float64(r.body.Pixels[at+k]) / 255 * (1 - color[3])
				}
				r.body.Set(x, y, color)
			} else if color[3] >= .995 {
				r.depth[index] = z
				r.body.Set(x, y, color)
			}
		}
	}
}
