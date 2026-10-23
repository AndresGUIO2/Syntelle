package nativerender

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"syntelle/render-worker/internal/nativeframe"
)

type gpuRenderer interface {
	Render(context.Context, *Renderer, FrameState, *image.RGBA, bool, bool, bool) error
	Close() error
}
type fullGPURenderer interface {
	RenderAll(context.Context, *Renderer, FrameState, *image.RGBA, *nativeframe.Surface) error
	Readback(*image.RGBA) error
}

func (r *Renderer) disableGPU() {
	if r.gpu != nil {
		if err := r.gpu.Close(); err != nil {
			slog.Warn("Failed GPU renderer cleanup", "reason", "close_failed")
		}
	}
	r.gpu = nil
	r.gpuActive.Store(false)
	// CPU patches were deliberately not evaluated by the device path.
	r.initialized = false
	r.oral.hasLast = false
	r.eyes.hasLast = false
	slog.Warn("Native renderer switched", "renderer", "cpu", "reason", "gpu_failed")
}

func (r *Renderer) selectGPU(mode string) error {
	switch mode {
	case "", "auto", "gpu":
	case "cpu":
		return nil
	default:
		return errors.New("renderer must be auto, cpu or gpu")
	}
	r.gpuRequired = mode == "gpu"
	backend, err := openGPU()
	if err != nil {
		if r.gpuRequired {
			return fmt.Errorf("initialize required gpu renderer: %w", err)
		}
		slog.Info("Native renderer selected", "renderer", "cpu", "reason", "gpu_unavailable")
		return nil
	}
	r.gpu = backend
	r.gpuActive.Store(true)
	slog.Info("Native renderer selected", "renderer", "cuda")
	return nil
}

// The existing video-loop owner serializes rendering and closes the scene after
// its media goroutines exit. Cancellation never triggers a stale CPU replay.
func (r *Renderer) renderGPU(ctx context.Context, s FrameState, dst *image.RGBA, mouth, eyes, background bool) (bool, error) {
	if r.gpu == nil {
		return false, nil
	}
	err := r.gpu.Render(ctx, r, s, dst, mouth, eyes, background)
	if err == nil {
		return true, nil
	}
	if ctx.Err() != nil || r.gpuRequired {
		return true, err
	}
	if closeErr := r.gpu.Close(); closeErr != nil {
		slog.Warn("Failed GPU renderer cleanup", "reason", "close_failed")
	}
	r.gpu = nil
	r.gpuActive.Store(false)
	slog.Warn("Native renderer switched", "renderer", "cpu", "reason", "gpu_failed")
	return false, nil // Redraw this exact state in CPU; keep the media epoch.
}

func (r *Renderer) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	r.gpuActive.Store(false)
	if r.gpu == nil {
		return nil
	}
	err := r.gpu.Close()
	r.gpu = nil
	return err
}

func (r *Renderer) Backend() string {
	if r.gpuActive.Load() {
		return "native-cuda/1"
	}
	return "native-cpu/1"
}
