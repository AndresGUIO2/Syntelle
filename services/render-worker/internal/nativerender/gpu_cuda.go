//go:build nativegpu && cgo && linux && amd64

package nativerender

/*
#cgo CFLAGS: -I/opt/syntelle-nvenc/include
#cgo LDFLAGS: -ldl -lpthread
#include <stdlib.h>
#include "gpu_cuda.h"
*/
import "C"

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"syntelle/render-worker/internal/nativeframe"
	"time"
	"unsafe"
)

//go:embed gpu_kernel.ptx
var gpuPTX string

type gpuVertex struct{ X, Y, Z, U, V, Q float64 }
type gpuTriangle struct {
	A, B, C gpuVertex
	Inv     float64
	Flags   [6]int32
}
type gpuAxis struct {
	A, B       int32
	F          float64
	Valid, Pad int32
}
type gpuParams struct {
	Dims                [16]int32
	Smile, Brows, Units float64
	EmotionDims         [8]int32
	EmotionUnits        float64
	EmotionWeights      [10]float64
	MaskMode            int32
	Precision           int32
	ViewDims            [4]int32
	ViewWeights         [4]float64
	LaughterDims        [8]int32
	LaughterValues      [6]float64
}

type cudaRenderer struct {
	native                                                              *C.GPUContext
	ready                                                               bool
	triangles                                                           []gpuTriangle
	bins                                                                [][]int32
	indices, offsets                                                    []int32
	tilesX                                                              int
	poseTextures                                                        map[string][3]int32
	laughLipIndex                                                       int32
	referenceCavityIndex                                                int32
	oralTriangles                                                       []gpuOralTriangle
	lastState                                                           FrameState
	hasState                                                            bool
	hostPipeline                                                        bool
	shareImmutable                                                      bool
	maskMode                                                            int32
	precision                                                           int32
	measure                                                             bool
	geometryNS, patchNS, uploadNS, bodyNS, composeNS, convertNS, syncNS atomic.Uint64
}

// Explicit measurement reference: preserves phase-66 CPU patches/readback.
// Its wrapper deliberately omits the device-surface interface.
type hostCUDA struct{ gpu *cudaRenderer }

func (h *hostCUDA) Close() error                  { return h.gpu.Close() }
func (h *hostCUDA) RenderMetrics() map[string]any { return h.gpu.RenderMetrics() }
func (h *hostCUDA) Render(ctx context.Context, r *Renderer, s FrameState, dst *image.RGBA, mouth, eyes, background bool) error {
	return h.gpu.renderAll(ctx, r, s, dst, nil, mouth, eyes)
}

type gpuOralTriangle struct {
	P, From, To [3][2]float64
	Area        float64
	Flags       [4]int32
}
type gpuOralParams struct {
	Dims   [16]int32
	Values [60]float64
}
type gpuEyeParams struct {
	Dims   [12]int32
	Values [8]float64
}
type gpuSkinParams struct {
	Dims   [12]int32
	Values [4]float64
}
type gpuReferenceParams struct {
	Dims   [16]int32
	Values [149]float64
}

type gpuAtlasIdentity struct {
	Digest [32]byte
	Bytes  int
}
type gpuAtlasSource struct {
	mu       sync.Mutex
	identity func() gpuAtlasIdentity
}

func openGPU() (gpuRenderer, error) {
	if unsafe.Sizeof(gpuTriangle{}) != C.sizeof_GPUTriangle || unsafe.Sizeof(gpuAxis{}) != C.sizeof_GPUAxis || unsafe.Sizeof(gpuParams{}) != C.sizeof_GPUParams {
		return nil, errors.New("cuda renderer ABI mismatch")
	}
	ptx := C.CString(gpuPTX)
	defer C.free(unsafe.Pointer(ptx))
	var status C.int
	native := C.syntelle_gpu_open(ptx, &status)
	if native == nil {
		return nil, fmt.Errorf("cuda renderer initialization (native code %d)", int(status))
	}
	g := &cudaRenderer{native: native, hostPipeline: os.Getenv("SYNTELLE_GPU_PIPELINE") == "host", shareImmutable: os.Getenv("SYNTELLE_GPU_SHARING") != "off", measure: os.Getenv("SYNTELLE_OFFLINE_PROBE") == "1"}
	if os.Getenv("SYNTELLE_GPU_PRECISION") == "fp32" {
		g.precision = 1
	}
	if os.Getenv("SYNTELLE_OFFLINE_PROBE") == "1" {
		switch os.Getenv("SYNTELLE_OFFLINE_MASK_MODE") {
		case "frozen":
			g.maskMode = 1
		case "color-only":
			g.maskMode = 2
		}
		if os.Getenv("SYNTELLE_OFFLINE_PRECISION") == "fp32" {
			g.precision = 1
		}
	}
	if g.hostPipeline {
		return &hostCUDA{gpu: g}, nil
	}
	return g, nil
}

