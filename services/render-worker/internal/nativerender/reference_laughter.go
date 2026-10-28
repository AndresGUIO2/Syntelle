package nativerender

import (
	"errors"
	"math"
)

// ReferenceLaughter owns one independent full-face expression. Its source and
// geometry are prepared once; ordinary smile resources remain untouched.
type ReferenceLaughter struct {
	Presentation *LaughterCompanion `json:"presentation"`
	Version      string
	SourceSHA    string `json:"source_sha256"`
	MotionBuild  string `json:"motion_build_id"`
	OralBuild    string `json:"oral_build_id"`
	ReferenceSHA string `json:"reference_sha256"`
	Crop         Rect
	Frames       []textureRecord
	DentalReady  bool `json:"dental_ready"`
	Validation   struct{ Valid bool }
	Eyes         struct {
		Crop     Rect
		Profiles []ReferenceEyeProfile
	}
	Oral struct {
		Transition       *OralTransition         `json:"transition"`
		ContactShadow    *referenceContactShadow `json:"contact_shadow"`
		Inner            [][][2]float64
		Outer            [][][2]float64
		LowerTranslation [][2]float64 `json:"lower_translation"`
		Cavity           textureRecord
		ObservedUpper    *textureRecord `json:"observed_upper"`
		ObservedLower    *textureRecord `json:"observed_lower"`
		JawLimitPX       float64        `json:"jaw_limit_px"`
	}
}

// OralTransition joins two already calibrated material coordinate paths.
// It does not reconstruct hidden teeth or deform the canonical enamel.
type OralTransition struct {
	Version   string
	SourceSHA string `json:"source_sha256"`
	OralBuild string `json:"oral_build_id"`
	Upper     [][2]float64
	Lower     [][2]float64
}

type referenceContactShadow struct {
	Version  string
	Strength float64
	Width    float64 `json:"width_px"`
}

// Shade observed enamel only when the lips cover more than in its source.
// Coverage/opacity and the source material remain independent of illumination.
func referenceToothExposure(distance, gap, referenceGap float64, shadow *referenceContactShadow) float64 {
	if shadow == nil || referenceGap <= 0 {
		return 1
	}
	closure := clamp(1-gap/referenceGap, 0, 1)
	illumination := 1 - shadow.Strength*closure*math.Exp(-math.Max(0, distance)/shadow.Width)
	return math.Pow(math.Max(.55, illumination), 1/2.2)
}

type ReferenceEyeProfile struct {
	Curves      [][]float64
	IrisOffsets [2][2]float64 `json:"iris_offsets"`
}

