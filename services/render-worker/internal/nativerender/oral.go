package nativerender

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"syntelle/render-contract"
)

// OralAssets contains the immutable, source-bound resources used by the existing
// semantic mouth shader. A renderer owns only its reusable output buffer.
type OralAssets struct {
	Width, Height int
	poses         map[string]oralPose
	pairs         map[string]oralPair
	upper, lower  *Texture
	lighting      oralLighting
}

type oralLighting struct {
	ShadowStrength float64           `json:"contact_shadow_strength"`
	ShadowWidth    float64           `json:"contact_shadow_width_px"`
	DepthStrength  float64           `json:"depth_strength"`
	ClosedGap      float64           `json:"closed_gap_px"`
	Visible        *apertureLighting `json:"visible_aperture"`
}

type apertureLighting struct {
	Version      string  `json:"version"`
	ReferenceGap float64 `json:"reference_gap_px"`
	Minimum      float64 `json:"minimum_irradiance"`
	FullRatio    float64 `json:"full_light_ratio"`
	ContactWidth float64 `json:"contact_width_px"`
	Measurement  string  `json:"measurement"`
}

type oralDefinition struct {
	ID               string    `json:"id"`
	Surface          string    `json:"surface"`
	Mask             string    `json:"mask"`
	Cavity           string    `json:"cavity"`
	ApertureSDF      string    `json:"aperture_sdf"`
	UpperTranslation []float64 `json:"upper_translation"`
	LowerTranslation []float64 `json:"lower_translation"`
}

type oralPose struct {
	definition             oralDefinition
	surface, mask, cavity  *Texture
	upperShift, lowerShift [2]float64
}

type oralMesh struct {
	Width, Height int
	Rest, Open    [][2]float64
	Triangles     [][3]int
}

type oralPair struct {
	Safe bool      `json:"safe"`
	Mesh *oralMesh `json:"mesh"`
}

