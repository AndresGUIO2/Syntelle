package nativerender

import "syntelle/render-contract"

func insideField(x, y float64, rect Rect) bool {
	return x >= float64(rect.X)-.5 && x <= float64(rect.X+rect.Width)-.5 && y >= float64(rect.Y)-.5 && y <= float64(rect.Y+rect.Height)-.5
}

func (r *Renderer) emotionDisplacement(x, y float64, weights rendercontract.EmotionWeights) (dx, dy float64) {
	rig := r.assets.Expression.EmotionRig
	if rig == nil || r.assets.EmotionField == nil || weights == (rendercontract.EmotionWeights{}) {
		return
	}
	u, v := (x-float64(rig.Crop.X)+.5)/float64(rig.Crop.Width), (y-float64(rig.Crop.Y)+.5)/float64(rig.Crop.Height)
	if u < 0 || u > 1 || v < 0 || v > 1 {
		return
	}
	width, height := float64(rig.FieldSize[0]), float64(rig.FieldSize[1])
	fx, fy := clamp(u*width-.5, 0, width-1), clamp(v*height-.5, 0, height-1)
	for i, weight := range weights {
		if i == 0 || weight == 0 {
			continue
		}
		c := r.assets.EmotionField.Sample(fx, fy+float64(i)*height)
		dx += (c[0]*255 - 128) * rig.Units * weight
		dy += (c[1]*255 - 128) * rig.Units * weight
	}
	return
}

func emotionSquint(weights rendercontract.EmotionWeights) float64 {
	factors := [10]float64{0, .12, .11, .07, .01, .03, .17, 0, 0, .14}
	value := 0.
	for i, w := range weights {
		value += w * factors[i]
	}
	return value
}
