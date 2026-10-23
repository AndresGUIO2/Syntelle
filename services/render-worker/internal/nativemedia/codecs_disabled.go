//go:build !nativecodecs || !cgo

package nativemedia

import (
	"errors"
)

func defaultEncoders() (EncoderFactory, error) {
	return nil, errors.New("native media requires a nativecodecs build")
}
