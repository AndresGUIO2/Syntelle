//go:build !libyuv || !cgo

package nativemedia

import "image"

func limitedI420(dst *image.YCbCr, src *image.RGBA) error { return scalarI420(dst, src) }
