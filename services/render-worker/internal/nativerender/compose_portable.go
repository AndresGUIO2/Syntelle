//go:build !ispc || !cgo || !linux || !amd64

package nativerender

import (
	"context"
	"image"
)

type composeBackend struct{}

func (r *Renderer) composePixels(ctx context.Context, dst *image.RGBA) error {
	return r.composeScalar(ctx, dst)
}
