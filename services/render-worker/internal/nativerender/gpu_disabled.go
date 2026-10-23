//go:build !nativegpu || !cgo || !linux || !amd64

package nativerender

import "errors"

func openGPU() (gpuRenderer, error) {
	return nil, errors.New("cuda renderer unavailable in this build")
}