// NewOralAssets loads the exact resources named by the current preparation
// package. The loader owns decoding/hash verification; no image is generated.
func NewOralAssets(packageJSON []byte, load func(string) (*Texture, error)) (*OralAssets, error) {
	var definition struct {
		Crop      struct{ Width, Height int }
		Reference struct{ Upper, Lower string }
		Oral      *oralLighting
		Poses     map[string]oralDefinition
		Pairs     map[string]oralPair
	}
	if err := json.Unmarshal(packageJSON, &definition); err != nil {
		return nil, fmt.Errorf("decode oral package: %w", err)
	}
	if load == nil || definition.Crop.Width < 1 || definition.Crop.Height < 1 || len(definition.Poses) == 0 {
		return nil, errors.New("oral package has no usable resources")
	}
	assets := &OralAssets{Width: definition.Crop.Width, Height: definition.Crop.Height,
		poses: make(map[string]oralPose, len(definition.Poses)), pairs: definition.Pairs,
		lighting: oralLighting{ShadowStrength: .16, ShadowWidth: 3, DepthStrength: .08, ClosedGap: .8}}
	if definition.Oral != nil {
		assets.lighting = *definition.Oral
	}
	values := []float64{assets.lighting.ShadowStrength, assets.lighting.ShadowWidth, assets.lighting.DepthStrength, assets.lighting.ClosedGap}
	for _, value := range values {
		if !patchFinite(value) {
			return nil, errors.New("oral lighting is not finite")
		}
	}
	if v := assets.lighting.Visible; v != nil {
		if v.Version != "visible-aperture-lighting/1" || v.Measurement != "mask-b-column-coverage/1" ||
			!patchFinite(v.ReferenceGap) || v.ReferenceGap <= 1 || !patchFinite(v.Minimum) || v.Minimum < .2 || v.Minimum > 1 ||
			!patchFinite(v.FullRatio) || v.FullRatio <= 0 || v.FullRatio > 1 || !patchFinite(v.ContactWidth) || v.ContactWidth <= 0 {
			return nil, errors.New("invalid visible aperture lighting")
		}
	}
	read := func(name string) (*Texture, error) {
		if name == "" {
			return nil, errors.New("oral resource name is missing")
		}
		image, err := load(name)
		if err != nil {
			return nil, fmt.Errorf("load oral resource %q: %w", name, err)
		}
		if !patchTextureValid(image) || image.Width != assets.Width || image.Height != assets.Height {
			return nil, fmt.Errorf("oral resource %q has incompatible dimensions", name)
		}
		return image, nil
	}
	upper, err := read(definition.Reference.Upper)
	if err != nil {
		return nil, err
	}
	lower, err := read(definition.Reference.Lower)
	if err != nil {
		return nil, err
	}
	// WebGL uploads these two materials with UNPACK_PREMULTIPLY_ALPHA_WEBGL.
	// Premultiply before filtering, retaining the original immutable textures.
	assets.upper, assets.lower = patchPremultipliedCopy(upper), patchPremultipliedCopy(lower)
	for id, data := range definition.Poses {
		if data.ID != id || len(data.LowerTranslation) != 2 ||
			(len(data.UpperTranslation) != 0 && len(data.UpperTranslation) != 2) {
			return nil, fmt.Errorf("oral pose %q has invalid identity or translation", id)
		}
		pose := oralPose{definition: data}
		pose.surface, err = read(data.Surface)
		if err != nil {
			return nil, err
		}
		pose.mask, err = read(data.Mask)
		if err != nil {
			return nil, err
		}
		pose.cavity = pose.surface
		if data.Cavity != "" {
			pose.cavity, err = read(data.Cavity)
			if err != nil {
				return nil, err
			}
		}
		for axis := range 2 {
			pose.lowerShift[axis] = data.LowerTranslation[axis]
			if len(data.UpperTranslation) != 0 {
				pose.upperShift[axis] = data.UpperTranslation[axis]
			}
			if !patchFinite(pose.lowerShift[axis]) || !patchFinite(pose.upperShift[axis]) {
				return nil, fmt.Errorf("oral pose %q translation is not finite", id)
			}
		}
		assets.poses[id] = pose
	}
	for id, pair := range assets.pairs {
		if !pair.Safe {
			continue
		}
		m := pair.Mesh
		if m == nil || m.Width != assets.Width || m.Height != assets.Height || len(m.Rest) < 40 || len(m.Open) != len(m.Rest) || len(m.Triangles) == 0 {
			return nil, fmt.Errorf("oral pair %q has invalid geometry", id)
		}
		for i := range m.Rest {
			for axis := range 2 {
				if !patchFinite(m.Rest[i][axis]) || !patchFinite(m.Open[i][axis]) {
					return nil, fmt.Errorf("oral pair %q has non-finite vertices", id)
				}
				// The existing vertex buffer stores source positions as float32.
				m.Rest[i][axis] = float64(float32(m.Rest[i][axis]))
				m.Open[i][axis] = float64(float32(m.Open[i][axis]))
			}
		}
		for _, triangle := range m.Triangles {
			for _, i := range triangle {
				if i < 0 || i >= len(m.Rest) {
					return nil, fmt.Errorf("oral pair %q has an invalid triangle", id)
				}
			}
		}
	}
	return assets, nil
}

type OralRenderer struct {
	emotionRig           *EmotionRig
	emotion, lastEmotion rendercontract.EmotionWeights
	laughLips            *Texture
	assets               *OralAssets
	output               *Texture
	lastFrom             string
	lastTo               string
	lastAmount           float64
	lastSmile            float64
	joint                *JointExpression
	laugh, lastLaugh     float64
	hasLast              bool
}

func NewOralRenderer(assets *OralAssets) *OralRenderer {
	return &OralRenderer{assets: assets, output: NewTexture(assets.Width, assets.Height, false)}
}

// Draw returns the renderer-owned straight-alpha image. It remains valid until
// the next changed draw; callers must not mutate it or retain it across draws.
func (renderer *OralRenderer) Draw(fromID, toID string, amount float64) (*Texture, error) {
	return renderer.DrawExpression(fromID, toID, amount, 0)
}

func (renderer *OralRenderer) DrawExpression(fromID, toID string, amount, smile float64) (*Texture, error) {
	return renderer.drawExpression(fromID, toID, amount, smile, nil)
}