func (g *cudaRenderer) upload(slot int, data unsafe.Pointer, bytes int) error {
	if bytes == 0 {
		return nil
	}
	if status := C.syntelle_gpu_upload(g.native, C.int(slot), data, C.size_t(bytes)); status != 0 {
		return fmt.Errorf("upload cuda renderer buffer (native code %d)", int(status))
	}
	return nil
}

// Hash exact prepared bytes, never character names or mutable package paths.
// The native lease is scoped to the CUDA context and released after stream drain.
func (g *cudaRenderer) immutable(slot int, data unsafe.Pointer, bytes int) error {
	if bytes == 0 {
		return nil
	}
	if !g.shareImmutable {
		return g.upload(slot, data, bytes)
	}
	digest := sha256.Sum256(unsafe.Slice((*byte)(data), bytes))
	if status := C.syntelle_gpu_share(g.native, C.int(slot), data, C.size_t(bytes), (*C.uchar)(unsafe.Pointer(&digest[0]))); status != 0 {
		return fmt.Errorf("share immutable cuda buffer %d (native code %d)", slot, int(status))
	}
	return nil
}

func (g *cudaRenderer) texture(slot int, texture *Texture) error {
	if texture == nil {
		return nil
	}
	if texture.Width < 1 || texture.Height < 1 || len(texture.Pixels) != texture.Width*texture.Height*4 {
		return errors.New("invalid cuda source texture")
	}
	if slot == 0 || slot == 3 {
		return g.upload(slot, unsafe.Pointer(&texture.Pixels[0]), len(texture.Pixels))
	}
	return g.immutable(slot, unsafe.Pointer(&texture.Pixels[0]), len(texture.Pixels))
}

func (g *cudaRenderer) initialize(r *Renderer) error {
	if r.body.Width < 1 || r.body.Height < 1 || r.body.Width > 1024 || r.body.Height > 1024 {
		return errors.New("invalid cuda body dimensions")
	}
	for _, item := range []struct {
		slot    int
		texture *Texture
	}{{0, r.subjectTexture}, {1, r.assets.ExpressionField}, {2, r.assets.SmileMaterial}, {23, r.assets.EmotionField}} {
		if err := g.texture(item.slot, item.texture); err != nil {
			return err
		}
	}
	if len(r.assets.viewTextures) > 0 {
		var atlas []byte
		for _, t := range r.assets.viewTextures {
			atlas = append(atlas, t.Pixels...)
		}
		if err := g.immutable(24, unsafe.Pointer(&atlas[0]), len(atlas)); err != nil {
			return err
		}
	}
	for _, item := range []struct {
		slot int
		axes []sampleAxis
	}{{4, r.composeX[:r.displayWidth]}, {5, r.composeY[:r.displayHeight]}} {
		axes := make([]gpuAxis, len(item.axes))
		for i, a := range item.axes {
			axes[i] = gpuAxis{A: int32(a.A), B: int32(a.B), F: a.F}
			if a.Valid {
				axes[i].Valid = 1
			}
		}
		if err := g.immutable(item.slot, unsafe.Pointer(&axes[0]), len(axes)*int(unsafe.Sizeof(gpuAxis{}))); err != nil {
			return err
		}
	}
	g.tilesX = (r.body.Width + 15) / 16
	g.bins = make([][]int32, g.tilesX*((r.body.Height+15)/16))
	g.offsets = make([]int32, len(g.bins)+1)
	if !g.hostPipeline {
		if err := g.initializePatches(r); err != nil {
			return err
		}
	}
	if r.assets.ReferenceField != nil {
		if err := g.texture(25, r.assets.ReferenceField); err != nil {
			return err
		}
	}
	g.ready = true
	return nil
}

