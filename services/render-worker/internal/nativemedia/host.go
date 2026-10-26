package nativemedia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"regexp"
	"sync"
	"time"

	"syntelle/render-contract"
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

type Host struct {
	mu       sync.Mutex
	createMu sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	closing  bool
	sessions map[string]*session
	config   Config
}

func New(parent context.Context, config Config) (*Host, error) {
	if config.Capacity == 0 {
		config.Capacity = 1
	}
	if config.Width == 0 {
		config.Width = 720
	}
	if config.Height == 0 {
		config.Height = 960
	}
	if config.FPS == 0 {
		config.FPS = 30
	}
	validSize := (config.Height == 960 && (config.Width == 720 || config.Width == 1440)) || (config.Height == 720 && (config.Width == 540 || config.Width == 1080))
	if config.Capacity < 1 || config.Capacity > 64 || !validSize || config.FPS != 30 || config.SceneFactory == nil || config.Emit == nil || config.PackageHash == "" {
		return nil, errors.New("invalid private native media configuration")
	}
	if config.Encoders == nil {
		factory, err := defaultEncoders()
		if err != nil {
			return nil, err
		}
		config.Encoders = factory
	}
	ctx, cancel := context.WithCancel(parent)
	h := &Host{ctx: ctx, cancel: cancel, config: config, sessions: map[string]*session{}}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.mu.Lock()
				sessions := make([]*session, 0, len(h.sessions))
				for _, s := range h.sessions {
					sessions = append(sessions, s)
				}
				h.mu.Unlock()
				for _, s := range sessions {
					h.emit(event("metrics", s.id, "", s.stats()))
				}
			}
		}
	}()
	return h, nil
}

func (h *Host) emit(value Envelope) {
	if h.ctx.Err() == nil {
		h.config.Emit(value)
	}
}

func (h *Host) Dispatch(ctx context.Context, command Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !identifier.MatchString(command.SessionID) {
		return errors.New("invalid private session identity")
	}
	if command.Type == "create" {
		return h.create(ctx, command)
	}
	h.mu.Lock()
	s := h.sessions[command.SessionID]
	h.mu.Unlock()
	if s == nil {
		return nil
	}
	switch command.Type {
	case "stop":
		s.cancel()
		select {
		case <-s.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case "interrupt":
		s.interrupt()
		return nil
	case "signal":
		return s.signal(command.Data)
	case "connect":
		return s.offer()
	case "preview":
		return s.preview(command.Data)
	case "configure":
		return s.configure(command.Data)
	case "conversation":
		return s.conversation(command.Data)
	default:
		return errors.New("unknown private native media command")
	}
}

func (h *Host) create(ctx context.Context, command Envelope) error {
	h.createMu.Lock()
	defer h.createMu.Unlock()
	h.mu.Lock()
	_, exists := h.sessions[command.SessionID]
	available := !h.closing && h.ctx.Err() == nil && len(h.sessions) < h.config.Capacity
	h.mu.Unlock()
	if exists {
		return nil
	}
	if !available {
		return errors.New("native media capacity is unavailable")
	}
	input := struct {
		Settings       rendercontract.Settings `json:"settings"`
		PackageHash    string                  `json:"package_hash"`
		PolicyRevision uint64                  `json:"emotion_policy_revision"`
	}{Settings: rendercontract.Defaults()}
	if err := json.Unmarshal(command.Data, &input); err != nil {
		return errors.New("invalid native media settings")
	}
	if err := input.Settings.Validate(); err != nil {
		return fmt.Errorf("native media settings: %w", err)
	}
	if input.PackageHash != "" && input.PackageHash != h.config.PackageHash {
		return errors.New("native character package mismatch")
	}
	s, err := h.newSession(command.SessionID, input.Settings)
	if err != nil {
		return err
	}
	s.policyRevision = input.PolicyRevision
	if s.policyRevision == 0 {
		s.policyRevision = 1
	}
	h.mu.Lock()
	if h.closing || h.ctx.Err() != nil || ctx.Err() != nil {
		h.mu.Unlock()
		s.cancel()
		s.release()
		return context.Canceled
	}
	h.sessions[s.id] = s
	h.wg.Add(1)
	h.mu.Unlock()
	s.wg.Add(3)
	go func() {
		defer h.wg.Done()
		<-s.ctx.Done()
		s.closePeer()
		s.wg.Wait()
		s.release()
		h.mu.Lock()
		if h.sessions[s.id] == s {
			delete(h.sessions, s.id)
		}
		h.mu.Unlock()
		close(s.done)
	}()
	go s.videoLoop()
	go s.audioLoop()
	go s.preparationLoop()
	if err = s.offer(); err != nil {
		s.cancel()
		<-s.done
		return err
	}
	displayWidth := h.config.Width
	if h.config.Width != 720 {
		displayWidth /= 2
	}
	h.emit(event("ready", s.id, "", map[string]any{"width": h.config.Width, "height": h.config.Height, "fps": h.config.FPS, "renderer": s.rendererBackend(), "package_hash": h.config.PackageHash, "video_layout": videoLayout(h.config.Width), "display_width": displayWidth, "display_height": h.config.Height}))
	return nil
}

func (h *Host) Stats(id string) (map[string]any, error) {
	h.mu.Lock()
	s := h.sessions[id]
	h.mu.Unlock()
	if s == nil {
		return nil, errors.New("native session not found")
	}
	return s.stats(), nil
}

func (h *Host) Active() int { h.mu.Lock(); defer h.mu.Unlock(); return len(h.sessions) }

func (h *Host) Close() error {
	h.mu.Lock()
	h.closing = true
	h.cancel()
	for _, s := range h.sessions {
		s.cancel()
	}
	h.mu.Unlock()
	// A create already in flight cannot publish after closing becomes true.
	h.createMu.Lock()
	h.createMu.Unlock()
	h.wg.Wait()
	return nil
}

// Snapshot is an explicit offline inspection copy, never a frame transport path.
func (h *Host) Snapshot(id string) (*image.RGBA, error) {
	h.mu.Lock()
	s := h.sessions[id]
	h.mu.Unlock()
	if s == nil {
		return nil, errors.New("native session not found")
	}
	s.frameMu.Lock()
	defer s.frameMu.Unlock()
	result := image.NewRGBA(s.frame.Bounds())
	if scene, ok := s.scene.(interface {
		Readback(context.Context, *image.RGBA) error
	}); ok {
		if err := scene.Readback(s.ctx, s.frame); err != nil {
			return nil, err
		}
	}
	copy(result.Pix, s.frame.Pix)
	return result, nil
}

func videoLayout(width int) string {
	if width == 1440 || width == 1080 {
		return "premultiplied-alpha-sbs/1"
	}
	return "opaque/1"
}