// The optional emitter reuses exactly the CPU reference's validated geometry
// and shading state, while pixel evaluation is performed by CUDA.
func (renderer *OralRenderer) drawExpression(fromID, toID string, amount, smile float64, emit func(*oralMesh, [3]int, [3][2]float64, *oralShadeState)) (*Texture, error) {
	if !patchFinite(amount) || amount < 0 || amount > 1 {
		return nil, errors.New("oral interpolation is outside the prepared range")
	}
	if fromID == toID {
		toID = "Rest"
		if fromID == "Rest" {
			toID = "A"
		}
		amount = 0
	}
	amount = float64(float32(amount))
	if emit == nil && renderer.hasLast && renderer.lastFrom == fromID && renderer.lastTo == toID && renderer.lastAmount == amount && renderer.lastSmile == smile && renderer.lastLaugh == renderer.laugh && renderer.lastEmotion == renderer.emotion {
		return renderer.output, nil
	}
	assets := renderer.assets
	from, ok := assets.poses[fromID]
	if !ok {
		return nil, errors.New("oral source pose is unavailable")
	}
	to, ok := assets.poses[toID]
	if !ok {
		return nil, errors.New("oral target pose is unavailable")
	}
	pair, ok := assets.pairs[fromID+"-"+toID]
	if !ok || !pair.Safe || pair.Mesh == nil {
		return nil, errors.New("oral transition has no valid prepared geometry")
	}
	if emit == nil {
		clear(renderer.output.Pixels)
	}
	mesh := pair.Mesh
	var expression *JointPair
	var laughing *JointPair
	if renderer.joint != nil && smile > 0 {
		if !patchFinite(smile) || smile > 1 {
			return nil, errors.New("invalid oral smile amount")
		}
		prepared, ok := renderer.joint.Pairs[fromID+"-"+toID]
		if !ok || len(prepared.RestDelta) != len(mesh.Rest) || len(prepared.OpenDelta) != len(mesh.Open) {
			return nil, errors.New("joint expression does not match oral geometry")
		}
		expression = &prepared
		if renderer.laugh > 0 && renderer.joint.Laugh != nil {
			if renderer.joint.Laugh.Mode == "boundary-laugh/1" && (fromID != "Rest" || toID != "A" || amount > renderer.joint.Laugh.MaximumArticulation+1e-6) {
				return nil, errors.New("laughter is outside its prepared boundary jaw range")
			}
			p, ok := renderer.joint.Laugh.Pairs[fromID+"-"+toID]
			if !ok {
				return nil, errors.New("missing joint laughter geometry")
			}
			laughing = &p
		}
	}
	hasOralEmotion := false
	if renderer.emotionRig != nil && renderer.emotionRig.Oral != nil {
		for index, weight := range renderer.emotion {
			if weight > 0 && renderer.emotionRig.Oral.States[rendercontract.EmotionStates[index]] != nil {
				hasOralEmotion = true
			}
		}
	}
	position := func(id, axis int) float64 {
		value := patchMix(mesh.Rest[id][axis], mesh.Open[id][axis], amount)
		if expression != nil {
			delta := patchMix(expression.RestDelta[id][axis], expression.OpenDelta[id][axis], amount)
			if laughing != nil {
				delta = patchMix(delta, patchMix(laughing.RestDelta[id][axis], laughing.OpenDelta[id][axis], amount), renderer.laugh)
			}
			value += delta * smile
		}
		if renderer.emotionRig != nil && renderer.emotionRig.Oral != nil {
			for state, weight := range renderer.emotion {
				if weight == 0 {
					continue
				}
				deltas, ok := renderer.emotionRig.Oral.States[rendercontract.EmotionStates[state]][fromID+"-"+toID]
				if ok && id < len(deltas.RestDelta) && id < len(deltas.OpenDelta) {
					value += patchMix(deltas.RestDelta[id][axis], deltas.OpenDelta[id][axis], amount) * weight
				}
			}
		}
		return value
	}
	var inner [20][2]float64
	minimum, maximum := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
	for i := range inner {
		for axis := range 2 {
			inner[i][axis] = float64(float32(position(20+i, axis)))
			minimum[axis] = math.Min(minimum[axis], inner[i][axis])
			maximum[axis] = math.Max(maximum[axis], inner[i][axis])
		}
	}
	upper, lower := [2]float64{}, [2]float64{}
	for axis := range 2 {
		upper[axis] = float64(float32(patchMix(from.upperShift[axis], to.upperShift[axis], amount)))
		lower[axis] = float64(float32(patchMix(from.lowerShift[axis], to.lowerShift[axis], amount)))
		if expression != nil {
			translation := renderer.joint.DentalTranslation[axis]
			if laughing != nil {
				translation = patchMix(translation, renderer.joint.Laugh.DentalTranslation[axis], renderer.laugh)
			}
			upper[axis] += translation * smile
			lower[axis] += translation * smile
			opening := renderer.joint.RestLowerTranslation[axis]
			if laughing != nil {
				opening = patchMix(opening, renderer.joint.Laugh.RestLowerTranslation[axis], renderer.laugh)
			}
			fromOpening, toOpening := 0., 0.
			if fromID == "Rest" {
				fromOpening = opening
			}
			if toID == "Rest" {
				toOpening = opening
			}
			lower[axis] += patchMix(fromOpening, toOpening, amount) * smile
		}
	}
	cavityAmount := amount
	if fromID == "Rest" || fromID == "MBP" {
		cavityAmount = 1
	} else if toID == "Rest" || toID == "MBP" {
		cavityAmount = 0
	}
	state := oralShadeState{from: from, to: to, amount: amount, cavityAmount: cavityAmount,
		inner: inner, minimum: minimum, maximum: maximum, upper: upper, lower: lower,
		semantic: from.definition.ApertureSDF != "" && to.definition.ApertureSDF != ""}
	// Emotional lip geometry also moves source ownership, even when this
	// emotion supplies no smile. Endpoint UV shortcuts require a neutral mesh.
	state.joint = expression != nil || hasOralEmotion
	state.deformedCoverage = state.joint && renderer.emotionRig != nil && renderer.emotionRig.EditCoverage == "deformed-oral-coverage/1"
	if hasOralEmotion {
		state.semantic = false
	}
	if expression != nil && (fromID == "Rest" || toID == "Rest") {
		state.semantic = false
	}
	triangles := mesh.Triangles
	if expression != nil {
		triangles = expression.Triangles
	}
	if laughing != nil && renderer.joint.Laugh.Mode == "boundary-laugh/1" {
		triangles = laughing.Triangles
	}
	if hasOralEmotion {
		if prepared := renderer.emotionRig.Oral.Triangles[fromID+"-"+toID]; len(prepared) > 0 {
			triangles = prepared
		}
	}
	for _, ids := range triangles {
		var points [3][2]float64
		for vertex, id := range ids {
			for axis := range 2 {
				points[vertex][axis] = position(id, axis)
			}
		}
		if emit != nil {
			emit(mesh, ids, points, &state)
		} else {
			renderer.drawTriangle(mesh, ids, points, &state)
		}
	}
	if emit != nil {
		return nil, nil
	}
	renderer.lastFrom, renderer.lastTo, renderer.lastAmount, renderer.hasLast, renderer.lastSmile = fromID, toID, amount, true, smile
	renderer.lastLaugh = renderer.laugh
	renderer.lastEmotion = renderer.emotion
	return renderer.output, nil
}

