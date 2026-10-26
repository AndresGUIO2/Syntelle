package nativerender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"slices"
	"sync/atomic"
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeframe"
)

// FrameState contains presentation state, never wall-clock callback arrival.
type FrameState struct {
	Emotion         rendercontract.EmotionWeights `json:"emotion,omitempty"`
	Head            [3]float64                    `json:"head"`
	Breath          float64                       `json:"breath"`
	From, To        string
	Amount, Closure float64
	Gaze            [2][2]float64
	Smile, Brows    float64
	Laugh           float64
	// RestingSmile preserves the registered, closed-mouth manual expression.
	RestingSmile bool
	Background   string
	Blur         float64
}
type rasterVertex struct{ X, Y, Z, U, V, Q, Head, Torso float64 }
type patch struct {
	Rect   Rect
	Pixels *Texture
}
type sampleAxis struct {
	A, B  int
	F     float64
	Valid bool
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func fmtSortKey(yaw, pitch, roll float64) string {
	return fmt.Sprintf("%.1f,%.1f,%.1f", yaw*180/math.Pi, pitch*180/math.Pi, roll*180/math.Pi)
}

type Renderer struct {
	inspectionState    FrameState
	hasInspectionState bool
	assets             *Assets
	oral               *OralRenderer
	eyes               *EyeRenderer
	body               *Texture
	subjectTexture     *Texture
	expressionBase     *Texture
	expressionPatch    patch
	skinChanged        bool
	lastSkinSmile      float64
	lastSkinLaugh      float64
	lastReference      float64
	referencePatch     patch
	referenceChanged   bool
	depth              []float64
	vertices           []rasterVertex
	fringe             []fringeTriangle
	lastSort           string
	lastMouth          struct {
		Emotion  rendercontract.EmotionWeights
		From, To string
		Amount   float64
	}
	lastEye struct {
		Emotion rendercontract.EmotionWeights
		Laugh   float64
		Closure float64
		Gaze    [2][2]float64
	}
	initialized                 bool
	mouthPatch, eyePatch        patch
	background                  *Texture
	backgroundID                string
	backgroundBlur              float64
	left, top, right, bottom    int
	x, y, drawWidth, drawHeight float64
	composeX                    [720]sampleAxis
	composeY                    [960]sampleAxis
	composeNative               composeBackend
	gpu                         gpuRenderer
	gpuRequired, closed         bool
	gpuActive                   atomic.Bool
}

func New(a *Assets) (*Renderer, error) {
	return newRenderer(a, os.Getenv("SYNTELLE_RENDERER"))
}

func newRenderer(a *Assets, mode string) (*Renderer, error) {
	if a == nil || a.Foreground == nil {
		return nil, errors.New("native assets unavailable")
	}
	eyes, err := NewEyeRenderer(a.Opened, a.Closed, a.EyeMask, a.Sclera, a.Ownership, a.Motion.Eyes.Curves, a.EyeSplit)
	if err != nil {
		return nil, err
	}
	scale := math.Min(1, 1024/float64(max(a.Motion.Canvas.Width, a.Motion.Canvas.Height)))
	w, h := int(math.Round(float64(a.Motion.Canvas.Width)*scale)), int(math.Round(float64(a.Motion.Canvas.Height)*scale))
	r := &Renderer{assets: a, oral: NewOralRenderer(a.Oral), eyes: eyes, body: NewTexture(w, h, true), depth: make([]float64, w*h), vertices: make([]rasterVertex, len(a.vertices)), fringe: slices.Clone(a.fringe)}
	r.subjectTexture = NewTexture(a.Foreground.Width, a.Foreground.Height, true)
	copy(r.subjectTexture.Pixels, a.Foreground.Pixels)
	var oral struct{ Crop Rect }
	if err = jsonUnmarshal(a.OralJSON, &oral); err != nil {
		return nil, err
	}
	r.mouthPatch = r.newPatch(oral.Crop)
	if a.Expression.Joint != nil {
		r.oral.emotionRig = a.Expression.EmotionRig
		r.oral.joint = a.Expression.Joint
		r.oral.laughLips = a.LaughLips
		r.expressionPatch = r.newPatch(a.Expression.Crop)
		r.expressionBase = a.Foreground.Premultiply()
	}
	r.eyePatch = r.newPatch(a.Motion.Eyes.Crop)
	if a.Expression.ReferenceLaughter != nil {
		r.referencePatch = r.newPatch(a.Expression.ReferenceLaughter.Crop)
	}
	portraitHeight := a.Motion.Anatomy.FaceHeight * 2.6
	bottom := math.Min(float64(a.Motion.Canvas.Height), a.Motion.Anatomy.ChinY+a.Motion.Anatomy.FaceHeight*.9)
	r.drawWidth = 960 * float64(a.Motion.Canvas.Width) / portraitHeight
	r.drawHeight = 960 * float64(a.Motion.Canvas.Height) / portraitHeight
	r.x = (720 - r.drawWidth) / 2
	r.y = 960 * (portraitHeight - bottom) / portraitHeight
	// Same two-raster-pixel guard used by the preceding private renderer.
	sx, sy := float64(w)/r.drawWidth, float64(h)/r.drawHeight
	r.left = max(0, int(math.Floor(-r.x*sx))-2)
	r.top = max(0, int(math.Floor(-r.y*sy))-2)
	r.right = min(w, int(math.Ceil((720-r.x)*sx))+2)
	r.bottom = min(h, int(math.Ceil((960-r.y)*sy))+2)
	axis := func(position float64, size, stride int) sampleAxis {
		if position < -.5 || position >= float64(size)-.5 {
			return sampleAxis{}
		}
		position = clamp(position, 0, float64(size-1))
		low := int(position)
		return sampleAxis{A: low * stride, B: min(low+1, size-1) * stride, F: position - float64(low), Valid: true}
	}
	for x := range 720 {
		r.composeX[x] = axis((float64(x)+.5-r.x)*float64(w)/r.drawWidth-.5, w, 4)
	}
	for y := range 960 {
		r.composeY[y] = axis((float64(y)+.5-r.y)*float64(h)/r.drawHeight-.5, h, w*4)
	}
	if err := r.selectGPU(mode); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Renderer) newPatch(rect Rect) patch {
	return patch{Rect: rect, Pixels: NewTexture(rect.Width, rect.Height, true)}
}

// patchOver preserves original foreground alpha, exactly as the previous
// source-atop patch operation. Prepared source bytes remain immutable.
func (r *Renderer) patchOver(p patch, overlay *Texture) {
	base := r.assets.Foreground
	if r.expressionBase != nil {
		base = r.expressionBase
	}
	for y := range p.Rect.Height {
		for x := range p.Rect.Width {
			i := (y*p.Rect.Width + x) * 4
			b := ((y+p.Rect.Y)*r.assets.Foreground.Width + x + p.Rect.X) * 4
			a := float64(overlay.Pixels[i+3]) / 255
			ba := float64(base.Pixels[b+3]) / 255
			for k := range 3 {
				v := float64(overlay.Pixels[i+k]) / 255
				if overlay.Premultiplied && a > 0 {
					v /= a
				}
				p.Pixels.Pixels[i+k] = byteColor(v*a*ba + float64(base.Pixels[b+k])/255*(1-a))
			}
			p.Pixels.Pixels[i+3] = r.assets.Foreground.Pixels[b+3]
			copy(r.subjectTexture.Pixels[b:b+4], p.Pixels.Pixels[i:i+4])
		}
	}
}

func (r *Renderer) subject(x, y float64) Color {
	return r.subjectTexture.Sample(x, y)
}

func (r *Renderer) Render(ctx context.Context, state FrameState, dst *image.RGBA) error {
	if r.closed {
		return errors.New("native renderer is closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if dst == nil || dst.Rect.Min != (image.Point{}) || (dst.Rect.Dx() != 720 && dst.Rect.Dx() != 1440) || dst.Rect.Dy() != 960 {
		return errors.New("native output must be 720x960 or packed 1440x960")
	}
	if state.From == "" {
		state.From = "Rest"
	}
	if state.To == "" {
		state.To = "A"
	}
	if state.Background == "" {
		state.Background = "light-studio"
	}
	if r.assets.Expression.Joint != nil {
		state.Smile = math.Round(clamp(state.Smile, 0, 1)*120) / 120
		state.Laugh = math.Round(clamp(state.Laugh, 0, 1)*120) / 120
	}
	r.inspectionState = state
	r.hasInspectionState = true
	if full, ok := r.gpu.(fullGPURenderer); ok {
		if err := full.RenderAll(ctx, r, state, dst, nil); err == nil {
			return nil
		} else if ctx.Err() != nil || r.gpuRequired {
			return err
		}
		r.disableGPU()
	}
	if r.lastReference > 0 && !r.referenceActive(state) {
		rect := r.assets.Expression.ReferenceLaughter.Crop
		for y := range rect.Height {
			start := ((y+rect.Y)*r.subjectTexture.Width + rect.X) * 4
			copy(r.subjectTexture.Pixels[start:start+rect.Width*4], r.assets.Foreground.Pixels[start:start+rect.Width*4])
		}
		r.initialized = false
	}
	jointSmile := r.jointSmile(state)
	legacyLaugh := state.Laugh
	if r.referenceActive(state) {
		legacyLaugh = 0
	}
	r.skinChanged = r.prepareSkin(jointSmile, legacyLaugh)
	mouthChanged := r.skinChanged || !r.initialized || r.lastMouth.From != state.From || r.lastMouth.To != state.To || r.lastMouth.Amount != state.Amount || r.lastMouth.Emotion != state.Emotion
	if mouthChanged {
		r.oral.laugh = legacyLaugh
		r.oral.emotion = state.Emotion
		oral, err := r.oral.DrawExpression(state.From, state.To, state.Amount, jointSmile)
		if err != nil {
			return err
		}
		r.patchOver(r.mouthPatch, oral)
		r.lastMouth.From, r.lastMouth.To, r.lastMouth.Amount = state.From, state.To, state.Amount
		r.lastMouth.Emotion = state.Emotion
	}
	eyesChanged := r.skinChanged || !r.initialized || r.lastEye.Closure != state.Closure || r.lastEye.Gaze != state.Gaze || r.lastEye.Laugh != state.Laugh || r.lastEye.Emotion != state.Emotion
	if eyesChanged {
		r.patchOver(r.eyePatch, r.eyes.DrawExpression(state.Closure, state.Gaze, r.laughSquint(state)))
		r.lastEye.Closure, r.lastEye.Gaze, r.lastEye.Laugh = state.Closure, state.Gaze, state.Laugh
		r.lastEye.Emotion = state.Emotion
	}
	r.prepareReference(state)
	r.lastReference = state.Laugh
	r.initialized = true
	backgroundChanged := dst.Rect.Dx() == 720 && (r.background == nil || r.backgroundID != state.Background || r.backgroundBlur != state.Blur)
	if backgroundChanged {
		background, err := r.makeBackground(state.Background, state.Blur)
		if err != nil {
			return err
		}
		r.background = background
		r.backgroundID = state.Background
		r.backgroundBlur = state.Blur
	}
	if rendered, err := r.renderGPU(ctx, state, dst, mouthChanged, eyesChanged, backgroundChanged); rendered || err != nil {
		return err
	}
	if err := r.drawBody(ctx, state); err != nil {
		return err
	}
	if dst.Rect.Dx() == 1440 {
		return r.packForeground(ctx, dst)
	}
	return r.compose(ctx, dst)
}

// RenderFrame borrows an encoder-owned CUDA surface for this one frame. CPU
// fallback draws the same sampled animation state without advancing its clock.
func (r *Renderer) RenderFrame(ctx context.Context, s FrameState, dst *image.RGBA, target *nativeframe.Surface) (bool, error) {
	if r.closed {
		return false, errors.New("native renderer is closed")
	}
	if s.From == "" {
		s.From = "Rest"
	}
	if s.To == "" {
		s.To = "A"
	}
	if s.Background == "" {
		s.Background = "light-studio"
	}
	if r.assets.Expression.Joint != nil {
		s.Smile = math.Round(clamp(s.Smile, 0, 1)*120) / 120
		s.Laugh = math.Round(clamp(s.Laugh, 0, 1)*120) / 120
	}
	r.inspectionState = s
	r.hasInspectionState = true
	if full, ok := r.gpu.(fullGPURenderer); ok && target != nil {
		if err := full.RenderAll(ctx, r, s, dst, target); err == nil {
			return true, nil
		} else if ctx.Err() != nil || r.gpuRequired {
			return false, err
		}
		r.disableGPU()
	}
	return false, r.Render(ctx, s, dst)
}

func (r *Renderer) Readback(ctx context.Context, dst *image.RGBA) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if full, ok := r.gpu.(fullGPURenderer); ok && r.hasInspectionState {
		if err := full.Readback(dst); err == nil {
			return nil
		} else if r.gpuRequired {
			return err
		}
		r.disableGPU()
		return r.Render(ctx, r.inspectionState, dst)
	}
	return nil // The CPU path already populated the session-owned frame.
}

func (r *Renderer) compose(ctx context.Context, dst *image.RGBA) error {
	return r.composePixels(ctx, dst)
}

func (r *Renderer) composeScalar(ctx context.Context, dst *image.RGBA) error {
	pixels := r.body.Pixels
	for y := range 960 {
		if y%32 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		yaxis := r.composeY[y]
		for x := range 720 {
			i := y*dst.Stride + x*4
			bg := (y*720 + x) * 4
			xaxis := r.composeX[x]
			if !yaxis.Valid || !xaxis.Valid {
				copy(dst.Pix[i:i+4], r.background.Pixels[bg:bg+4])
				continue
			}
			a, b, c, d := yaxis.A+xaxis.A, yaxis.A+xaxis.B, yaxis.B+xaxis.A, yaxis.B+xaxis.B
			// Four bounded texel views let the compiler prove channel accesses
			// once per pixel, instead of checking each bilinear channel lookup.
			pa, pb, pc, pd := pixels[a:a+4], pixels[b:b+4], pixels[c:c+4], pixels[d:d+4]
			if pa[3]|pb[3]|pc[3]|pd[3] == 0 {
				copy(dst.Pix[i:i+4], r.background.Pixels[bg:bg+4])
				continue
			}
			remaining := 0.
			if pa[3]&pb[3]&pc[3]&pd[3] != 255 {
				top := mix(float64(pa[3]), float64(pb[3]), xaxis.F)
				bottom := mix(float64(pc[3]), float64(pd[3]), xaxis.F)
				remaining = 1 - mix(top, bottom, yaxis.F)/255
			}
			background, output := r.background.Pixels[bg:bg+4], dst.Pix[i:i+4]
			for k := range 3 {
				top := mix(float64(pa[k]), float64(pb[k]), xaxis.F)
				bottom := mix(float64(pc[k]), float64(pd[k]), xaxis.F)
				v := mix(top, bottom, yaxis.F) + float64(background[k])*remaining
				output[k] = byte(min(255., v+.5))
			}
			output[3] = 255
		}
	}
	return nil
}

func (r *Renderer) prepareBody(s FrameState) bool {
	p := r.assets.Motion.Projection
	c := p.Camera
	scale := math.Max(1, r.assets.MotionScale)
	yaw, pitch, roll := clamp(s.Head[0], -p.Bounds.Yaw*scale, p.Bounds.Yaw*scale)*math.Pi/180, clamp(s.Head[1], -p.Bounds.Pitch*scale, p.Bounds.Pitch*scale)*math.Pi/180, clamp(s.Head[2], -p.Bounds.Roll*scale, p.Bounds.Roll*scale)*math.Pi/180
	cy, sy, cp, sp, cr, sr := math.Cos(yaw), math.Sin(yaw), math.Cos(pitch), math.Sin(pitch), math.Cos(roll), math.Sin(roll)
	reference := yaw == 0 && pitch == 0 && roll == 0
	w, h := float64(r.assets.Motion.Canvas.Width), float64(r.assets.Motion.Canvas.Height)
	for i, v := range r.assets.vertices {
		sourceQ := c.Distance - v.Z
		ox, oy := v.X*w, v.Y*h
		world := vector3{(ox - c.Center[0]) * sourceQ / c.Focal, (oy - c.Center[1]) * sourceQ / c.Focal, v.Z}
		x, y, z := world[0]-p.Pivot[0], world[1]-p.Pivot[1], world[2]-p.Pivot[2]
		ax, az := cy*x+sy*z, -sy*x+cy*z
		by, bz := cp*y-sp*az, sp*y+cp*az
		world[0] += v.Head * (cr*ax - sr*by - x)
		world[1] += v.Head * (sr*ax + cr*by - y)
		world[2] += v.Head * (bz - z)
		q := c.Distance - world[2]
		px, py := world[0]*c.Focal/q+c.Center[0], world[1]*c.Focal/q+c.Center[1]
		if reference || v.Head == 0 {
			px, py = ox, oy
		}
		py += v.Torso * s.Breath
		px += v.Torso * (ox - c.PivotPx[0]) * s.Breath / h * .25
		targetW, sourceW := 1., 1.
		if !reference {
			targetW = mix(1, q/c.Distance, v.Head)
			sourceW = mix(1, sourceQ/c.Distance, v.Head)
		}
		zndc := (c.Far+c.Near)/(c.Far-c.Near) - 2*c.Far*c.Near/(c.Far-c.Near)/q
		r.vertices[i] = rasterVertex{X: px / w * float64(r.body.Width), Y: py / h * float64(r.body.Height), Z: zndc, U: v.X * sourceW / targetW, V: v.Y * sourceW / targetW, Q: sourceW / targetW, Head: v.Head, Torso: v.Torso}
	}
	key := fmtSortKey(yaw, pitch, roll)
	if key != r.lastSort {
		for i := range r.fringe {
			t := &r.fringe[i]
			t.Depth = c.Distance - (t.Z + sp*t.Weighted[1] + cp*(-sy*t.Weighted[0]+cy*t.Weighted[2]) - t.Weighted[2])
		}
		slices.SortStableFunc(r.fringe, func(a, b fringeTriangle) int {
			if a.Depth > b.Depth {
				return -1
			}
			if a.Depth < b.Depth {
				return 1
			}
			return 0
		})
		r.lastSort = key
	}
	return reference
}

func (r *Renderer) drawBody(ctx context.Context, s FrameState) error {
	reference := r.prepareBody(s)
	for y := r.top; y < r.bottom; y++ {
		start := (y*r.body.Width + r.left) * 4
		clear(r.body.Pixels[start : (y*r.body.Width+r.right)*4])
		for x := r.left; x < r.right; x++ {
			r.depth[y*r.body.Width+x] = math.Inf(1)
		}
	}
	for i := 0; i < len(r.assets.indices); i += 3 {
		if i%300 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		r.triangle(r.assets.indices[i], r.assets.indices[i+1], r.assets.indices[i+2], false, reference, &s)
	}
	for _, t := range r.fringe {
		r.triangle(t.IDs[0], t.IDs[1], t.IDs[2], true, reference, &s)
	}
	return nil
}

func edge(a, b rasterVertex, x, y float64) float64 { return (x-a.X)*(b.Y-a.Y) - (y-a.Y)*(b.X-a.X) }
func topLeft(a, b rasterVertex) bool               { return b.Y < a.Y || (b.Y == a.Y && b.X > a.X) }

func (r *Renderer) triangle(ia, ib, ic uint16, boundary, reference bool, s *FrameState) {
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
	if x0 > x1 || y0 > y1 {
		return
	}
	inv := 1 / area
	tl0, tl1, tl2 := topLeft(b, c), topLeft(c, a), topLeft(a, b)
	sourceAligned := (reference && (s.Breath == 0 || a.Torso == 0 && b.Torso == 0 && c.Torso == 0)) || (a.Head == 0 && b.Head == 0 && c.Head == 0 && s.Breath == 0)
	for y := y0; y <= y1; y++ {
		py := float64(y) + .5
		for x := x0; x <= x1; x++ {
			px := float64(x) + .5
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
			var u, v float64
			if sourceAligned {
				u = px / float64(r.body.Width)
				v = py / float64(r.body.Height)
			} else {
				inverseQ := 1 / (wa*a.Q + wb*b.Q + wc*c.Q)
				u, v = (wa*a.U+wb*b.U+wc*c.U)*inverseQ, (wa*a.V+wb*b.V+wc*c.V)*inverseQ
			}
			color := r.fragment(u, v, s)
			if boundary {
				if color[3] >= .995 || color[3] <= .001 {
					continue
				}
				at := index * 4
				for k := range 4 {
					color[k] += float64(r.body.Pixels[at+k]) / 255 * (1 - color[3])
				}
				r.body.Set(x, y, color)
			} else {
				if color[3] < .995 {
					continue
				}
				r.depth[index] = z
				r.body.Set(x, y, color)
			}
		}
	}
}

func (r *Renderer) fragment(u, v float64, s *FrameState) Color {
	if r.referenceActive(*s) {
		copy := *s
		copy.Smile = r.referenceSmile(*s)
		s = &copy
	} else if r.assets.Expression.Joint != nil && !s.RestingSmile {
		copy := *s
		copy.Smile = 0
		s = &copy
	}
	w, h := float64(r.assets.Motion.Canvas.Width), float64(r.assets.Motion.Canvas.Height)
	if s.Smile > 0 || s.Brows > 0 || s.Emotion != (rendercontract.EmotionWeights{}) {
		tx, ty := u*w-.5, v*h-.5
		rect := r.assets.Expression.Crop
		// A zero first displacement leaves every inverse iteration at the same
		// coordinate. Avoid all five lookups outside the prepared field.
		if insideField(tx, ty, rect) || r.assets.Expression.EmotionRig != nil && insideField(tx, ty, r.assets.Expression.EmotionRig.Crop) {
			x, y := tx, ty
			iterations := 5
			if rig := r.assets.Expression.EmotionRig; rig != nil && s.Emotion != (rendercontract.EmotionWeights{}) {
				iterations = rig.InverseIterations
			}
			for range iterations {
				dx, dy := r.displacement(x, y, s)
				edx, edy := r.emotionDisplacement(x, y, s.Emotion)
				dx += edx
				dy += edy
				nextX, nextY := tx-dx, ty-dy
				// Exact fixed points remain unchanged in every later iteration.
				// Protected/neutral field regions commonly meet this on step one.
				if nextX == x && nextY == y {
					break
				}
				x, y = nextX, nextY
			}
			u, v = (x+.5)/w, (y+.5)/h
		}
	}
	color := r.subject(u*w-.5, v*h-.5)
	if s.Smile > 0 && r.assets.SmileMaterial != nil {
		rect := r.assets.Expression.Crop
		x, y := (u*w-float64(rect.X))/float64(rect.Width), (v*h-float64(rect.Y))/float64(rect.Height)
		if x >= 0 && x <= 1 && y >= 0 && y <= 1 {
			ref := r.assets.SmileMaterial.Sample(x*float64(rect.Width)-.5, y*float64(rect.Height)-.5)
			for k := range 3 {
				color[k] = color[k]*(1-ref[3]*s.Smile) + ref[k]*color[3]*s.Smile
			}
		}
	}
	return color
}

func (r *Renderer) displacement(x, y float64, s *FrameState) (float64, float64) {
	if r.assets.ExpressionField == nil {
		return 0, 0
	}
	rect := r.assets.Expression.Crop
	u, v := (x-float64(rect.X)+.5)/float64(rect.Width), (y-float64(rect.Y)+.5)/float64(rect.Height)
	if u < 0 || u > 1 || v < 0 || v > 1 {
		return 0, 0
	}
	f := r.assets.ExpressionField
	fx, fy := u*float64(f.Width)-.5, v*float64(f.Height)-.5
	if s.Smile == 0 {
		// Brows use only the blue vertical component. Horizontal displacement
		// is exactly zero, irrespective of the unused red/green channels.
		return 0, (f.SampleChannel(fx, fy, 2)*255 - 128) * s.Brows * r.assets.Expression.Units
	}
	c := f.Sample(fx, fy)
	return (c[0]*255 - 128) * s.Smile * r.assets.Expression.Units, ((c[1]*255-128)*s.Smile + (c[2]*255-128)*s.Brows) * r.assets.Expression.Units
}
