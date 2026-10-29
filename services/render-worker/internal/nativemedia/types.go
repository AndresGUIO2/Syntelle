// Package nativemedia owns private native media sessions. It does not
// call providers or expose a public control endpoint.
package nativemedia

import (
	"context"
	"encoding/json"
	"image"
	"time"

	"github.com/pion/webrtc/v4"

	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"syntelle/render-worker/internal/nativeframe"
)

type Envelope = rendercontract.Envelope
type Viseme = rendercontract.Viseme

// Samples and Visemes are immutable for the lifetime of the callback.
type SpeechState struct {
	Streaming    bool
	LaughterSpan *rendercontract.LaughterSpan `json:"laughter_span,omitempty"`
	Preview      bool
	Active       bool
	Waiting      bool
	Revision     uint64
	TurnID       string
	Position     time.Duration
	Duration     time.Duration
	Samples      []int16
	SampleRate   int
	Visemes      []Viseme
	Text         string
	Performance  json.RawMessage
	Prepared     *nativeanimation.PreparedSpeech
}

type FrameInput struct {
	Activity       string
	EmotionControl *nativeanimation.EmotionSnapshot
	EmotionTime    float64

	PreviewKind        string
	PreviewRepetitions int
	PreviewAmount      float64
	PreviewSequence    uint64
	MediaTime          time.Duration
	Generation         uint64
	Settings           rendercontract.Settings
	Paused             bool
	Speech             SpeechState
}

type Scene interface {
	Render(context.Context, FrameInput, *image.RGBA) error
}

// deviceScene and surfaceEncoder borrow one surface on the same video-loop
// owner. Readback is reserved for explicit inspection or software fallback.
type deviceScene interface {
	RenderFrame(context.Context, FrameInput, *image.RGBA, *nativeframe.Surface) (bool, error)
	Readback(context.Context, *image.RGBA) error
}

// SpeechPreparer is implemented by character scenes. This method may run on
// the session preparation lane concurrently with Render; it may read only
// immutable configuration and input, never the scene's animation engine.
// Synthetic media-only scenes may omit it because they do not animate speech.
type SpeechPreparer interface {
	PrepareSpeech(context.Context, SpeechState) (*nativeanimation.PreparedSpeech, error)
}

type SceneFactory func(rendercontract.Settings) (Scene, error)

type Config struct {
	Capacity    int
	Width       int
	Height      int
	FPS         int
	PackageHash string
	// NetworkSettings is immutable worker configuration, never a client command.
	// Nil preserves direct container-network ICE used by offline comparisons.
	NetworkSettings *webrtc.SettingEngine
	// OfflineCalibration enables only the unpublished development probe. It is
	// false for the worker CLI and is never controlled by a session command.
	OfflineCalibration bool
	SceneFactory       SceneFactory
	// Emit must be bounded and must not wait on another session's media loop.
	Emit func(Envelope)
	// Encoders is optional. The nativecodecs build selects the production codecs.
	Encoders EncoderFactory
}

type VideoEncoder interface {
	Encode(*image.RGBA) ([]byte, error)
	ForceKeyFrame() error
	Close() error
}

type AudioEncoder interface {
	Encode([]int16) ([]byte, error)
	Close() error
}

type EncoderFactory interface {
	Video(width, height, fps, bitrate int) (VideoEncoder, error)
	Audio(sampleRate, channels, bitrate int) (AudioEncoder, error)
}

type Signal = rendercontract.Signal
type SessionDescription = rendercontract.SessionDescription

func event(kind, id, commandID string, data any) Envelope {
	encoded, err := json.Marshal(data)
	if err != nil {
		encoded = json.RawMessage(`{"message":"The private renderer failed."}`)
		kind = "error"
	}
	return Envelope{Type: kind, SessionID: id, CommandID: commandID, Data: encoded}
}