type oralShadeState struct {
	deformedCoverage     bool
	joint                bool
	from, to             oralPose
	amount, cavityAmount float64
	inner                [20][2]float64
	minimum, maximum     [2]float64
	upper, lower         [2]float64
	semantic             bool
}

func (renderer *OralRenderer) drawTriangle(mesh *oralMesh, ids [3]int, p [3][2]float64, state *oralShadeState) {
	area := patchEdge(p[0], p[1], p[2])
	if math.Abs(area) < 1e-10 {
		return
	}
	if area < 0 {
		p[1], p[2], ids[1], ids[2] = p[2], p[1], ids[2], ids[1]
		area = -area
	}
	minX := max(0, int(math.Ceil(math.Min(p[0][0], math.Min(p[1][0], p[2][0]))-.5)))
	maxX := min(mesh.Width-1, int(math.Floor(math.Max(p[0][0], math.Max(p[1][0], p[2][0]))-.5)))
	minY := max(0, int(math.Ceil(math.Min(p[0][1], math.Min(p[1][1], p[2][1]))-.5)))
	maxY := min(mesh.Height-1, int(math.Floor(math.Max(p[0][1], math.Max(p[1][1], p[2][1]))-.5)))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			point := [2]float64{float64(x) + .5, float64(y) + .5}
			a := patchEdge(p[1], p[2], point)
			b := patchEdge(p[2], p[0], point)
			c := patchEdge(p[0], p[1], point)
			if !patchInside(a, p[1], p[2]) || !patchInside(b, p[2], p[0]) || !patchInside(c, p[0], p[1]) {
				continue
			}
			at := (y*mesh.Width + x) * 4
			alpha := patchMix(float64(state.from.mask.Pixels[at]), float64(state.to.mask.Pixels[at]), state.amount) / 255
			// The output is applied source-atop. A zero stored alpha contributes
			// no RGB to the character, so do not calculate invisible fragments.
			if !state.deformedCoverage && alpha < .5/255 {
				continue
			}
			a, b, c = a/area, b/area, c/area
			from := [2]float64{mesh.Rest[ids[0]][0]*a + mesh.Rest[ids[1]][0]*b + mesh.Rest[ids[2]][0]*c,
				mesh.Rest[ids[0]][1]*a + mesh.Rest[ids[1]][1]*b + mesh.Rest[ids[2]][1]*c}
			to := [2]float64{mesh.Open[ids[0]][0]*a + mesh.Open[ids[1]][0]*b + mesh.Open[ids[2]][0]*c,
				mesh.Open[ids[0]][1]*a + mesh.Open[ids[1]][1]*b + mesh.Open[ids[2]][1]*c}
			if state.amount == 0 && !state.joint {
				from = point
			}
			if state.amount == 1 && !state.joint {
				to = point
			}
			if state.deformedCoverage {
				// Edit ownership follows the same source coordinates as the lip
				// texture. A stationary destination mask clips moving corners.
				alpha = patchMix(patchChannel(state.from.mask, from[0]-.5, from[1]-.5, 0), patchChannel(state.to.mask, to[0]-.5, to[1]-.5, 0), state.amount) / 255
				if alpha < .5/255 {
					continue
				}
			}
			renderer.output.Set(x, y, renderer.shade(x, y, from, to, alpha, state))
		}
	}
}

