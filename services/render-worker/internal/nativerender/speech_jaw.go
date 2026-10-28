package nativerender

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
)

func (a *Assets) validateSpeechJaw() error {
	rig := a.Motion.SpeechJaw
	if rig == nil {
		return nil
	}
	var binding struct{ Mandible json.RawMessage }
	if err := json.Unmarshal(a.MotionJSON, &binding); err != nil {
		return err
	}
	if len(binding.Mandible) > 0 && string(binding.Mandible) != "null" {
		return errors.New("speech-only skin jaw cannot enable the oral mandible path")
	}
	if (rig.Version != "speech-jaw/1" && rig.Version != "speech-jaw/2") || rig.SourceSHA != a.Motion.OriginalSHA || rig.MotionBuild != a.Motion.BuildID || rig.OralBuild != a.Motion.OralBuildID || rig.GeometrySHA != a.Motion.Projection.Geometry.SHA256 || len(rig.Weights) != len(a.vertices) || !rig.Validation.Valid || rig.Validation.MinimumAreaRatio <= .25 || rig.MaximumChinFraction <= 0 || rig.MaximumChinFraction > .0075 || rig.Validation.ChinFraction < 0 || rig.Validation.ChinFraction > rig.MaximumChinFraction+1e-8 || !patchFinite(rig.MaximumAngle) || rig.MaximumAngle <= 0 || rig.MaximumAngle > .08 {
		return errors.New("invalid source-bound speech-only jaw")
	}
	var oral struct{ Crop Rect }
	if err := json.Unmarshal(a.OralJSON, &oral); err != nil {
		return err
	}
	crop := rig.ProtectedOralCrop
	if crop.X != oral.Crop.X || crop.Y != oral.Crop.Y || crop.Width != oral.Crop.Width || crop.Height != oral.Crop.Height || !patchFinite(rig.LowerNeckY) || rig.LowerNeckY < a.Motion.Anatomy.ChinY || rig.LowerNeckY > a.Motion.Anatomy.ChinY+a.Motion.Anatomy.FaceHeight*.1 {
		return errors.New("invalid speech jaw protection boundary")
	}
	for _, v := range append(append(rig.Hinge[:], rig.Chin[:]...), rig.MaximumChinFraction, rig.Validation.ChinFraction, rig.Validation.MinimumAreaRatio) {
		if !patchFinite(v) {
			return errors.New("non-finite speech jaw calibration")
		}
	}
	for pose := range a.Oral.poses {
		amount, ok := rig.PoseOpening[pose]
		if !ok || !patchFinite(amount) || amount < 0 || amount > 1 {
			return errors.New("invalid speech jaw pose binding")
		}
	}
	if rig.PoseOpening["Rest"] != 0 {
		return errors.New("speech jaw rest must remain zero")
	}
	upper := float64(crop.Y)
	if rig.Version == "speech-jaw/2" {
		support := rig.OralSupport
		guard := math.Max(2, a.Motion.Anatomy.FaceHeight*2.6/720*2)
		if support == nil || support.Version != "calibrated-outer-lip-support/1" || len(support.Polygon) < 3 || !patchFinite(support.SamplingGuard) || math.Abs(support.SamplingGuard-guard) > 1e-8 || !patchFinite(support.TransitionWidth) || support.TransitionWidth <= guard*2 || support.TransitionWidth > a.Motion.Anatomy.FaceHeight*.1+1e-8 {
			return errors.New("invalid calibrated speech jaw oral support")
		}
		expected := a.calibratedSpeechOralSupport()
		if len(expected) != len(support.Polygon) {
			return errors.New("speech jaw oral support does not match calibrated sources")
		}
		upper = math.Inf(1)
		for i, p := range support.Polygon {
			if !patchFinite(p[0]) || !patchFinite(p[1]) || math.Hypot(p[0]-expected[i][0], p[1]-expected[i][1]) > 1e-5 {
				return errors.New("speech jaw oral support does not match calibrated sources")
			}
			upper = math.Min(upper, p[1])
		}
		bottom := math.Inf(-1)
		for i, a := range support.Polygon {
			b := support.Polygon[(i+1)%len(support.Polygon)]
			if a[0] <= rig.Chin[0] && rig.Chin[0] < b[0] || b[0] <= rig.Chin[0] && rig.Chin[0] < a[0] {
				bottom = math.Max(bottom, a[1]+(rig.Chin[0]-a[0])*(b[1]-a[1])/(b[0]-a[0]))
			}
		}
		if !patchFinite(bottom) || bottom >= rig.Chin[1] {
			return errors.New("invalid central jaw transition source")
		}
		previous := -1.
		for _, fraction := range []float64{.5, .65, .8} {
			targetY := bottom + (rig.Chin[1]-bottom)*fraction
			nearest, best := 0, math.Inf(1)
			for i, v := range a.vertices {
				dx := v.X*float64(a.Motion.Canvas.Width) - rig.Chin[0]
				dy := v.Y*float64(a.Motion.Canvas.Height) - targetY
				if distance := dx*dx + dy*dy; distance < best {
					nearest, best = i, distance
				}
			}
			weight := rig.Weights[nearest]
			if weight <= .001 || weight+1e-7 < previous {
				return errors.New("speech jaw central skin transition is disconnected")
			}
			previous = weight
		}
	} else if rig.OralSupport != nil {
		return errors.New("legacy speech jaw must retain its rectangular protection semantics")
	}
	for i, weight := range rig.Weights {
		if !patchFinite(weight) || weight < 0 || weight > 1 {
			return errors.New("invalid speech jaw weight")
		}
		v := a.vertices[i]
		x, y := v.X*float64(a.Motion.Canvas.Width), v.Y*float64(a.Motion.Canvas.Height)
		inside := x >= float64(crop.X) && x <= float64(crop.X+crop.Width) && y >= float64(crop.Y) && y <= float64(crop.Y+crop.Height)
		if weight != 0 && ((rig.Version == "speech-jaw/1" && inside) || y >= rig.LowerNeckY || y <= upper || v.Head == 0) {
			return errors.New("speech jaw overlaps protected oral, upper face or neck geometry")
		}
	}
	return nil
}