func (a *Assets) loadReference(load func(string) (*Texture, error)) error {
	p := a.Expression.ReferenceLaughter
	if p == nil {
		return nil
	}
	if err := a.loadLaughterCompanion(load); err != nil {
		return err
	}
	if t := p.Oral.Transition; t != nil {
		if t.Version != "oral-transition/1" || t.SourceSHA != a.Motion.OriginalSHA || t.OralBuild != a.Motion.OralBuildID || len(t.Upper) != 25 || len(t.Lower) != 25 || t.Upper[24] != ([2]float64{}) || t.Lower[24] != ([2]float64{}) {
			return errors.New("invalid oral transition binding")
		}
		for _, row := range [][][2]float64{t.Upper, t.Lower} {
			for _, v := range row {
				if !patchFinite(v[0]) || !patchFinite(v[1]) || math.Hypot(v[0], v[1]) > float64(a.Oral.Height)*.25 {
					return errors.New("invalid oral transition placement")
				}
			}
		}
	}
	if shadow := p.Oral.ContactShadow; shadow != nil {
		if shadow.Version != "aperture-contact-shadow/1" || !patchFinite(shadow.Strength) || shadow.Strength < 0 || shadow.Strength > .45 || !patchFinite(shadow.Width) || shadow.Width <= 0 || shadow.Width > float64(a.Oral.Width) {
			return errors.New("invalid reference contact lighting")
		}
	}
	if p.Version != "reference-laughter/1" || !p.Validation.Valid || p.SourceSHA != a.Motion.OriginalSHA || p.MotionBuild != a.Motion.BuildID || p.OralBuild != a.Motion.OralBuildID || len(p.ReferenceSHA) != 64 || p.Crop.X < 0 || p.Crop.Y < 0 || p.Crop.Width < 1 || p.Crop.Height < 1 || p.Crop.X+p.Crop.Width > a.Foreground.Width || p.Crop.Y+p.Crop.Height > a.Foreground.Height || p.Eyes.Crop != a.Motion.Eyes.Crop || len(p.Frames) != 25 || len(p.Eyes.Profiles) != 25 || len(p.Oral.Inner) != 25 || len(p.Oral.Outer) != 25 || len(p.Oral.LowerTranslation) != 25 || !patchFinite(p.Oral.JawLimitPX) || p.Oral.JawLimitPX < 0 {
		return errors.New("invalid reference laughter binding")
	}
	for i, frame := range p.Frames {
		image, err := load("expressions/" + frame.File)
		if err != nil {
			return err
		}
		if image.Width != p.Crop.Width || image.Height != p.Crop.Height {
			return errors.New("reference face dimensions differ")
		}
		a.ReferenceFrames = append(a.ReferenceFrames, image.Premultiply())
		profile := p.Eyes.Profiles[i]
		if len(profile.Curves) != p.Eyes.Crop.Width || len(p.Oral.Inner[i]) != 20 || len(p.Oral.Outer[i]) != 20 {
			return errors.New("reference profiles have incompatible dimensions")
		}
		for _, curve := range profile.Curves {
			if len(curve) != 4 {
				return errors.New("incomplete reference eye curve")
			}
			for _, v := range curve {
				if !patchFinite(v) {
					return errors.New("non-finite reference eye curve")
				}
			}
			if curve[3] > 0 && (curve[0] > curve[1] || curve[1]-curve[0] > 1e-6 && (curve[2] < curve[0] || curve[2] > curve[1])) {
				return errors.New("inverted reference eye closure")
			}
		}
		for _, points := range [][][2]float64{p.Oral.Inner[i], p.Oral.Outer[i], profile.IrisOffsets[:], {p.Oral.LowerTranslation[i]}} {
			for _, point := range points {
				if !patchFinite(point[0]) || !patchFinite(point[1]) {
					return errors.New("non-finite reference geometry")
				}
			}
		}
	}
	var err error
	a.ReferenceCavity, err = load("expressions/" + p.Oral.Cavity.File)
	if err != nil {
		return err
	}
	if a.ReferenceCavity.Width != a.Oral.Width || a.ReferenceCavity.Height != a.Oral.Height {
		return errors.New("reference cavity dimensions differ")
	}
	// Earlier diagnostic packages retain their full canonical support. Current
	// preparation supplies exact observed crown subsets of those same materials.
	a.ReferenceUpper, a.ReferenceLower = a.Oral.upper, a.Oral.lower
	if (p.Oral.ObservedUpper == nil) != (p.Oral.ObservedLower == nil) {
		return errors.New("incomplete reference crown support")
	}
	if p.Oral.ObservedUpper != nil {
		for _, row := range []struct {
			record *textureRecord
			target **Texture
		}{{p.Oral.ObservedUpper, &a.ReferenceUpper}, {p.Oral.ObservedLower, &a.ReferenceLower}} {
			material, loadErr := load("expressions/" + row.record.File)
			if loadErr != nil {
				return loadErr
			}
			if material.Width != a.Oral.Width || material.Height != a.Oral.Height {
				return errors.New("reference crown support dimensions differ")
			}
			*row.target = material.Premultiply()
		}
	}
	return nil
}

func (r *Renderer) referenceActive(s FrameState) bool {
	return r.assets.Expression.ReferenceLaughter != nil && s.Laugh > 0
}

type referenceState struct {
	low, high      int
	weight, jaw    float64
	inner, outer   [20][2]float64
	lower, upper   [2]float64
	localContact   bool
	offsets        [2][2]float64
	triangles      []laughterOralTriangle
	mappingInvalid bool
}

// These use the existing oral triangulation, including its fixed boundary.
// Both endpoint materials are sampled through this one final lip contour.
type laughterOralTriangle struct {
	P, From, To [3][2]float64
	Area        float64
	Flags       [4]int32
}