func (renderer *OralRenderer) shade(x, y int, fromUV, toUV [2]float64, alpha float64, state *oralShadeState) Color {
	var color Color
	if state.amount == 0 {
		color = state.from.surface.Sample(fromUV[0]-.5, fromUV[1]-.5)
	} else if state.amount == 1 {
		color = state.to.surface.Sample(toUV[0]-.5, toUV[1]-.5)
	} else {
		from := state.from.surface.Sample(fromUV[0]-.5, fromUV[1]-.5)
		to := state.to.surface.Sample(toUV[0]-.5, toUV[1]-.5)
		color = patchMixColor(from, to, state.amount)
	}
	if renderer.laughLips != nil && renderer.laugh > 0 {
		lip := renderer.laughLips.Sample(fromUV[0]-.5, fromUV[1]-.5)
		for channel := range 3 {
			color[channel] = patchMix(color[channel], lip[channel], lip[3]*renderer.laugh)
		}
	}
	px, py := float64(x)+.5, float64(y)+.5
	lighting := renderer.assets.lighting
	gap := state.inner[15][1] - state.inner[5][1]
	if state.semantic || (px >= state.minimum[0]-1 && py >= state.minimum[1]-1 && px <= state.maximum[0]+1 && py <= state.maximum[1]+1 && gap > lighting.ClosedGap) {
		coverage := 0.0
		apertureDistance := 0.0
		openingRatio := 0.0
		if state.semantic {
			var distance float64
			if state.amount == 0 {
				distance = (patchChannel(state.from.mask, fromUV[0]-.5, fromUV[1]-.5, 1) - 128) / 8
			} else if state.amount == 1 {
				distance = (patchChannel(state.to.mask, toUV[0]-.5, toUV[1]-.5, 1) - 128) / 8
			} else {
				a := (patchChannel(state.from.mask, fromUV[0]-.5, fromUV[1]-.5, 1) - 128) / 8
				b := (patchChannel(state.to.mask, toUV[0]-.5, toUV[1]-.5, 1) - 128) / 8
				distance = patchMix(a, b, state.amount)
			}
			coverage = patchSmoothstep(-.5, .5, distance)
			apertureDistance = math.Max(0, distance)
			if lighting.Visible != nil {
				openingRatio = patchMix(patchChannel(state.from.mask, fromUV[0]-.5, fromUV[1]-.5, 2), patchChannel(state.to.mask, toUV[0]-.5, toUV[1]-.5, 2), state.amount) / 255
			}
		} else {
			point := [2]float64{px, py}
			edge := patchDistanceSquared(point, state.inner[19], state.inner[0])
			inside := patchCrosses(point, state.inner[19], state.inner[0])
			for i := 0; i < 19; i++ {
				edge = math.Min(edge, patchDistanceSquared(point, state.inner[i], state.inner[i+1]))
				if patchCrosses(point, state.inner[i], state.inner[i+1]) {
					inside = !inside
				}
			}
			distance := math.Sqrt(edge)
			if !inside {
				distance = -distance
			}
			coverage = patchSmoothstep(-.5, .65, distance)
			apertureDistance = math.Max(0, distance)
			if lighting.Visible != nil {
				openingRatio = math.Max(0, gap) / lighting.Visible.ReferenceGap
			}
		}
		if coverage > 0 && (state.semantic || gap > lighting.ClosedGap) {
			var interior Color
			if state.cavityAmount == 0 {
				interior = state.from.cavity.Sample(fromUV[0]-.5, fromUV[1]-.5)
			} else if state.cavityAmount == 1 {
				interior = state.to.cavity.Sample(toUV[0]-.5, toUV[1]-.5)
			} else {
				fromCavity := state.from.cavity.Sample(fromUV[0]-.5, fromUV[1]-.5)
				toCavity := state.to.cavity.Sample(toUV[0]-.5, toUV[1]-.5)
				interior = patchMixColor(fromCavity, toCavity, state.cavityAmount)
			}
			upper := renderer.assets.upper.Sample(float64(x)-state.upper[0], float64(y)-state.upper[1])
			lower := renderer.assets.lower.Sample(float64(x)-state.lower[0], float64(y)-state.lower[1])
			for channel := range 3 {
				upper[channel] /= math.Max(upper[3], .00001)
				lower[channel] /= math.Max(lower[3], .00001)
			}
			if upper[3] > 0 || lower[3] > 0 {
				point := [2]float64{px, py}
				edge := math.Min(1e8, patchDistanceSquared(point, state.inner[19], state.inner[0]))
				for i := 0; i < 19; i++ {
					edge = math.Min(edge, patchDistanceSquared(point, state.inner[i], state.inner[i+1]))
				}
				xDepth := math.Abs(px-(state.inner[0][0]+state.inner[10][0])*.5) / math.Max(1, (state.inner[10][0]-state.inner[0][0])*.5)
				depth := 1 - lighting.DepthStrength*xDepth*xDepth
				minimum := .68
				if state.semantic {
					minimum = .22
				}
				illumination := math.Max(minimum, math.Min(1, depth-lighting.ShadowStrength*math.Exp(-math.Sqrt(edge)/math.Max(.1, lighting.ShadowWidth))))
				if v := lighting.Visible; v != nil {
					ambient := patchMix(v.Minimum, 1, patchSmoothstep(0, v.FullRatio, openingRatio))
					illumination = math.Max(.12, math.Min(1, depth-lighting.ShadowStrength*math.Exp(-apertureDistance/v.ContactWidth))) * ambient
				}
				exposure := math.Pow(illumination, 1/2.2)
				for channel := range 3 {
					upper[channel] *= exposure
					lower[channel] *= exposure
				}
			}
			for channel := range 3 {
				oral := patchMix(interior[channel], lower[channel], lower[3])
				oral = patchMix(oral, upper[channel], upper[3])
				color[channel] = patchMix(color[channel], oral, coverage)
			}
		}
	}
	// Edit alpha is resolved in its selected ownership domain before shading.
	color[3] = alpha
	return color
}