func (a *Assets) validateSpeechJawSamplingProtection() error {
	rig := a.Motion.SpeechJaw
	if rig == nil {
		return nil
	}
	crop := rig.ProtectedOralCrop
	guard := math.Max(2, a.Motion.Anatomy.FaceHeight*2.6/720*2)
	for i := 0; i < len(a.indices); i += 3 {
		left, top, right, bottom := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, id := range a.indices[i : i+3] {
			v := a.vertices[id]
			x, y := v.X*float64(a.Motion.Canvas.Width), v.Y*float64(a.Motion.Canvas.Height)
			left, top, right, bottom = math.Min(left, x), math.Min(top, y), math.Max(right, x), math.Max(bottom, y)
		}
		if rig.Version == "speech-jaw/2" {
			points := [3][2]float64{}
			for k, id := range a.indices[i : i+3] {
				v := a.vertices[id]
				points[k] = [2]float64{v.X * float64(a.Motion.Canvas.Width), v.Y * float64(a.Motion.Canvas.Height)}
			}
			if !speechJawSupportOverlaps(points, rig.OralSupport.Polygon, guard) {
				continue
			}
		} else if left > float64(crop.X+crop.Width)+guard || right < float64(crop.X)-guard || top > float64(crop.Y+crop.Height)+guard || bottom < float64(crop.Y)-guard {
			continue
		}
		for _, id := range a.indices[i : i+3] {
			if rig.Weights[id] != 0 {
				return errors.New("speech jaw overlaps interpolated oral sampling")
			}
		}
	}
	return nil
}

