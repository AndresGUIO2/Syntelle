package nativerender

import (
	"errors"
	"math"
)

type eyeCurve struct {
	upper, lower, seam float64
	valid              bool
}

type eyeSpan struct {
	upper, lower, band float64
}

// EyeRenderer evaluates the existing eyelid and source-bound iris composition
// directly. Static iris covered colors are prepared once, not every frame.
type EyeRenderer struct {
	opened, closed, mask, sclera, ownership *Texture
	output                                  *Texture
	curves                                  []eyeCurve
	spans                                   []eyeSpan
	iris                                    []Color
	eyeSplit, lashBand                      float64
	lastClosure                             float64
	lastSquint                              float64
	lastGaze                                [2][2]float64
	hasLast                                 bool
}

func NewEyeRenderer(opened, closed, mask, sclera, ownership *Texture, curves [][]float64, eyeSplit float64) (*EyeRenderer, error) {
	if !patchTextureValid(opened) || opened.Height < 2 || !patchFinite(eyeSplit) {
		return nil, errors.New("eye resources have invalid dimensions")
	}
	for _, image := range []*Texture{closed, mask, sclera, ownership} {
		if !patchTextureValid(image) || image.Width != opened.Width || image.Height != opened.Height {
			return nil, errors.New("eye resources have incompatible dimensions")
		}
	}
	if len(curves) != opened.Width {
		return nil, errors.New("eyelid curves have incompatible dimensions")
	}
	renderer := &EyeRenderer{opened: opened, closed: closed, mask: mask, sclera: sclera, ownership: ownership,
		output: NewTexture(opened.Width, opened.Height, false), curves: make([]eyeCurve, opened.Width),
		spans: make([]eyeSpan, opened.Width), iris: make([]Color, opened.Width*opened.Height), eyeSplit: eyeSplit}
	pack := func(value float64) float64 {
		maximum := float64(opened.Height - 1)
		return math.Round(math.Max(0, math.Min(1, value/maximum))*65535) / 65535 * maximum
	}
	run, longest := 0, 0
	for x, curve := range curves {
		if len(curve) < 4 || !patchFinite(curve[0]) || !patchFinite(curve[1]) || !patchFinite(curve[2]) || !patchFinite(curve[3]) {
			return nil, errors.New("eyelid curve is incomplete")
		}
		renderer.curves[x] = eyeCurve{upper: pack(curve[0]), lower: pack(curve[1]), seam: pack(curve[2]), valid: curve[3] > 0}
		if curve[3] > 0 {
			run++
		} else {
			run = 0
		}
		longest = max(longest, run)
	}
	renderer.lashBand = math.Max(2, float64(longest)*.12)
	for y := range opened.Height {
		for x := range opened.Width {
			original := opened.Sample(float64(x), float64(y))
			own := ownership.Sample(float64(x), float64(y))
			clean := sclera.Sample(float64(x), float64(y))
			value := Color{0, 0, 0, own[1] * own[2]}
			for channel := range 3 {
				value[channel] = (original[channel] - (1-own[1])*clean[channel]) * own[2]
			}
			renderer.iris[y*opened.Width+x] = value
		}
	}
	return renderer, nil
}

// Draw returns a reused opaque patch; the original foreground alpha is retained
// when the patch is applied to the character texture.
func (renderer *EyeRenderer) Draw(closure float64, gaze [2][2]float64) *Texture {
	return renderer.DrawExpression(closure, gaze, 0)
}