func sharedLaughterTriangles(indices [][3]int, position func(int) ([2]float64, [2]float64, [2]float64)) ([]laughterOralTriangle, bool) {
	indices = append([][3]int(nil), indices...)
	makeTriangle := func(ids [3]int) laughterOralTriangle {
		t := laughterOralTriangle{}
		for i, id := range ids {
			t.P[i], t.From[i], t.To[i] = position(id)
		}
		// One orientation comes from the closed source, never from a folded
		// final triangle. Both other endpoints must retain that orientation.
		if patchEdge(t.From[0], t.From[1], t.From[2]) < 0 {
			t.P[1], t.P[2] = t.P[2], t.P[1]
			t.From[1], t.From[2] = t.From[2], t.From[1]
			t.To[1], t.To[2] = t.To[2], t.To[1]
		}
		t.Area = patchEdge(t.P[0], t.P[1], t.P[2])
		return t
	}
	valid := func(t laughterOralTriangle) bool {
		areaAt := func(q float64) float64 {
			p := [3][2]float64{}
			for i := range 3 {
				for axis := range 2 {
					p[i][axis] = mix(t.From[i][axis], t.To[i][axis], q)
				}
			}
			return patchEdge(p[0], p[1], p[2])
		}
		v0, vh, v1 := areaAt(0), areaAt(.5), areaAt(1)
		if math.Min(v0, v1) <= 1e-8 {
			return false
		}
		// Signed area is quadratic in interpolation. Check its exact interior
		// minimum rather than missing a thin inversion between sample frames.
		a := 2 * (v1 + v0 - 2*vh)
		b := v1 - v0 - a
		if a > 0 {
			q := -b / (2 * a)
			if q > 0 && q < 1 && areaAt(q) <= 1e-8 {
				return false
			}
		}
		return true
	}
	for attempt := 0; attempt < len(indices)*4; attempt++ {
		bad := -1
		for i, ids := range indices {
			if !valid(makeTriangle(ids)) {
				bad = i
				break
			}
		}
		if bad < 0 {
			out := make([]laughterOralTriangle, len(indices))
			for i, ids := range indices {
				out[i] = makeTriangle(ids)
			}
			return out, true
		}
		flipped := false
		for j, other := range indices {
			if j == bad {
				continue
			}
			shared := []int{}
			a, b := -1, -1
			for _, id := range indices[bad] {
				found := false
				for _, v := range other {
					if id == v {
						found = true
						break
					}
				}
				if found {
					shared = append(shared, id)
				} else {
					a = id
				}
			}
			if len(shared) != 2 {
				continue
			}
			for _, id := range other {
				if id != shared[0] && id != shared[1] {
					b = id
				}
			}
			x, y := [3]int{a, shared[0], b}, [3]int{a, b, shared[1]}
			// A replacement diagonal must split the same quadrilateral, never
			// create two overlapping triangles on one side of that diagonal.
			_, af, at := position(a)
			_, bf, bt := position(b)
			_, uf, ut := position(shared[0])
			_, vf, vt := position(shared[1])
			if patchEdge(af, bf, uf)*patchEdge(af, bf, vf) >= 0 || patchEdge(at, bt, ut)*patchEdge(at, bt, vt) >= 0 {
				continue
			}
			if valid(makeTriangle(x)) && valid(makeTriangle(y)) {
				indices[bad], indices[j] = x, y
				flipped = true
				break
			}
		}
		if !flipped {
			return nil, false
		}
	}
	return nil, false
}

func referenceCoordinates(x, y float64, triangles []laughterOralTriangle) ([2]float64, [2]float64) {
	point := [2]float64{x + .5, y + .5}
	for _, t := range triangles {
		a, b, c := patchEdge(t.P[1], t.P[2], point), patchEdge(t.P[2], t.P[0], point), patchEdge(t.P[0], t.P[1], point)
		if a < -1e-7 || b < -1e-7 || c < -1e-7 {
			continue
		}
		weights := [3]float64{a / t.Area, b / t.Area, c / t.Area}
		from, to := [2]float64{}, [2]float64{}
		for i, w := range weights {
			for axis := range 2 {
				from[axis] += t.From[i][axis] * w
				to[axis] += t.To[i][axis] * w
			}
		}
		return from, to
	}
	return point, point
}