func (g *cudaRenderer) addTriangle(r *Renderer, ia, ib, ic uint16, boundary, reference bool, s FrameState) {
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
	vertex := func(v rasterVertex) gpuVertex { return gpuVertex{v.X, v.Y, v.Z, v.U, v.V, v.Q} }
	t := gpuTriangle{A: vertex(a), B: vertex(b), C: vertex(c), Inv: 1 / area}
	flags := [5]bool{topLeft(b, c), topLeft(c, a), topLeft(a, b),
		((reference || r.speechJawTriangleReference(s, ia, ib, ic)) && (s.Breath == 0 || a.Torso == 0 && b.Torso == 0 && c.Torso == 0)) || (a.Head == 0 && b.Head == 0 && c.Head == 0 && s.Breath == 0), boundary}
	for i, value := range flags {
		if value {
			t.Flags[i] = 1
		}
	}
	id := int32(len(g.triangles))
	g.triangles = append(g.triangles, t)
	for y := y0 / 16; y <= y1/16; y++ {
		for x := x0 / 16; x <= x1/16; x++ {
			index := y*g.tilesX + x
			g.bins[index] = append(g.bins[index], id)
		}
	}
}

func (g *cudaRenderer) Render(ctx context.Context, r *Renderer, s FrameState, dst *image.RGBA, mouth, eyes, background bool) error {
	return g.RenderAll(ctx, r, s, dst, nil)
}

func (g *cudaRenderer) RenderAll(ctx context.Context, r *Renderer, s FrameState, dst *image.RGBA, target *nativeframe.Surface) error {
	return g.renderAll(ctx, r, s, dst, target, false, false)
}

