//go:build ispc && cgo && linux && amd64

package nativerender

/*
#cgo CFLAGS: -I/opt/syntelle-simd/include
#cgo LDFLAGS: -L/opt/syntelle-simd/lib -lsyntelle_ispc
#include "compose.h"
*/
import "C"

import (
	"context"
	"image"
	"runtime"
	"unsafe"
)

// Immutable sampling axes are initialized once by the scene's video owner.
// C retains no Go pointers and writes only into the session output frame.
type composeBackend struct {
	ready  bool
	xa, xb [720]int32
	ya, yb [960]int32
	xf     [720]float64
	yf     [960]float64
}

func (r *Renderer) composePixels(ctx context.Context, dst *image.RGBA) error {
	// Packed loads require four-byte alignment and whole-pixel row strides.
	// image.RGBA also permits borrowed subimages and unusual byte strides.
	if dst.Stride%4 != 0 || uintptr(unsafe.Pointer(&dst.Pix[0]))%4 != 0 {
		return r.composeScalar(ctx, dst)
	}
	b := &r.composeNative
	if !b.ready {
		for i, a := range r.composeX {
			b.xa[i], b.xb[i], b.xf[i] = int32(a.A), int32(a.B), a.F
			if !a.Valid {
				b.xa[i] = -1
			}
		}
		for i, a := range r.composeY {
			b.ya[i], b.yb[i], b.yf[i] = int32(a.A), int32(a.B), a.F
			if !a.Valid {
				b.ya[i] = -1
			}
		}
		b.ready = true
	}
	for y := 0; y < 960; y += 32 {
		if err := ctx.Err(); err != nil {
			return err
		}
		C.syntelle_compose(
			(*C.uint32_t)(unsafe.Pointer(&r.body.Pixels[0])),
			(*C.uint32_t)(unsafe.Pointer(&r.background.Pixels[0])),
			(*C.uint32_t)(unsafe.Pointer(&dst.Pix[0])), C.int32_t(dst.Stride/4),
			(*C.int32_t)(unsafe.Pointer(&b.xa[0])), (*C.int32_t)(unsafe.Pointer(&b.xb[0])),
			(*C.double)(unsafe.Pointer(&b.xf[0])),
			(*C.int32_t)(unsafe.Pointer(&b.ya[0])), (*C.int32_t)(unsafe.Pointer(&b.yb[0])),
			(*C.double)(unsafe.Pointer(&b.yf[0])), C.int32_t(y), C.int32_t(y+32))
	}
	runtime.KeepAlive(r)
	runtime.KeepAlive(dst)
	return ctx.Err()
}
