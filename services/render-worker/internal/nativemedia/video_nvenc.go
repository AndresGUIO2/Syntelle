//go:build nvenc && nativecodecs && cgo && linux && amd64

package nativemedia

/*
#cgo CFLAGS: -I/opt/syntelle-nvenc/include
#cgo LDFLAGS: -L/opt/syntelle-nvenc/lib -lavcodec -lavutil -Wl,-rpath,/opt/syntelle-nvenc/lib
#include "video_nvenc.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"os"
	"syntelle/render-worker/internal/nativeframe"
	"unsafe"
)

type nvencVideo struct {
	native        *C.syntelle_nvenc
	width, height int
	key           bool
	device        bool
}

func newNVENCVideo(width, height, fps, bitrate int) (VideoEncoder, error) {
	if width < 2 || height < 2 || width > 4096 || height > 4096 || width%2 != 0 || height%2 != 0 || fps < 1 || fps > 60 || bitrate < 1 || bitrate > 100000000 {
		return nil, errors.New("invalid nvenc video dimensions or rate")
	}
	var status C.int
	device := os.Getenv("SYNTELLE_GPU_PIPELINE") != "host"
	useDevice := C.int(0)
	if device {
		useDevice = 1
	}
	native := C.syntelle_nvenc_open(C.int(width), C.int(height), C.int(fps), C.int(bitrate), useDevice, &status)
	if native == nil {
		return nil, fmt.Errorf("nvenc initialization failed (native code %d)", int(status))
	}
	return &nvencVideo{native: native, width: width, height: height, key: true, device: device}, nil
}

func (v *nvencVideo) Encode(frame *image.RGBA) ([]byte, error) {
	if v.native == nil {
		return nil, errors.New("nvenc encoder is closed")
	}
	if frame == nil || frame.Rect != image.Rect(0, 0, v.width, v.height) {
		return nil, errors.New("unexpected nvenc input dimensions")
	}
	var y, u, cr *C.uint8_t
	var ys, cs C.int
	if status := C.syntelle_nvenc_frame(v.native, &y, &u, &cr, &ys, &cs); status < 0 {
		return nil, fmt.Errorf("prepare nvenc frame (native code %d)", int(status))
	}
	// Restricted native-codec boundary: these slices borrow C-owned AVFrame
	// planes only until Encode returns. Conversion writes directly into reusable
	// encoder memory; C never retains a Go pointer, including across GPU upload.
	if y == nil || u == nil || cr == nil || int(ys) < v.width || int(cs) < v.width/2 || ys > 65536 || cs > 65536 {
		return nil, errors.New("invalid native nvenc frame planes")
	}
	planes := &image.YCbCr{Rect: frame.Rect, SubsampleRatio: image.YCbCrSubsampleRatio420,
		YStride: int(ys), CStride: int(cs),
		Y:  unsafe.Slice((*byte)(unsafe.Pointer(y)), int(ys)*v.height),
		Cb: unsafe.Slice((*byte)(unsafe.Pointer(u)), int(cs)*(v.height/2)),
		Cr: unsafe.Slice((*byte)(unsafe.Pointer(cr)), int(cs)*(v.height/2))}
	if err := limitedI420(planes, frame); err != nil {
		return nil, err
	}
	return v.encode(false)
}

func (v *nvencVideo) Surface() (nativeframe.Surface, bool, error) {
	var out nativeframe.Surface
	if !v.device {
		return out, false, nil
	}
	if v.native == nil {
		return out, false, errors.New("nvenc encoder is closed")
	}
	var context C.uint64_t
	var planes [3]C.uint64_t
	var strides [3]C.int
	if status := C.syntelle_nvenc_surface(v.native, &context, &planes[0], &strides[0]); status < 0 {
		return out, false, fmt.Errorf("prepare cuda encoder surface (native code %d)", int(status))
	}
	out.Context = uint64(context)
	out.Width = v.width
	out.Height = v.height
	for i := range 3 {
		out.Planes[i] = uint64(planes[i])
		out.Strides[i] = int32(strides[i])
	}
	return out, true, nil
}

func (v *nvencVideo) EncodeSurface() ([]byte, error) { return v.encode(true) }

func (v *nvencVideo) encode(device bool) ([]byte, error) {
	var data *C.uint8_t
	var size C.int
	key := C.int(0)
	if v.key {
		key = 1
	}
	var status C.int
	if device {
		status = C.syntelle_nvenc_encode_device(v.native, key, &data, &size)
	} else {
		status = C.syntelle_nvenc_encode(v.native, key, &data, &size)
	}
	if status < 0 {
		return nil, fmt.Errorf("encode nvenc frame (native code %d)", int(status))
	}
	if size < 1 || size > 16<<20 || data == nil {
		return nil, errors.New("invalid nvenc access unit")
	}
	v.key = false
	// Pion may retain parameter sets. Copy once into Go-owned packet bytes,
	// which survive subsequent GPU reads and encoder Close.
	return C.GoBytes(unsafe.Pointer(data), size), nil
}

func (v *nvencVideo) ForceKeyFrame() error {
	if v.native == nil {
		return errors.New("nvenc encoder is closed")
	}
	v.key = true
	return nil
}

func (v *nvencVideo) Close() error {
	C.syntelle_nvenc_close(v.native)
	v.native = nil
	return nil
}

// downloadSurface is an explicit offline inspection, never live transport.
// The returned view borrows C planes until the next encoder operation.
func (v *nvencVideo) downloadSurface() (*image.YCbCr, error) {
	if status := C.syntelle_nvenc_download(v.native); status < 0 {
		return nil, fmt.Errorf("inspect cuda planes (native code %d)", int(status))
	}
	var y, u, cr *C.uint8_t
	var ys, cs C.int
	if status := C.syntelle_nvenc_frame(v.native, &y, &u, &cr, &ys, &cs); status < 0 {
		return nil, fmt.Errorf("inspect cuda host view (native code %d)", int(status))
	}
	return &image.YCbCr{Rect: image.Rect(0, 0, v.width, v.height), SubsampleRatio: image.YCbCrSubsampleRatio420, YStride: int(ys), CStride: int(cs),
		Y: unsafe.Slice((*byte)(unsafe.Pointer(y)), int(ys)*v.height), Cb: unsafe.Slice((*byte)(unsafe.Pointer(u)), int(cs)*v.height/2), Cr: unsafe.Slice((*byte)(unsafe.Pointer(cr)), int(cs)*v.height/2)}, nil
}