func (g *cudaRenderer) renderAll(ctx context.Context, r *Renderer, s FrameState, dst *image.RGBA, target *nativeframe.Surface, mouth, eyes bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if g.native == nil {
		return errors.New("cuda renderer is closed")
	}
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	validWidth := w == 2*r.displayWidth || (g.maskMode == 2 && w == r.displayWidth)
	if !validWidth || h != r.displayHeight || dst.Stride < w*4 || len(dst.Pix) < (h-1)*dst.Stride+w*4 {
		return errors.New("invalid cuda output buffer")
	}
	first := !g.ready
	stage := time.Now()
	if first {
		if err := g.initialize(r); err != nil {
			return err
		}
	}
	if !g.hostPipeline {
		if err := g.updatePatches(r, s); err != nil {
			return err
		}
	} else if !first {
		for _, u := range []struct {
			changed bool
			p       patch
		}{{r.skinChanged, r.expressionPatch}, {mouth, r.mouthPatch}, {eyes, r.eyePatch}, {r.referenceChanged, r.referencePatch}} {
			if !u.changed {
				continue
			}
			p := u.p
			if status := C.syntelle_gpu_patch(g.native, C.int(p.Rect.X), C.int(p.Rect.Y), C.int(p.Rect.Width), C.int(p.Rect.Height), C.int(r.subjectTexture.Width), (*C.uchar)(unsafe.Pointer(&p.Pixels.Pixels[0]))); status != 0 {
				return fmt.Errorf("update reference cuda patch (native code %d)", int(status))
			}
		}
	}
	if g.measure {
		g.patchNS.Add(uint64(time.Since(stage)))
	}
	background := w == 720 && (!g.hasState || g.lastState.Background != s.Background || g.lastState.Blur != s.Blur)
	if background {
		if r.background == nil || r.backgroundID != s.Background || r.backgroundBlur != s.Blur {
			var err error
			r.background, err = r.makeBackground(s.Background, s.Blur)
			if err != nil {
				return err
			}
			r.backgroundID = s.Background
			r.backgroundBlur = s.Blur
		}
		if err := g.texture(3, r.background); err != nil {
			return err
		}
	}
	stage = time.Now()
	reference := r.prepareBody(s)
	g.triangles = g.triangles[:0]
	g.indices = g.indices[:0]
	for i := range g.bins {
		g.bins[i] = g.bins[i][:0]
	}
	for i := 0; i < len(r.assets.indices); i += 3 {
		if i%300 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		g.addTriangle(r, r.assets.indices[i], r.assets.indices[i+1], r.assets.indices[i+2], false, reference, s)
	}
	for _, t := range r.fringe {
		g.addTriangle(r, t.IDs[0], t.IDs[1], t.IDs[2], true, reference, s)
	}
	for i, bin := range g.bins {
		g.offsets[i] = int32(len(g.indices))
		g.indices = append(g.indices, bin...)
	}
	g.offsets[len(g.bins)] = int32(len(g.indices))
	if len(g.triangles) == 0 || len(g.indices) == 0 {
		return errors.New("empty cuda geometry")
	}
	if g.measure {
		g.geometryNS.Add(uint64(time.Since(stage)))
	}
	stage = time.Now()
	for _, buffer := range []struct {
		slot    int
		pointer unsafe.Pointer
		bytes   int
	}{
		{6, unsafe.Pointer(&g.triangles[0]), len(g.triangles) * int(unsafe.Sizeof(gpuTriangle{}))},
		{7, unsafe.Pointer(&g.offsets[0]), len(g.offsets) * 4}, {8, unsafe.Pointer(&g.indices[0]), len(g.indices) * 4},
	} {
		if err := g.upload(buffer.slot, buffer.pointer, buffer.bytes); err != nil {
			return err
		}
	}
	if g.measure {
		g.uploadNS.Add(uint64(time.Since(stage)))
	}
	p := gpuParams{Dims: [16]int32{int32(r.body.Width), int32(r.body.Height), int32(r.subjectTexture.Width), int32(r.subjectTexture.Height), int32(w), int32(h), int32(g.tilesX), int32(len(g.triangles)),
		int32(r.assets.Expression.Crop.X), int32(r.assets.Expression.Crop.Y), int32(r.assets.Expression.Crop.Width), int32(r.assets.Expression.Crop.Height)}, Smile: s.Smile, Brows: s.Brows, Units: r.assets.Expression.Units}
	if companion := r.assets.Expression.ReferenceLaughter; companion != nil && companion.Presentation != nil {
		c := companion.Presentation.Crop
		o := r.mouthPatch.Rect
		p.LaughterDims = [8]int32{int32(c.X), int32(c.Y), int32(c.Width), int32(c.Height), int32(o.X), int32(o.Y), int32(o.Width), int32(o.Height)}
		p.LaughterValues = [6]float64{s.Laugh, companion.Presentation.Units, 0, float64(companion.Presentation.Iterations), companion.Presentation.Feather}
		if !s.RestingSmile {
			p.LaughterValues[5] = 1
		}
		if s.LaughSpeech {
			p.LaughterValues[2] = 1
		}
	}
	p.MaskMode = g.maskMode
	p.Precision = g.precision
	if views := r.assets.Motion.Projection.MultiView; views != nil {
		p.ViewDims = [4]int32{int32(len(views.Views)), int32(r.subjectTexture.Width), int32(r.subjectTexture.Height), 0}
		for i, view := range views.Views {
			p.ViewWeights[i] = viewWeight(view, s.Head)
		}
	}
	if rig := r.assets.Expression.EmotionRig; rig != nil {
		// The eighth dimension is a per-frame activity flag used by the body
		// kernel, avoiding a scan of all nine weights for every body pixel.
		p.EmotionDims = [8]int32{int32(rig.Crop.X), int32(rig.Crop.Y), int32(rig.Crop.Width), int32(rig.Crop.Height), int32(rig.FieldSize[0]), int32(rig.FieldSize[1]), int32(rig.InverseIterations), 0}
		p.EmotionUnits = rig.Units
		p.EmotionWeights = s.Emotion
		for i := 1; i < len(s.Emotion); i++ {
			if s.Emotion[i] != 0 {
				p.EmotionDims[7] = 1
				break
			}
		}
	}
	if r.assets.Expression.Joint != nil && !s.RestingSmile {
		p.Smile = 0
	}
	if r.referenceActive(s) {
		p.Smile = r.referenceSmile(s)
	}
	if f := r.assets.ExpressionField; f != nil {
		p.Dims[12], p.Dims[13] = int32(f.Width), int32(f.Height)
	}
	if f := r.assets.SmileMaterial; f != nil {
		p.Dims[14], p.Dims[15] = int32(f.Width), int32(f.Height)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var output *C.uchar
	var device *C.GPUOutput
	var descriptor C.GPUOutput
	if target == nil {
		output = (*C.uchar)(unsafe.Pointer(&dst.Pix[0]))
	} else {
		descriptor.context = C.ulonglong(target.Context)
		descriptor.width = C.int(target.Width)
		descriptor.height = C.int(target.Height)
		for i := range 3 {
			descriptor.planes[i] = C.ulonglong(target.Planes[i])
			descriptor.strides[i] = C.int(target.Strides[i])
		}
		if target.Marker {
			descriptor.marker = 1
		}
		device = &descriptor
	}
	var timing C.GPUFrameTiming
	var timingTarget *C.GPUFrameTiming
	if g.measure {
		timingTarget = &timing
	}
	if status := C.syntelle_gpu_run(g.native, (*C.GPUParams)(unsafe.Pointer(&p)), output, C.int(dst.Stride), device, timingTarget); status != 0 {
		return fmt.Errorf("render cuda frame (native code %d)", int(status))
	}
	if g.measure {
		g.bodyNS.Add(uint64(float64(timing.body_ms) * 1e6))
		g.composeNS.Add(uint64(float64(timing.compose_ms) * 1e6))
		g.convertNS.Add(uint64(float64(timing.convert_ms) * 1e6))
		g.syncNS.Add(uint64(float64(timing.sync_ms) * 1e6))
	}
	g.lastState = s
	g.hasState = true
	return ctx.Err()
}

func (g *cudaRenderer) RenderMetrics() map[string]any {
	if !g.measure {
		return nil
	}
	ms := func(v *atomic.Uint64) float64 { return float64(v.Load()) / 1e6 }
	return map[string]any{"cpu_patch_ms": ms(&g.patchNS), "cpu_geometry_ms": ms(&g.geometryNS), "geometry_upload_ms": ms(&g.uploadNS), "cuda_body_ms": ms(&g.bodyNS), "cuda_compose_ms": ms(&g.composeNS), "cuda_convert_ms": ms(&g.convertNS), "cuda_sync_wall_ms": ms(&g.syncNS), "mask_mode": g.maskMode, "precision": g.precision}
}

func (g *cudaRenderer) Readback(dst *image.RGBA) error {
	if status := C.syntelle_gpu_readback(g.native, (*C.uchar)(unsafe.Pointer(&dst.Pix[0])), C.int(dst.Rect.Dx()), C.int(dst.Rect.Dy()), C.int(dst.Stride)); status != 0 {
		return fmt.Errorf("inspect cuda frame (native code %d)", int(status))
	}
	return nil
}

func (g *cudaRenderer) initializePatches(r *Renderer) error {
	if unsafe.Sizeof(gpuOralTriangle{}) != C.sizeof_GPUOralTriangle || unsafe.Sizeof(gpuOralParams{}) != C.sizeof_GPUOralParams || unsafe.Sizeof(gpuEyeParams{}) != C.sizeof_GPUEyeParams || unsafe.Sizeof(gpuSkinParams{}) != C.sizeof_GPUSkinParams || unsafe.Sizeof(gpuReferenceParams{}) != C.sizeof_GPUReferenceParams {
		return errors.New("cuda patch ABI mismatch")
	}
	if err := g.texture(14, r.assets.Foreground); err != nil {
		return err
	}
	uploadAtlas := func(slot int, textures []*Texture) error {
		var identity gpuAtlasIdentity
		if g.shareImmutable {
			value, _ := r.assets.gpuAtlasKeys.LoadOrStore(slot, &gpuAtlasSource{identity: sync.OnceValue(func() gpuAtlasIdentity {
				h := sha256.New()
				var key gpuAtlasIdentity
				for _, texture := range textures {
					h.Write(texture.Pixels)
					key.Bytes += len(texture.Pixels)
				}
				copy(key.Digest[:], h.Sum(nil))
				return key
			})})
			source := value.(*gpuAtlasSource)
			// Only cold atlas acquisition is serialized. Frame rendering never
			// takes this lock; concurrent first sessions build one host atlas.
			source.mu.Lock()
			defer source.mu.Unlock()
			identity = source.identity()
			if identity.Bytes == 0 {
				return nil
			}
			status := C.syntelle_gpu_share(g.native, C.int(slot), nil, C.size_t(identity.Bytes), (*C.uchar)(unsafe.Pointer(&identity.Digest[0])))
			if status == 0 {
				return nil
			}
			if status != -1003 {
				return fmt.Errorf("acquire immutable cuda atlas (native code %d)", int(status))
			}
		}
		// Preserve the preceding allocator only for the private reference.
		var data []byte
		if g.shareImmutable {
			data = make([]byte, 0, identity.Bytes)
		}
		for _, t := range textures {
			data = append(data, t.Pixels...)
		}
		if len(data) == 0 {
			return nil
		}
		if !g.shareImmutable {
			return g.upload(slot, unsafe.Pointer(&data[0]), len(data))
		}
		if status := C.syntelle_gpu_share(g.native, C.int(slot), unsafe.Pointer(&data[0]), C.size_t(len(data)), (*C.uchar)(unsafe.Pointer(&identity.Digest[0]))); status != 0 {
			return fmt.Errorf("share immutable cuda atlas (native code %d)", int(status))
		}
		return nil
	}
	textures := []*Texture{r.assets.Oral.upper, r.assets.Oral.lower}
	keys := make([]string, 0, len(r.assets.Oral.poses))
	for id := range r.assets.Oral.poses {
		keys = append(keys, id)
	}
	slices.Sort(keys)
	g.poseTextures = make(map[string][3]int32, len(keys))
	for _, id := range keys {
		p := r.assets.Oral.poses[id]
		n := int32(len(textures))
		g.poseTextures[id] = [3]int32{n, n + 1, n + 2}
		textures = append(textures, p.surface, p.mask, p.cavity)
	}
	if r.assets.LaughLips != nil {
		g.laughLipIndex = int32(len(textures))
		textures = append(textures, r.assets.LaughLips)
	}
	if r.assets.ReferenceCavity != nil {
		g.referenceCavityIndex = int32(len(textures))
		textures = append(textures, r.assets.ReferenceCavity, r.assets.ReferenceUpper, r.assets.ReferenceLower)
	}
	if err := uploadAtlas(11, textures); err != nil {
		return err
	}
	if len(r.assets.SkinFrames) == 25 {
		if err := uploadAtlas(16, r.assets.SkinFrames); err != nil {
			return err
		}
	}
	if len(r.assets.LaughFrames) == 25 {
		if err := uploadAtlas(17, r.assets.LaughFrames); err != nil {
			return err
		}
	}
	if len(r.assets.ReferenceFrames) == 25 {
		if err := uploadAtlas(21, r.assets.ReferenceFrames); err != nil {
			return err
		}
		p := r.assets.Expression.ReferenceLaughter
		curves := make([][4]float64, 0, 25*p.Eyes.Crop.Width)
		for _, profile := range p.Eyes.Profiles {
			for _, c := range profile.Curves {
				curves = append(curves, [4]float64{c[0], c[1], c[2], c[3]})
			}
		}
		if err := g.immutable(22, unsafe.Pointer(&curves[0]), len(curves)*32); err != nil {
			return err
		}
	}
	e := r.eyes
	if err := uploadAtlas(18, []*Texture{e.opened, e.closed, e.mask, e.sclera, e.ownership}); err != nil {
		return err
	}
	if err := g.immutable(19, unsafe.Pointer(&e.iris[0]), len(e.iris)*int(unsafe.Sizeof(Color{}))); err != nil {
		return err
	}
	curves := make([][4]float64, len(e.curves))
	for i, c := range e.curves {
		curves[i] = [4]float64{c.upper, c.lower, c.seam, 0}
		if c.valid {
			curves[i][3] = 1
		}
	}
	return g.immutable(20, unsafe.Pointer(&curves[0]), len(curves)*32)
}

// Private offline qualification accounting; excludes driver/module/NVENC memory.
func gpuMemory() (bytes, allocations, leases uint64) {
	var b, a, l C.size_t
	C.syntelle_gpu_memory(&b, &a, &l)
	return uint64(b), uint64(a), uint64(l)
}
func (g *cudaRenderer) privateBytes() uint64 { return uint64(C.syntelle_gpu_private_bytes(g.native)) }

func (g *cudaRenderer) update(kind int, p unsafe.Pointer, width int) error {
	if status := C.syntelle_gpu_update(g.native, C.int(kind), p, C.int(width)); status != 0 {
		return fmt.Errorf("update cuda patch (native code %d)", int(status))
	}
	return nil
}

func (g *cudaRenderer) updatePatches(r *Renderer, s FrameState) error {
	smile := math.Round(clamp(r.jointSmile(s), 0, 1)*120) / 120
	laugh := math.Round(clamp(s.Laugh, 0, 1)*120) / 120
	legacyLaugh := laugh
	if r.referenceActive(s) {
		legacyLaugh = 0
	}
	skin := len(r.assets.SkinFrames) == 25 && (!g.hasState || math.Round(clamp(r.jointSmile(g.lastState), 0, 1)*120)/120 != smile || math.Round(clamp(g.lastState.Laugh, 0, 1)*120)/120 != laugh)
	sw := r.subjectTexture.Width
	resetReference := !r.referenceActive(s) && g.hasState && r.referenceActive(g.lastState)
	refreshReference := r.referenceActive(s) && r.assets.Expression.ReferenceLaughter.Presentation != nil
	skin = skin || refreshReference
	if resetReference || refreshReference {
		p := r.referenceParams(s, g.referenceCavityIndex)
		p.Dims[15] = 1
		if err := g.update(4, unsafe.Pointer(&p), sw); err != nil {
			return err
		}
	}
	if skin {
		rect := r.expressionPatch.Rect
		index := smile * 24
		lo := int(index)
		p := gpuSkinParams{Dims: [12]int32{int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height), int32(lo), int32(min(24, lo+1))}, Values: [4]float64{index - float64(lo), smile, legacyLaugh}}
		if len(r.assets.LaughFrames) == 25 {
			p.Dims[6] = 1
		}
		if err := g.update(1, unsafe.Pointer(&p), sw); err != nil {
			return err
		}
	}
	r.configureArticulation(s)
	if resetReference || skin || !g.hasState || s.JawOpen != g.lastState.JawOpen || s.LipOpening != g.lastState.LipOpening || s.LipTension != g.lastState.LipTension || s.From != g.lastState.From || s.To != g.lastState.To || s.Amount != g.lastState.Amount || s.Emotion != g.lastState.Emotion {
		g.oralTriangles = g.oralTriangles[:0]
		var params gpuOralParams
		r.oral.laugh = legacyLaugh
		r.oral.emotion = s.Emotion
		_, err := r.oral.drawExpression(s.From, s.To, s.Amount, r.oralSmile(s), func(mesh *oralMesh, ids [3]int, points [3][2]float64, state *oralShadeState) {
			area := patchEdge(points[0], points[1], points[2])
			if math.Abs(area) < 1e-10 {
				return
			}
			if area < 0 {
				points[1], points[2], ids[1], ids[2] = points[2], points[1], ids[2], ids[1]
				area = -area
			}
			t := gpuOralTriangle{P: points, Area: area}
			for i, id := range ids {
				t.From[i] = mesh.Rest[id]
				t.To[i] = mesh.Open[id]
			}
			for i, pair := range [][2]int{{1, 2}, {2, 0}, {0, 1}} {
				a, b := points[pair[0]], points[pair[1]]
				if b[1] < a[1] || b[1] == a[1] && b[0] > a[0] {
					t.Flags[i] = 1
				}
			}
			g.oralTriangles = append(g.oralTriangles, t)
			if len(g.oralTriangles) > 1 {
				return
			}
			f, to := g.poseTextures[state.from.definition.ID], g.poseTextures[state.to.definition.ID]
			rect := r.mouthPatch.Rect
			params.Dims = [16]int32{int32(mesh.Width), int32(mesh.Height), int32(rect.X), int32(rect.Y), f[0], f[1], f[2], to[0], to[1], to[2], 0, 1}
			params.Dims[15] = g.laughLipIndex
			params.Values[14] = legacyLaugh
			if state.joint {
				params.Dims[12] = 1
			}
			if state.deformedCoverage {
				params.Dims[12] |= 2
			}
			if r.oral.withoutTeeth {
				params.Dims[12] |= 4
			}
			if state.semantic {
				params.Dims[13] = 1
			}
			l := r.assets.Oral.lighting
			params.Values = [60]float64{state.amount, state.cavityAmount, state.upper[0], state.upper[1], state.lower[0], state.lower[1], state.minimum[0], state.minimum[1], state.maximum[0], state.maximum[1], l.ShadowStrength, l.ShadowWidth, l.DepthStrength, l.ClosedGap}
			if v := l.Visible; v != nil {
				params.Values[56], params.Values[57], params.Values[58], params.Values[59] = v.ReferenceGap, v.Minimum, v.FullRatio, v.ContactWidth
			}
			params.Values[14] = legacyLaugh
			for i, v := range state.inner {
				params.Values[16+i*2] = v[0]
				params.Values[17+i*2] = v[1]
			}
		})
		if err != nil {
			return err
		}
		if len(g.oralTriangles) == 0 {
			return errors.New("empty cuda mouth geometry")
		}
		params.Dims[14] = int32(len(g.oralTriangles))
		if err := g.upload(12, unsafe.Pointer(&g.oralTriangles[0]), len(g.oralTriangles)*int(unsafe.Sizeof(gpuOralTriangle{}))); err != nil {
			return err
		}
		if err := g.update(2, unsafe.Pointer(&params), sw); err != nil {
			return err
		}
	}
	if resetReference || skin || !g.hasState || s.Closure != g.lastState.Closure || s.Gaze != g.lastState.Gaze || s.Laugh != g.lastState.Laugh || s.Emotion != g.lastState.Emotion {
		e := r.eyes
		rect := r.eyePatch.Rect
		p := gpuEyeParams{Dims: [12]int32{int32(e.opened.Width), int32(e.opened.Height), int32(rect.X), int32(rect.Y)}, Values: [8]float64{float64(float32(clamp(s.Closure, 0, 1))), e.eyeSplit, e.lashBand}}
		p.Values[7] = r.laughSquint(s)
		for eye := range 2 {
			for axis := range 2 {
				v := s.Gaze[eye][axis]
				if !patchFinite(v) {
					v = 0
				}
				p.Values[3+eye*2+axis] = float64(float32(v))
			}
		}
		if err := g.update(3, unsafe.Pointer(&p), sw); err != nil {
			return err
		}
	}
	if r.referenceActive(s) {
		f := r.referenceState(s)
		if f.mappingInvalid {
			return errors.New("laughter material correspondence has no valid shared triangulation")
		}
		if len(f.triangles) > 0 {
			if err := g.upload(26, unsafe.Pointer(&f.triangles[0]), len(f.triangles)*int(unsafe.Sizeof(laughterOralTriangle{}))); err != nil {
				return err
			}
		}
		p := r.referenceParams(s, g.referenceCavityIndex)
		if err := g.update(4, unsafe.Pointer(&p), sw); err != nil {
			return err
		}
	}
	return nil
}

