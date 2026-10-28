package nativerender

import "math"

// Only source-derived soft tissue is interpolated here. Oral geometry and the
// opaque canonical teeth are rendered afterward, in their own coordinate space.
func (r *Renderer) prepareSkin(amount, laugh float64) bool {
	if r.expressionBase == nil || len(r.assets.SkinFrames) != 25 {
		return false
	}
	amount = math.Round(clamp(amount, 0, 1)*120) / 120
	laugh = math.Round(clamp(laugh, 0, 1)*120) / 120
	if r.initialized && r.lastSkinSmile == amount && r.lastSkinLaugh == laugh {
		return false
	}
	index := amount * 24
	low := int(index)
	high := min(24, low+1)
	weight := index - float64(low)
	a, b := r.assets.SkinFrames[low], r.assets.SkinFrames[high]
	rect := r.expressionPatch.Rect
	for y := range rect.Height {
		for x := range rect.Width {
			at := (y*rect.Width + x) * 4
			global := ((y+rect.Y)*r.expressionBase.Width + x + rect.X) * 4
			alpha := float64(r.assets.Foreground.Pixels[global+3]) / 255
			sampleAlpha := mix(float64(a.Pixels[at+3]), float64(b.Pixels[at+3]), weight) / 255
			for channel := range 3 {
				value := float64(r.assets.Foreground.Pixels[global+channel]) / 255
				if sampleAlpha > 0 {
					value = mix(float64(a.Pixels[at+channel]), float64(b.Pixels[at+channel]), weight) / 255 * alpha / sampleAlpha
				}
				if laugh > 0 && len(r.assets.LaughFrames) == 25 {
					la, lb := r.assets.LaughFrames[low], r.assets.LaughFrames[high]
					opacity := mix(float64(la.Pixels[at+3]), float64(lb.Pixels[at+3]), weight) / 255
					if opacity > 0 {
						color := mix(float64(la.Pixels[at+channel]), float64(lb.Pixels[at+channel]), weight) / 255 * alpha / opacity
						value = mix(value, color, laugh)
					}
				}
				if amount == 0 {
					r.expressionBase.Pixels[global+channel] = r.assets.Foreground.Pixels[global+channel]
				} else {
					r.expressionBase.Pixels[global+channel] = byteColor(value)
				}
			}
			r.expressionBase.Pixels[global+3] = r.assets.Foreground.Pixels[global+3]
			copy(r.expressionPatch.Pixels.Pixels[at:at+4], r.expressionBase.Pixels[global:global+4])
			copy(r.subjectTexture.Pixels[global:global+4], r.expressionBase.Pixels[global:global+4])
		}
	}
	r.lastSkinSmile = amount
	r.lastSkinLaugh = laugh
	return true
}

// The manual resting smile keeps the original registered material and field.
// Joint geometry is used only by articulation and prepared laughter.
func (r *Renderer) jointSmile(s FrameState) float64 {
	if s.RestingSmile || (r.referenceActive(s) && r.assets.Expression.ReferenceLaughter.Presentation == nil) {
		return 0
	}
	return s.Smile
}

// Laughter owns a complete smile even when its acoustic jaw has closed. Reuse
// the calibrated joint oral smile only; cheeks and lids keep their separate
// full reference expression and never receive another smile deformation.
func (r *Renderer) oralSmile(s FrameState) float64 {
	amount := r.jointSmile(s)
	if r.referenceActive(s) && r.assets.Expression.ReferenceLaughter.Presentation != nil && !s.LaughSpeech {
		amount = math.Max(amount, s.Laugh)
	}
	return amount
}

func (r *Renderer) laughSquint(s FrameState) float64 {
	tension := emotionSquint(s.Emotion)
	if rig := r.assets.Expression.EmotionRig; rig != nil && len(rig.EyelidTension) == 10 {
		tension = 0
		for i, weight := range s.Emotion {
			tension += weight * rig.EyelidTension[i]
		}
	}
	if joint := r.assets.Expression.Joint; joint != nil && joint.Laugh != nil {
		return clamp(s.Laugh*joint.Laugh.EyeClosure+tension, 0, .4)
	}
	return 0
}
