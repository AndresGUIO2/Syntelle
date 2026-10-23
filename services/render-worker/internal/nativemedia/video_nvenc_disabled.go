//go:build !nvenc || !nativecodecs || !cgo || !linux || !amd64

package nativemedia

import "errors"

func newNVENCVideo(width, height, fps, bitrate int) (VideoEncoder, error) {
	return nil, errors.New("nvenc is unavailable in this build")
}