func (r *Renderer) referenceState(s FrameState) referenceState {
	p := r.assets.Expression.ReferenceLaughter
	index := clamp(s.Laugh, 0, 1) * 24
	f := referenceState{low: int(index), high: min(24, int(index)+1), weight: index - float64(int(index)), jaw: p.Oral.JawLimitPX * clamp(s.Amount, 0, 1) * clamp(s.Laugh, 0, 1)}
	if rig := r.assets.Motion.Mandible; rig != nil {
		gap := mix(p.Oral.Inner[f.low][15][1]-p.Oral.Inner[f.low][5][1], p.Oral.Inner[f.high][15][1]-p.Oral.Inner[f.high][5][1], f.weight)
		desired := rig.PoseGap["Rest"] + clamp(s.JawOpen, 0, 1)*(rig.ReferenceGap-rig.PoseGap["Rest"])
		f.jaw = desired - gap
		if p.Presentation != nil {
			// The complete reference is the open endpoint. The source-supported
			// smile below is the closed endpoint; interpolate opening once.
			f.jaw = 0
		}
	}
	for i := range 20 {
		for axis := range 2 {
			f.inner[i][axis] = mix(p.Oral.Inner[f.low][i][axis], p.Oral.Inner[f.high][i][axis], f.weight)
			f.outer[i][axis] = mix(p.Oral.Outer[f.low][i][axis], p.Oral.Outer[f.high][i][axis], f.weight)
		}
	}
	for i := 11; i < 20; i++ {
		f.inner[i][1] += f.jaw * math.Sin(math.Pi*float64(20-i)/10)
		if r.assets.Motion.Mandible != nil {
			f.outer[i][1] += f.jaw * math.Sin(math.Pi*float64(20-i)/10)
		}
	}
	for axis := range 2 {
		f.lower[axis] = mix(p.Oral.LowerTranslation[f.low][axis], p.Oral.LowerTranslation[f.high][axis], f.weight)
		if t := p.Oral.Transition; t != nil {
			f.upper[axis] = mix(t.Upper[f.low][axis], t.Upper[f.high][axis], f.weight)
			f.lower[axis] += mix(t.Lower[f.low][axis], t.Lower[f.high][axis], f.weight)
			f.localContact = true
		}
		for eye := range 2 {
			f.offsets[eye][axis] = mix(p.Eyes.Profiles[f.low].IrisOffsets[eye][axis], p.Eyes.Profiles[f.high].IrisOffsets[eye][axis], f.weight)
		}
	}
	f.lower[1] += f.jaw
	if rig := r.assets.Motion.Mandible; rig != nil {
		f.lower[1] = math.Max(f.upper[1]+rig.LowerStop, f.lower[1])
	}
	if p.Presentation != nil {
		// Oral opening has two immutable source endpoints. Facial lifecycle
		// does not substitute a partially closed reference as a third endpoint.
		for i := range f.inner {
			f.inner[i] = p.Oral.Inner[24][i]
			f.outer[i] = p.Oral.Outer[24][i]
		}
		openInner, openOuter := f.inner, f.outer
		base := r.oral.shadeState
		q := laughterOralWeight(s)
		for i := range f.inner {
			for axis := range 2 {
				f.inner[i][axis] = mix(base.inner[i][axis], f.inner[i][axis], q)
				f.outer[i][axis] = mix(r.oral.outer[i][axis], f.outer[i][axis], q)
			}
		}
		for axis := range 2 {
			f.upper[axis] = base.upper[axis]
			f.lower[axis] = mix(base.lower[axis], p.Oral.LowerTranslation[24][axis], q)
		}
		if rig := r.assets.Motion.Mandible; rig != nil {
			f.lower[1] = math.Max(f.upper[1]+rig.LowerStop, f.lower[1])
		}
		from, to := s.From, s.To
		if from == to {
			if from == "Rest" {
				to = "A"
			} else {
				from = "Rest"
			}
		}
		if s.LaughSpeech {
			return f
		}
		if pair, ok := r.assets.Oral.pairs[from+"-"+to]; ok && pair.Mesh != nil {
			position := func(id int) (final, from, to [2]float64) {
				for axis := range 2 {
					final[axis] = mix(pair.Mesh.Rest[id][axis], pair.Mesh.Open[id][axis], s.Amount)
				}
				from, to = final, final
				if id < 20 {
					final, from, to = f.outer[id], r.oral.outer[id], openOuter[id]
				} else if id < 40 {
					final, from, to = f.inner[id-20], base.inner[id-20], openInner[id-20]
				}
				return
			}
			var valid bool
			f.triangles, valid = sharedLaughterTriangles(pair.Mesh.Triangles, position)
			f.mappingInvalid = !valid
		} else {
			f.mappingInvalid = true
		}
	}
	return f
}

