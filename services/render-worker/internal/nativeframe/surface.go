// Package nativeframe describes a borrowed encoder-owned CUDA I420 surface.
// Device addresses never point to Go memory and never cross the private API.
package nativeframe

type Surface struct {
	Context       uint64
	Planes        [3]uint64
	Strides       [3]int32
	Width, Height int
	Marker        bool
}
