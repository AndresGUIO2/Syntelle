//go:build libyuv && cgo

package nativemedia

/*
#cgo CFLAGS: -I/opt/syntelle-simd/include
#cgo CXXFLAGS: -I/opt/syntelle-simd/include
#cgo LDFLAGS: -L/opt/syntelle-simd/lib -lyuv -lstdc++
#include <stdint.h>
int syntelle_rgba_i420(const uint8_t*, int, uint8_t*, int, uint8_t*, int, uint8_t*, int, int, int);
*/
import "C"

import (
	"errors"
	"image"
	"runtime"
	"unsafe"
)

func limitedI420(dst *image.YCbCr, src *image.RGBA) error {
	if src == nil || dst == nil {
		return errors.New("missing I420 conversion frame")
	}
	if err := validateI420(dst, src); err != nil {
		return err
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	if w < 0 || h < 0 {
		return errors.New("invalid I420 conversion dimensions")
	}
	if w == 0 || h == 0 {
		return nil
	}
	// Validate borrowed slices before crossing into C, where Go bounds checks
	// cannot protect truncated planes or a stride narrowed to a native int.
	const maxNativeInt = 1<<31 - 1
	fits := func(size, stride, width, height int) bool {
		return width > 0 && height > 0 && stride >= width && stride <= maxNativeInt &&
			size >= width && height-1 <= (size-width)/stride
	}
	if w > maxNativeInt/4 || h > maxNativeInt ||
		!fits(len(src.Pix), src.Stride, w*4, h) ||
		!fits(len(dst.Y), dst.YStride, w, h) ||
		!fits(len(dst.Cb), dst.CStride, w/2, h/2) ||
		!fits(len(dst.Cr), dst.CStride, w/2, h/2) {
		return errors.New("invalid I420 conversion buffers")
	}
	// Go RGBA bytes correspond to libyuv's little-endian ABGR layout.
	status := C.syntelle_rgba_i420((*C.uint8_t)(unsafe.Pointer(&src.Pix[0])), C.int(src.Stride),
		(*C.uint8_t)(unsafe.Pointer(&dst.Y[0])), C.int(dst.YStride),
		(*C.uint8_t)(unsafe.Pointer(&dst.Cb[0])), C.int(dst.CStride),
		(*C.uint8_t)(unsafe.Pointer(&dst.Cr[0])), C.int(dst.CStride),
		C.int(src.Rect.Dx()), C.int(src.Rect.Dy()))
	runtime.KeepAlive(src)
	runtime.KeepAlive(dst)
	if status != 0 {
		return errors.New("libyuv RGBA to I420 conversion failed")
	}
	return nil
}