// The facial lifecycle retains amused cheeks and lids through a quiet gap.
// Its mouth instead returns to the existing source-supported closed smile,
// avoiding a compressed open-reference contour and its exposed enamel strip.
func laughterOralWeight(s FrameState) float64 {
	return clamp(s.Laugh, 0, 1) * clamp(s.JawOpen, 0, 1)
}

func referenceJawWeight(x, y float64, f *referenceState, oral Rect, chin float64) float64 {
	left, right := f.outer[0][0]+float64(oral.X), f.outer[10][0]+float64(oral.X)
	width := math.Max(1, right-left)
	x = (x - (left+right)*.5) / (width * .85)
	horizontal := math.Max(0, 1-x*x)
	horizontal *= horizontal
	top := f.outer[5][1] + float64(oral.Y)
	lower := f.outer[15][1] + float64(oral.Y)
	return horizontal * patchSmoothstep(top, lower, y) * (1 - patchSmoothstep(chin, chin+width*.3, y))
}

func (r *Renderer) prepareReference(s FrameState) error {
	p := r.assets.Expression.ReferenceLaughter
	r.referenceChanged = p != nil && (s.Laugh > 0 || r.lastReference > 0)
	if !r.referenceChanged {
		return nil
	}
	defer func() {
		for y := range p.Crop.Height {
			start := ((y+p.Crop.Y)*r.subjectTexture.Width + p.Crop.X) * 4
			copy(r.referencePatch.Pixels.Pixels[y*p.Crop.Width*4:(y+1)*p.Crop.Width*4], r.subjectTexture.Pixels[start:start+p.Crop.Width*4])
		}
	}()
	if s.Laugh <= 0 {
		return nil
	}
	f := r.referenceState(s)
	if f.mappingInvalid {
		return errors.New("laughter material correspondence has no valid shared triangulation")
	}
	a, b := r.assets.ReferenceFrames[f.low], r.assets.ReferenceFrames[f.high]
	base := r.assets.Foreground
	var closed *Texture
	if p.Presentation != nil && !s.LaughSpeech {
		closed = NewTexture(r.mouthPatch.Rect.Width, r.mouthPatch.Rect.Height, true)
		for y := range closed.Height {
			start := ((y+r.mouthPatch.Rect.Y)*r.subjectTexture.Width + r.mouthPatch.Rect.X) * 4
			copy(closed.Pixels[y*closed.Width*4:(y+1)*closed.Width*4], r.subjectTexture.Pixels[start:start+closed.Width*4])
		}
	}
	for y := range p.Crop.Height {
		for x := range p.Crop.Width {
			gx, gy := x+p.Crop.X, y+p.Crop.Y
			sy := float64(gy)
			for range 3 {
				sy = float64(gy) - f.jaw*referenceJawWeight(float64(gx), sy, &f, r.mouthPatch.Rect, r.referenceChin())
			}
			color := patchMixColor(a.Sample(float64(x), sy-float64(p.Crop.Y)), b.Sample(float64(x), sy-float64(p.Crop.Y)), f.weight)
			// Face frames carry original foreground coverage. All samples use the
			// existing source-atop convention before the RGB-D projection.
			alpha := float64(base.Pixels[(gy*base.Width+gx)*4+3]) / 255
			if color[3] > 0 {
				for k := range 3 {
					color[k] *= alpha / color[3]
				}
			}
			color[3] = alpha
			if p.Presentation != nil {
				ownership := oralOutside(float64(gx), float64(gy), r.mouthPatch.Rect, p.Presentation.Feather)
				opening := laughterOralWeight(s)
				color = patchMixColor(r.subjectTexture.Sample(float64(gx), float64(gy)), color, opening+(1-opening)*ownership)
			}
			if gx >= r.mouthPatch.Rect.X && gy >= r.mouthPatch.Rect.Y && gx < r.mouthPatch.Rect.X+r.mouthPatch.Rect.Width && gy < r.mouthPatch.Rect.Y+r.mouthPatch.Rect.Height {
				if closed != nil {
					sx, tx := referenceCoordinates(float64(gx-r.mouthPatch.Rect.X), float64(gy-r.mouthPatch.Rect.Y), f.triangles)
					cx, cy := tx[0]+float64(r.mouthPatch.Rect.X-p.Crop.X)-.5, tx[1]+float64(r.mouthPatch.Rect.Y-p.Crop.Y)-.5
					ref := r.assets.ReferenceFrames[24].Sample(cx, cy)
					if ref[3] > 0 {
						for k := range 3 {
							ref[k] *= alpha / ref[3]
						}
					}
					ref[3] = alpha
					color = patchMixColor(closed.Sample(sx[0]-.5, sx[1]-.5), ref, laughterOralWeight(s))
				}
				color = r.referenceOral(color, float64(gx-r.mouthPatch.Rect.X), float64(gy-r.mouthPatch.Rect.Y), &f)
			}
			if gx >= p.Eyes.Crop.X && gy >= p.Eyes.Crop.Y && gx < p.Eyes.Crop.X+p.Eyes.Crop.Width && gy < p.Eyes.Crop.Y+p.Eyes.Crop.Height {
				color = r.referenceEye(color, gx-p.Eyes.Crop.X, gy-p.Eyes.Crop.Y, &f, s)
			}
			if p.Presentation != nil && s.LaughSpeech {
				weight := oralOutside(float64(gx), float64(gy), r.mouthPatch.Rect, p.Presentation.Feather)
				color = patchMixColor(r.subjectTexture.Sample(float64(gx), float64(gy)), color, weight)
			}
			r.subjectTexture.Set(gx, gy, color)
		}
	}
	return nil
}

