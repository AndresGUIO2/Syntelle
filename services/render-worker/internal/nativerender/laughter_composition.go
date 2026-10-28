package nativerender

import (
	"errors"
	"math"
	"reflect"
	"syntelle/render-contract"
)

type LaughterCompanion struct {
	JointFieldSHA string  `json:"joint_field_sha256"`
	FieldLayers   int     `json:"field_layers"`
	Feather       float64 `json:"oral_ownership_feather"`
	Version       string
	SourceSHA     string                      `json:"source_sha256"`
	ReferenceSHA  string                      `json:"reference_sha256"`
	GeometrySHA   string                      `json:"geometry_sha256"`
	Mandible      *rendercontract.MandibleRig `json:"mandible_binding"`
	Field         textureRecord
	Crop          Rect
	Units         float64 `json:"units_per_byte"`
	Iterations    int     `json:"inverse_iterations"`
	Validation    struct {
		Valid           bool
		MinimumJacobian float64 `json:"minimum_jacobian"`
		InverseResidual float64 `json:"maximum_inverse_residual_px"`
		Amplitude       float64 `json:"expression_amplitude"`
	}
}

func (a *Assets) loadLaughterCompanion(load func(string) (*Texture, error)) error {
	p := a.Expression.ReferenceLaughter.Presentation
	if p == nil {
		return nil
	}
	if a.Expression.Joint == nil || p.JointFieldSHA != a.Expression.Joint.SkinAppearance.FieldSHA || p.FieldLayers != 2 || p.Version != "laughter-presentation/2" || p.SourceSHA != a.Motion.OriginalSHA || p.ReferenceSHA != a.Expression.ReferenceLaughter.ReferenceSHA || p.GeometrySHA != a.Motion.Projection.Geometry.SHA256 || !reflect.DeepEqual(p.Mandible, a.Motion.Mandible) || !p.Validation.Valid || p.Validation.MinimumJacobian < .45 || p.Validation.InverseResidual > .2 || p.Validation.Amplitude != 1 || p.Units != 1./256 || (p.Feather < .15 || p.Feather > .5) || p.Iterations < 5 || p.Iterations > 16 {
		return errors.New("invalid source-bound laughter presentation companion")
	}
	var err error
	a.ReferenceField, err = load("expressions/" + p.Field.File)
	if err != nil {
		return err
	}
	if a.ReferenceField.Width != p.Crop.Width || a.ReferenceField.Height != p.Crop.Height*p.FieldLayers {
		return errors.New("invalid laughter field dimensions")
	}
	return nil
}
func (r *Renderer) referenceUpper() *Texture {
	if r.assets.Expression.ReferenceLaughter.Presentation != nil {
		return r.assets.Oral.upper
	}
	return r.assets.ReferenceUpper
}
func (r *Renderer) referenceLower() *Texture {
	if r.assets.Expression.ReferenceLaughter.Presentation != nil {
		return r.assets.Oral.lower
	}
	return r.assets.ReferenceLower
}
func oralOutside(x, y float64, c Rect, feather float64) float64 {
	outside := math.Max(0, math.Max(math.Max(float64(c.X)-x, x-float64(c.X+c.Width)), math.Max(float64(c.Y)-y, y-float64(c.Y+c.Height))))
	return patchSmoothstep(0, math.Max(1, float64(c.Width)*feather), outside)
}
func (r *Renderer) laughterDelta(x, y float64) (float64, float64) { return r.laughterField(x, y, 0) }
func (r *Renderer) laughterField(x, y float64, layer int) (float64, float64) {
	p := r.assets.Expression.ReferenceLaughter.Presentation
	if p == nil || x < float64(p.Crop.X) || y < float64(p.Crop.Y) || x > float64(p.Crop.X+p.Crop.Width-1) || y > float64(p.Crop.Y+p.Crop.Height-1) {
		return 0, 0
	}
	c := r.assets.ReferenceField.Sample(x-float64(p.Crop.X), y-float64(p.Crop.Y)+float64(layer*p.Crop.Height))
	return (c[0]*255 + c[1]*255*256 - 32768) * p.Units, (c[2]*255 + c[3]*255*256 - 32768) * p.Units
}
func (r *Renderer) fragmentLaughter(u, v float64, s *FrameState) Color {
	w, h := float64(r.assets.Motion.Canvas.Width), float64(r.assets.Motion.Canvas.Height)
	tx, ty := u*w-.5, v*h-.5
	x, y := tx, ty
	p := r.assets.Expression.ReferenceLaughter.Presentation
	amount := func(x, y float64) float64 {
		q := clamp(s.Laugh, 0, 1)
		if s.LaughSpeech {
			q *= oralOutside(x, y, r.mouthPatch.Rect, p.Feather)
		}
		return q
	}
	if (!s.LaughSpeech && s.Laugh >= 1) || (s.Smile == 0 && s.Brows == 0 && s.Emotion == (rendercontract.EmotionWeights{})) {
		return r.multiviewColor(r.subject(tx, ty), u, v, s)
	}
	total := func(x, y float64) (float64, float64) {
		baseline := *s
		if !s.RestingSmile {
			baseline.Smile = 0
		}
		jx, jy := 0., 0.
		if !s.RestingSmile {
			jx, jy = r.laughterField(x, y, 1)
			jx *= s.Smile * oralOutside(x, y, r.mouthPatch.Rect, p.Feather)
			jy *= s.Smile * oralOutside(x, y, r.mouthPatch.Rect, p.Feather)
		}
		dx, dy := r.displacement(x+jx, y+jy, &baseline)
		ex, ey := r.emotionDisplacement(x+jx, y+jy, s.Emotion)
		dx += jx
		dy += jy
		lx, ly := r.laughterDelta(x, y)
		q := amount(x, y)
		return (dx+ex)*(1-q) + lx*q, (dy+ey)*(1-q) + ly*q
	}
	for i := 0; i < p.Iterations; i++ {
		dx, dy := total(x, y)
		rx, ry := x+dx-tx, y+dy-ty
		if math.Hypot(rx, ry) < .001 {
			break
		}
		xp, xyp := total(x+.5, y)
		xm, xym := total(x-.5, y)
		yp, yyp := total(x, y+.5)
		ym, yym := total(x, y-.5)
		a, b, c, d := 1+xp-xm, yp-ym, xyp-xym, 1+yyp-yym
		det := a*d - b*c
		if det <= .2 {
			break
		}
		sx, sy := (d*rx-b*ry)/det, (-c*rx+a*ry)/det
		scale := math.Max(1, math.Hypot(sx, sy)/32)
		x -= sx / scale
		y -= sy / scale
	}

	lx, ly := r.laughterDelta(x, y)
	q := amount(x, y)
	u, v = (x+q*lx+.5)/w, (y+q*ly+.5)/h
	color := r.subject(u*w-.5, v*h-.5)
	smile := s.Smile * (1 - q)
	if s.RestingSmile && smile > 0 && r.assets.SmileMaterial != nil {
		rect := r.assets.Expression.Crop
		px, py := u*w-float64(rect.X)-.5, v*h-float64(rect.Y)-.5
		if px >= 0 && py >= 0 && px < float64(rect.Width) && py < float64(rect.Height) {
			ref := r.assets.SmileMaterial.Sample(px, py)
			for k := range 3 {
				color[k] = color[k]*(1-ref[3]*smile) + ref[k]*color[3]*smile
			}
		}
	}
	return r.multiviewColor(color, u, v, s)
}
