package nativemedia

import (
	"errors"
	"image"
	"math"
	"time"
)

// MeasureTiming is an offline-only fixture, not a dispatchable worker command.
// Both marker and tone use the same session media interval.
func (h *Host) MeasureTiming(id string) error {
	if !h.config.OfflineCalibration {
		return errors.New("offline media calibration is disabled")
	}
	h.mu.Lock()
	s := h.sessions[id]
	h.mu.Unlock()
	if s == nil {
		return errors.New("native session not found")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	now := s.mediaTime
	if !s.lastTick.IsZero() {
		now += s.tickElapsedLocked()
	}
	s.calibrationStart = now + 250*time.Millisecond
	s.calibrationEnd = s.calibrationStart + 250*time.Millisecond
	return nil
}

func (s *session) calibration() (time.Duration, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calibrationStart, s.calibrationEnd
}

func timingMarker(dst *image.RGBA, at, start, end time.Duration) {
	if start <= 0 || at < start || at >= end {
		return
	}
	for y := 0; y < min(32, dst.Bounds().Dy()); y++ {
		row := dst.Pix[y*dst.Stride:]
		for x := 0; x < min(32, dst.Bounds().Dx()); x++ {
			p := x * 4
			row[p], row[p+1], row[p+2], row[p+3] = 255, 0, 255, 255
		}
	}
}

func timingTone(pcm []int16, pts, start, end time.Duration) {
	if start <= 0 || pts >= end || pts+20*time.Millisecond <= start {
		return
	}
	for i, original := range pcm {
		at := pts + time.Duration(i)*time.Second/sampleRate
		if at < start || at >= end {
			continue
		}
		phase := (at - start).Seconds() * 1000 * 2 * math.Pi
		value := int(math.Round(float64(original) + math.Sin(phase)*.15*32768))
		pcm[i] = int16(max(-32768, min(32767, value)))
	}
}