func (r *Renderer) referenceOral(base Color, x, y float64, f *referenceState) Color {
	point := [2]float64{x + .5, y + .5}
	inside := false
	distance := math.Inf(1)
	for i, a := range f.inner {
		b := f.inner[(i+1)%20]
		if patchCrosses(point, a, b) {
			inside = !inside
		}
		distance = math.Min(distance, patchDistanceSquared(point, a, b))
	}
	d := math.Sqrt(distance)
	if !inside {
		d = -d
	}
	coverage := patchSmoothstep(-.5, .65, d)
	if coverage <= 0 {
		return base
	}
	color := r.assets.ReferenceCavity.Sample(x, y-f.jaw*.5)
	p := r.assets.Expression.ReferenceLaughter
	gap, referenceGap := f.inner[15][1]-f.inner[5][1], p.Oral.Inner[24][15][1]-p.Oral.Inner[24][5][1]
	if f.localContact {
		gap = referenceColumnGap(point[0], f.inner[:])
		referenceGap = referenceColumnGap(point[0], p.Oral.Inner[24])
	}
	exposure := referenceToothExposure(d, gap, referenceGap, p.Oral.ContactShadow)
	for _, row := range []struct {
		image *Texture
		shift [2]float64
	}{{r.referenceLower(), f.lower}, {r.referenceUpper(), f.upper}} {
		value := row.image.Sample(x-row.shift[0], y-row.shift[1])
		for k := range 3 {
			color[k] = value[k]*exposure + color[k]*(1-value[3])
		}
	}
	for k := range 3 {
		base[k] = mix(base[k], color[k]*base[3], coverage)
	}
	return base
}

func referenceColumnGap(x float64, polygon [][2]float64) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, a := range polygon {
		b := polygon[(i+1)%len(polygon)]
		if (a[0] <= x && x < b[0]) || (b[0] <= x && x < a[0]) {
			y := a[1] + (x-a[0])*(b[1]-a[1])/(b[0]-a[0])
			lo = math.Min(lo, y)
			hi = math.Max(hi, y)
		}
	}
	if math.IsInf(lo, 0) {
		return 0
	}
	return math.Max(0, hi-lo)
}