// Convex endpoint support includes smile-plus-normalized-emotion extrema.
// It uses the loaded calibrated source coordinates, never inferred imagery.
func (a *Assets) calibratedSpeechOralSupport() [][2]float64 {
	points := make([][2]float64, 0)
	for name, pair := range a.Oral.pairs {
		if pair.Mesh == nil {
			continue
		}
		for phase, vertices := range [][][2]float64{pair.Mesh.Rest, pair.Mesh.Open} {
			for _, smile := range []bool{false, true} {
				states := []JointPair{{}}
				if a.Expression.EmotionRig != nil && a.Expression.EmotionRig.Oral != nil {
					for _, pairs := range a.Expression.EmotionRig.Oral.States {
						states = append(states, pairs[name])
					}
				}
				for _, emotion := range states {
					for i := 0; i < 20; i++ {
						p := vertices[i]
						if smile && a.Expression.Joint != nil {
							delta := a.Expression.Joint.Pairs[name].RestDelta
							if phase == 1 {
								delta = a.Expression.Joint.Pairs[name].OpenDelta
							}
							if i < len(delta) {
								p[0] += delta[i][0]
								p[1] += delta[i][1]
							}
						}
						delta := emotion.RestDelta
						if phase == 1 {
							delta = emotion.OpenDelta
						}
						if i < len(delta) {
							p[0] += delta[i][0]
							p[1] += delta[i][1]
						}
						points = append(points, p)
					}
				}
			}
		}
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i][0] < points[j][0] || points[i][0] == points[j][0] && points[i][1] < points[j][1]
	})
	unique := points[:0]
	for _, p := range points {
		if len(unique) == 0 || unique[len(unique)-1] != p {
			unique = append(unique, p)
		}
	}
	if len(unique) < 3 {
		return nil
	}
	cross := func(a, b, c [2]float64) float64 { return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0]) }
	lower, upper := make([][2]float64, 0), make([][2]float64, 0)
	for _, p := range unique {
		for len(lower) >= 2 && cross(lower[len(lower)-2], lower[len(lower)-1], p) <= 0 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, p)
	}
	for i := len(unique) - 1; i >= 0; i-- {
		p := unique[i]
		for len(upper) >= 2 && cross(upper[len(upper)-2], upper[len(upper)-1], p) <= 0 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, p)
	}
	out := append(lower[:len(lower)-1], upper[:len(upper)-1]...)
	for i := range out {
		out[i][0] += float64(a.Motion.SpeechJaw.ProtectedOralCrop.X)
		out[i][1] += float64(a.Motion.SpeechJaw.ProtectedOralCrop.Y)
	}
	return out
}

func speechJawSupportOverlaps(triangle [3][2]float64, polygon [][2]float64, guard float64) bool {
	// Most source triangles are torso/background or upper face. This exact
	// separating axis avoids testing every lip edge against those triangles.
	left, top, right, bottom := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range polygon {
		left = math.Min(left, p[0])
		top = math.Min(top, p[1])
		right = math.Max(right, p[0])
		bottom = math.Max(bottom, p[1])
	}
	tl, tt, tr, tb := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range triangle {
		tl = math.Min(tl, p[0])
		tt = math.Min(tt, p[1])
		tr = math.Max(tr, p[0])
		tb = math.Max(tb, p[1])
	}
	if tr < left-guard-1e-7 || tl > right+guard+1e-7 || tb < top-guard-1e-7 || tt > bottom+guard+1e-7 {
		return false
	}
	separated := func(axis [2]float64) bool {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, p := range triangle {
			v := p[0]*axis[0] + p[1]*axis[1]
			lo = math.Min(lo, v)
			hi = math.Max(hi, v)
		}
		pl, ph := math.Inf(1), math.Inf(-1)
		for _, p := range polygon {
			v := p[0]*axis[0] + p[1]*axis[1]
			pl = math.Min(pl, v)
			ph = math.Max(ph, v)
		}
		padding := guard * math.Hypot(axis[0], axis[1])
		return hi < pl-padding-1e-7 || lo > ph+padding+1e-7
	}
	for i, a := range polygon {
		b := polygon[(i+1)%len(polygon)]
		if separated([2]float64{a[1] - b[1], b[0] - a[0]}) {
			return false
		}
	}
	for i, a := range triangle {
		b := triangle[(i+1)%3]
		if separated([2]float64{a[1] - b[1], b[0] - a[0]}) {
			return false
		}
	}
	return true
}

func (r *Renderer) speechJawActive(s FrameState) bool {
	return r.assets.Motion.SpeechJaw != nil && s.SpeechJaw > 0 && s.Laugh == 0
}
func (r *Renderer) speechJawVertexActive(s FrameState, index int) bool {
	return r.speechJawActive(s) && r.assets.Motion.SpeechJaw.Weights[index] > 0
}
func (r *Renderer) speechJawTriangleReference(s FrameState, a, b, c uint16) bool {
	return r.speechJawActive(s) && s.Head == [3]float64{} && !r.speechJawVertexActive(s, int(a)) && !r.speechJawVertexActive(s, int(b)) && !r.speechJawVertexActive(s, int(c))
}
func (r *Renderer) applySpeechJaw(world vector3, index int, s FrameState) vector3 {
	if !r.speechJawVertexActive(s, index) {
		return world
	}
	rig := r.assets.Motion.SpeechJaw
	angle := -rig.MaximumAngle * clamp(s.SpeechJaw, 0, 1)
	co, si := math.Cos(angle), math.Sin(angle)
	y, z := world[1]-rig.Hinge[1], world[2]-rig.Hinge[2]
	world[1] += rig.Weights[index] * (co*y - si*z - y)
	world[2] += rig.Weights[index] * (si*y + co*z - z)
	return world
}
