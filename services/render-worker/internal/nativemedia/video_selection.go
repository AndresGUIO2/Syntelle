package nativemedia

import (
	"errors"
	"fmt"
	"image"
	"log/slog"
	"syntelle/render-worker/internal/nativeframe"
)

func videoEncoderMode(value string) (string, error) {
	switch value {
	case "", "auto":
		return "auto", nil
	case "nvenc", "openh264":
		return value, nil
	default:
		return "", errors.New("video encoder must be auto, nvenc or openh264")
	}
}

type videoConstructor func() (VideoEncoder, error)

type surfaceEncoder interface {
	Surface() (nativeframe.Surface, bool, error)
	EncodeSurface() ([]byte, error)
}

var errSurfaceRetry = errors.New("redraw current device frame for software encoder")

func (v *selectedVideo) Surface() (nativeframe.Surface, bool, error) {
	if encoder, ok := v.active.(surfaceEncoder); ok {
		surface, available, err := encoder.Surface()
		if err == nil || !v.hardware || v.software == nil {
			return surface, available, err
		}
		if err := v.fallback(); err != nil {
			return nativeframe.Surface{}, false, err
		}
	}
	return nativeframe.Surface{}, false, nil
}

func (v *selectedVideo) EncodeSurface() ([]byte, error) {
	encoder, ok := v.active.(surfaceEncoder)
	if !ok {
		return nil, errSurfaceRetry
	}
	data, err := encoder.EncodeSurface()
	if err == nil || !v.hardware || v.software == nil {
		return data, err
	}
	if err := v.fallback(); err != nil {
		return nil, err
	}
	return nil, errSurfaceRetry
}

// The session's existing video loop owns this encoder, including PLI handling
// and Close after its WaitGroup. No queue, media clock or goroutine is added.
type selectedVideo struct {
	active   VideoEncoder
	software videoConstructor
	hardware bool
}

func selectVideoEncoder(mode string, hardware, software videoConstructor) (VideoEncoder, error) {
	mode, err := videoEncoderMode(mode)
	if err != nil {
		return nil, err
	}
	if mode != "openh264" {
		encoder, err := hardware()
		if err == nil {
			slog.Info("Native video encoder selected", "encoder", "nvenc")
			v := &selectedVideo{active: encoder, hardware: true}
			if mode == "auto" {
				v.software = software
			}
			return v, nil
		}
		if mode == "nvenc" {
			return nil, fmt.Errorf("initialize required nvenc encoder: %w", err)
		}
		// Driver diagnostics are private; do not expose native strings to clients.
		slog.Info("NVENC unavailable; using OpenH264", "reason", "initialization_failed")
	}
	encoder, err := software()
	if err != nil {
		return nil, fmt.Errorf("initialize openh264 encoder: %w", err)
	}
	slog.Info("Native video encoder selected", "encoder", "openh264")
	return &selectedVideo{active: encoder}, nil
}

func (v *selectedVideo) fallback() error {
	previous := v.active
	v.active = nil
	v.hardware = false
	if err := previous.Close(); err != nil {
		slog.Warn("Failed NVENC encoder cleanup", "reason", "close_failed")
	}
	encoder, err := v.software()
	if err != nil {
		return fmt.Errorf("initialize fallback openh264 encoder: %w", err)
	}
	// A new SPS/PPS and IDR replace the GPU reference chain on the same RTP
	// track/timeline. Never send a partial GPU packet or retry stale frames.
	if err := encoder.ForceKeyFrame(); err != nil {
		return errors.Join(fmt.Errorf("force fallback keyframe: %w", err), encoder.Close())
	}
	v.active = encoder
	v.software = nil // Never oscillate or retry a failed GPU within this session.
	slog.Warn("Native video encoder switched", "encoder", "openh264", "reason", "nvenc_failed")
	return nil
}

func (v *selectedVideo) Encode(frame *image.RGBA) ([]byte, error) {
	if v.active == nil {
		return nil, errors.New("video encoder is closed")
	}
	data, err := v.active.Encode(frame)
	if err == nil || !v.hardware || v.software == nil {
		return data, err
	}
	if err := v.fallback(); err != nil {
		return nil, err
	}
	return v.active.Encode(frame)
}

func (v *selectedVideo) ForceKeyFrame() error {
	if v.active == nil {
		return errors.New("video encoder is closed")
	}
	err := v.active.ForceKeyFrame()
	if err == nil || !v.hardware || v.software == nil {
		return err
	}
	return v.fallback()
}

func (v *selectedVideo) Close() error {
	if v.active == nil {
		return nil
	}
	err := v.active.Close()
	v.active = nil
	return err
}