// patchChannel returns one bilinearly filtered byte channel, avoiding three
// unused channel interpolations for the semantic aperture distance.
func patchChannel(texture *Texture, x, y float64, channel int) float64 {
	if x < 0 {
		x = 0
	} else if x > float64(texture.Width-1) {
		x = float64(texture.Width - 1)
	}
	if y < 0 {
		y = 0
	} else if y > float64(texture.Height-1) {
		y = float64(texture.Height - 1)
	}
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, texture.Width-1), min(y0+1, texture.Height-1)
	a, b := (y0*texture.Width+x0)*4+channel, (y0*texture.Width+x1)*4+channel
	c, d := (y1*texture.Width+x0)*4+channel, (y1*texture.Width+x1)*4+channel
	fx, fy := x-float64(x0), y-float64(y0)
	top := float64(texture.Pixels[a]) + (float64(texture.Pixels[b])-float64(texture.Pixels[a]))*fx
	bottom := float64(texture.Pixels[c]) + (float64(texture.Pixels[d])-float64(texture.Pixels[c]))*fx
	return top + (bottom-top)*fy
}

func patchMask(texture *Texture, x, y float64, linear bool) Color {
	if linear {
		return texture.Sample(x, y)
	}
	xi := max(0, min(texture.Width-1, int(math.Floor(x+.5))))
	yi := max(0, min(texture.Height-1, int(math.Floor(y+.5))))
	i := (yi*texture.Width + xi) * 4
	return Color{float64(texture.Pixels[i]) / 255, float64(texture.Pixels[i+1]) / 255, float64(texture.Pixels[i+2]) / 255, float64(texture.Pixels[i+3]) / 255}
}