func (r *Renderer) referenceParams(s FrameState, cavity int32) gpuReferenceParams {
	p := r.assets.Expression.ReferenceLaughter
	f := r.referenceState(s)
	rect, eye, oral := p.Crop, p.Eyes.Crop, r.mouthPatch.Rect
	v := gpuReferenceParams{Dims: [16]int32{int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height), int32(f.low), int32(f.high), int32(eye.X), int32(eye.Y), int32(eye.Width), int32(eye.Height), int32(oral.X), int32(oral.Y), int32(oral.Width), int32(oral.Height), cavity}, Values: [149]float64{f.weight, s.Laugh, f.jaw, r.referenceChin(), clamp(s.Closure, 0, 1), r.eyes.eyeSplit, r.eyes.lashBand, f.upper[0], f.upper[1], f.lower[0], f.lower[1], s.Gaze[0][0], s.Gaze[0][1], s.Gaze[1][0], s.Gaze[1][1], f.offsets[0][0], f.offsets[0][1], f.offsets[1][0], f.offsets[1][1]}}
	if p.Presentation != nil {
		v.Values[19] = laughterOralWeight(s)
		v.Values[148] = float64(len(f.triangles))
		v.Values[144] = 1
		v.Values[146] = p.Presentation.Feather
		baseline := s
		baseline.Laugh = 0
		v.Values[147] = r.laughSquint(baseline)
		if s.LaughSpeech {
			v.Values[145] = 1
		}
	}
	if f.localContact {
		v.Values[103] = 1
		for i := range 20 {
			v.Values[104+i*2] = p.Oral.Inner[24][i][0]
			v.Values[105+i*2] = p.Oral.Inner[24][i][1]
		}
	}
	if shadow := p.Oral.ContactShadow; shadow != nil {
		v.Values[100] = shadow.Strength
		v.Values[101] = shadow.Width
		v.Values[102] = p.Oral.Inner[24][15][1] - p.Oral.Inner[24][5][1]
	}
	for i := range 20 {
		for axis := range 2 {
			v.Values[20+i*2+axis] = f.inner[i][axis]
			v.Values[60+i*2+axis] = f.outer[i][axis]
		}
	}
	return v
}

func (g *cudaRenderer) Close() error {
	if g.native == nil {
		return nil
	}
	status := C.syntelle_gpu_close(g.native)
	g.native = nil
	if status != 0 {
		return fmt.Errorf("close cuda renderer (native code %d)", int(status))
	}
	return nil
}