func (r *Renderer) referenceEye(base Color, x, y int, f *referenceState, s FrameState) Color {
	p := r.assets.Expression.ReferenceLaughter
	a, b := p.Eyes.Profiles[f.low].Curves[x], p.Eyes.Profiles[f.high].Curves[x]
	if a[3] <= 0 && b[3] <= 0 {
		return base
	}
	upper, lower, seam := mix(a[0], b[0], f.weight), mix(a[1], b[1], f.weight), mix(a[2], b[2], f.weight)
	if p.Presentation != nil {
		baseline := s
		baseline.Laugh = 0
		lower -= (r.eyes.curves[x].lower - r.eyes.curves[x].upper) * r.laughSquint(baseline) * (1 - s.Laugh)
	}
	closure := clamp(s.Closure, 0, 1)
	openCoverage := clamp(math.Min(float64(y)+.5, lower)-math.Max(float64(y)-.5, upper), 0, 1)
	closedUpper, closedLower := mix(upper, seam, closure), mix(lower, seam, closure)
	visible := clamp(math.Min(float64(y)+.5, closedLower)-math.Max(float64(y)-.5, closedUpper), 0, 1)
	eye := 0
	if float64(x)+.5 >= r.eyes.eyeSplit {
		eye = 1
	}
	sourceX, sourceY := float64(x)-f.offsets[eye][0], float64(y)-f.offsets[eye][1]
	interior := r.eyes.referenceInterior(sourceX, sourceY, s.Gaze[eye])
	sourceCurve := r.eyes.curves[x]
	height := float64(p.Eyes.Crop.Height)
	band := math.Min(r.eyes.lashBand, math.Min(math.Min(upper, seam), math.Min(height-1-lower, height-1-seam))*.8)
	openY := math.Max(eyeLowerMap(float64(y), closedLower, lower, band, height), lower+.5)
	closedY := eyeLowerMap(float64(y), closedLower, sourceCurve.seam, band, height)
	if float64(y) < closedUpper {
		openY = math.Min(eyeUpperMap(float64(y), closedUpper, upper, band), upper-.5)
		closedY = eyeUpperMap(float64(y), closedUpper, sourceCurve.seam, band)
	}
	gx, gy := float64(x+p.Eyes.Crop.X-p.Crop.X), openY+float64(p.Eyes.Crop.Y-p.Crop.Y)
	openSkin := patchMixColor(r.assets.ReferenceFrames[f.low].Sample(gx, gy), r.assets.ReferenceFrames[f.high].Sample(gx, gy), f.weight)
	closed := r.eyes.closed.Sample(sourceX, closedY)
	skin := patchMixColor(openSkin, closed, closure)
	color := patchMixColor(skin, interior, visible)
	coverage := math.Max(openCoverage, r.eyes.mask.Sample(float64(x), float64(y))[0])
	for k := range 3 {
		base[k] = mix(base[k], color[k]*base[3], coverage)
	}
	return base
}

func (e *EyeRenderer) referenceInterior(x, y float64, gaze [2]float64) Color {
	base := e.opened.Sample(x, y)
	if gaze == [2]float64{} {
		return base
	}
	shifted := e.ownership.Sample(x-gaze[0], y-gaze[1])
	iris := e.irisSample(x-gaze[0], y-gaze[1])
	missing := clamp(shifted[1]-iris[3], 0, 1)
	clean := e.sclera.Sample(x, y)
	for k := range 3 {
		base[k] = iris[k] + base[k]*missing + clean[k]*math.Max(0, 1-iris[3]-missing)
	}
	return base
}

// Only fade the independent manual baseline while entering the laugh. The
// original exact path is retained at zero, including after interruption.
func (r *Renderer) referenceSmile(s FrameState) float64 {
	if !r.referenceActive(s) || r.assets.Expression.ReferenceLaughter.Presentation != nil {
		return s.Smile
	}
	return s.Smile * (1 - patchSmoothstep(0, .15, s.Laugh))
}

func (r *Renderer) referenceChin() float64 {
	if r.assets.Motion.Mandible != nil {
		return float64(r.mouthPatch.Rect.Y + r.mouthPatch.Rect.Height)
	}
	return r.assets.Motion.Anatomy.ChinY
}