func patchPremultipliedCopy(source *Texture) *Texture {
	result := NewTexture(source.Width, source.Height, true)
	copy(result.Pixels, source.Pixels)
	if !source.Premultiplied {
		for i := 0; i < len(result.Pixels); i += 4 {
			alpha := int(result.Pixels[i+3])
			for channel := range 3 {
				result.Pixels[i+channel] = byte((int(result.Pixels[i+channel])*alpha + 127) / 255)
			}
		}
	}
	return result
}

func patchTextureValid(texture *Texture) bool {
	return texture != nil && texture.Width > 0 && texture.Height > 0 && len(texture.Pixels) == texture.Width*texture.Height*4
}

func patchFinite(value float64) bool        { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func patchMix(a, b, amount float64) float64 { return a*(1-amount) + b*amount }
func patchMixColor(a, b Color, amount float64) Color {
	return Color{patchMix(a[0], b[0], amount), patchMix(a[1], b[1], amount), patchMix(a[2], b[2], amount), patchMix(a[3], b[3], amount)}
}
func patchSmoothstep(a, b, value float64) float64 {
	t := math.Max(0, math.Min(1, (value-a)/(b-a)))
	return t * t * (3 - 2*t)
}
func patchEdge(a, b, p [2]float64) float64 {
	return (b[0]-a[0])*(p[1]-a[1]) - (b[1]-a[1])*(p[0]-a[0])
}
func patchInside(edge float64, a, b [2]float64) bool {
	return edge > 0 || edge == 0 && (b[1] < a[1] || b[1] == a[1] && b[0] > a[0])
}
func patchDistanceSquared(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	t := math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dy)/math.Max(dx*dx+dy*dy, .0001)))
	x, y := p[0]-a[0]-t*dx, p[1]-a[1]-t*dy
	return x*x + y*y
}
func patchCrosses(p, a, b [2]float64) bool {
	return (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0]
}