func (renderer *EyeRenderer) DrawExpression(closure float64, gaze [2][2]float64, squint float64) *Texture {
	squint = clamp(squint, 0, .4)
	if !patchFinite(closure) {
		closure = 0
	}
	closure = float64(float32(math.Max(0, math.Min(1, closure))))
	for eye := range 2 {
		for axis := range 2 {
			if !patchFinite(gaze[eye][axis]) {
				gaze[eye][axis] = 0
			}
			gaze[eye][axis] = float64(float32(gaze[eye][axis]))
		}
	}
	if renderer.hasLast && renderer.lastClosure == closure && renderer.lastGaze == gaze && renderer.lastSquint == squint {
		return renderer.output
	}
	height := float64(renderer.opened.Height)
	for x, curve := range renderer.curves {
		renderer.spans[x] = eyeSpan{upper: patchMix(curve.upper, curve.seam, closure), lower: patchMix(curve.lower, curve.seam, closure) - (curve.lower-curve.upper)*squint*(1-closure),
			band: math.Min(renderer.lashBand, math.Min(math.Min(curve.upper, curve.seam), math.Min(height-1-curve.lower, height-1-curve.seam))*.8)}
	}
	for y := range renderer.opened.Height {
		for x := range renderer.opened.Width {
			base := renderer.opened.Sample(float64(x), float64(y))
			target := renderer.closed.Sample(float64(x), float64(y))
			alpha := renderer.mask.Sample(float64(x), float64(y))[0]
			color := base
			if closure == 1 {
				color = patchMixColor(base, target, alpha)
			} else {
				offset := gaze[0]
				if float64(x)+.5 >= renderer.eyeSplit {
					offset = gaze[1]
				}
				eye := renderer.gazed(x, y, offset, base)
				if closure == 0 && squint == 0 {
					color = eye
				} else {
					color = patchMixColor(base, target, closure)
					curve, span := renderer.curves[x], renderer.spans[x]
					if curve.valid {
						fy := float64(y)
						sourceY := math.Max(eyeLowerMap(fy, span.lower, curve.lower, span.band, height), curve.lower+.5)
						closedY := eyeLowerMap(fy, span.lower, curve.seam, span.band, height)
						if fy < span.upper {
							sourceY = math.Min(eyeUpperMap(fy, span.upper, curve.upper, span.band), curve.upper-.5)
							closedY = eyeUpperMap(fy, span.upper, curve.seam, span.band)
						}
						fromSkin := renderer.opened.Sample(float64(x), sourceY)
						toSkin := renderer.closed.Sample(float64(x), closedY)
						skin := patchMixColor(fromSkin, toSkin, closure)
						coverage := math.Max(0, math.Min(1, math.Min(fy+.5, span.lower)-math.Max(fy-.5, span.upper)))
						color = patchMixColor(skin, eye, coverage)
					}
					color = patchMixColor(base, color, alpha)
				}
			}
			color[3] = 1
			renderer.output.Set(x, y, color)
		}
	}
	renderer.lastClosure, renderer.lastGaze, renderer.hasLast = closure, gaze, true
	renderer.lastSquint = squint
	return renderer.output
}

func (renderer *EyeRenderer) gazed(x, y int, offset [2]float64, base Color) Color {
	if offset == [2]float64{} {
		return base
	}
	own := renderer.ownership.Sample(float64(x), float64(y))
	if own[0] == 0 {
		return base
	}
	sx, sy := float64(x)-offset[0], float64(y)-offset[1]
	shifted := renderer.ownership.Sample(sx, sy)
	iris := renderer.irisSample(sx, sy)
	missing := math.Max(0, math.Min(1, shifted[1]-iris[3]))
	clean := renderer.sclera.Sample(float64(x), float64(y))
	var color Color
	for channel := range 3 {
		value := iris[channel] + base[channel]*missing + clean[channel]*math.Max(0, 1-iris[3]-missing)
		color[channel] = patchMix(base[channel], value, own[0])
	}
	color[3] = 1
	return color
}

func (renderer *EyeRenderer) irisSample(x, y float64) Color {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	read := func(px, py int) Color {
		return renderer.iris[max(0, min(renderer.opened.Height-1, py))*renderer.opened.Width+max(0, min(renderer.opened.Width-1, px))]
	}
	a := patchMixColor(read(int(x0), int(y0)), read(int(x0)+1, int(y0)), fx)
	b := patchMixColor(read(int(x0), int(y0)+1), read(int(x0)+1, int(y0)+1), fx)
	return patchMixColor(a, b, fy)
}

func eyeUpperMap(y, edge, sourceEdge, band float64) float64 {
	if y >= edge-band {
		return y + sourceEdge - edge
	}
	return y * (sourceEdge - band) / math.Max(edge-band, 1)
}

func eyeLowerMap(y, edge, sourceEdge, band, height float64) float64 {
	if y <= edge+band {
		return y + sourceEdge - edge
	}
	bottom := height - 1
	return bottom + (y-bottom)*(sourceEdge+band-bottom)/math.Min(edge+band-bottom, -1)
}
