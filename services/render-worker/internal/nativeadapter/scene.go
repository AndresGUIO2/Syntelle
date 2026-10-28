// Package nativeadapter binds immutable character assets and the original
// animation behavior to the native media scene interface.
package nativeadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"time"

	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"syntelle/render-worker/internal/nativeframe"
	"syntelle/render-worker/internal/nativemedia"
	"syntelle/render-worker/internal/nativerender"
)

type renderer interface {
	Render(context.Context, nativerender.FrameState, *image.RGBA) error
}

func (s *scene) Close() error {
	if closer, ok := s.renderer.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (s *scene) RendererBackend() string {
	if backend, ok := s.renderer.(interface{ Backend() string }); ok {
		return backend.Backend()
	}
	return "native-cpu/1"
}

func (s *scene) RenderMetrics() map[string]any {
	if measured, ok := s.renderer.(interface{ RenderMetrics() map[string]any }); ok {
		return measured.RenderMetrics()
	}
	return nil
}

func (s *scene) UnifiedLaughterAvailable() bool { return s.config.LaughterV2 }
func (s *scene) EmotionsAvailable() bool        { return s.config.Emotions }

// Animation/renderer state has one owner: nativemedia's video loop. The
// preparation lane reads only immutable Config and publishes immutable clips.
type scene struct {
	speechJawRig             *rendercontract.SpeechJawRig
	manualLaughter           bool
	previewSequence          uint64
	previewStart             time.Duration
	config                   nativeanimation.Config
	engine                   *nativeanimation.Engine
	renderer                 renderer
	initialized              bool
	generation               uint64
	animationSession         uint64
	mediaTime, animationTime time.Duration
	clipPrepared             *nativeanimation.PreparedSpeech
	speech                   *nativeanimation.Speech
}

func Load(assetDir string) (nativemedia.SceneFactory, string, error) {
	assets, err := nativerender.Load(assetDir)
	if err != nil {
		return nil, "", fmt.Errorf("load native scene assets: %w", err)
	}
	return LoadAssets(assets)
}

// LoadForeground is the live worker path. Offline opaque comparisons retain Load.
func LoadForeground(assetDir string) (nativemedia.SceneFactory, string, error) {
	return LoadForegroundSized(assetDir, 720, 960)
}

func LoadForegroundSized(assetDir string, width, height int) (nativemedia.SceneFactory, string, error) {
	assets, err := nativerender.LoadCharacter(assetDir)
	if err != nil {
		return nil, "", fmt.Errorf("load native foreground assets: %w", err)
	}
	return LoadAssetsSized(assets, width, height)
}

// LoadAssets reuses already loaded immutable resources for review/probe hosts.
func LoadAssets(assets *nativerender.Assets) (nativemedia.SceneFactory, string, error) {
	return LoadAssetsSized(assets, 720, 960)
}

func LoadAssetsSized(assets *nativerender.Assets, width, height int) (nativemedia.SceneFactory, string, error) {
	if assets == nil {
		return nil, "", errors.New("native scene assets unavailable")
	}
	config, err := nativeanimation.LoadBundle(assets.MotionJSON, assets.GazeJSON, assets.ExpressionJSON, assets.OralJSON)
	if err != nil {
		return nil, "", fmt.Errorf("load native scene animation: %w", err)
	}
	config.MotionScale = assets.MotionScale
	packageHash := assets.PackageHash
	if len(packageHash) != 64 {
		return nil, "", errors.New("native scene character identity unavailable")
	}
	factory := func(settings rendercontract.Settings) (nativemedia.Scene, error) {
		if err := settings.Validate(); err != nil {
			return nil, fmt.Errorf("prepare native scene settings: %w", err)
		}
		r, err := nativerender.NewSized(assets, width, height)
		if err != nil {
			return nil, fmt.Errorf("prepare native scene renderer: %w", err)
		}
		return &scene{config: config, engine: nativeanimation.NewEngine(config), renderer: r, speechJawRig: assets.Motion.SpeechJaw}, nil
	}
	return factory, packageHash, nil
}
func (s *scene) Render(ctx context.Context, input nativemedia.FrameInput, dst *image.RGBA) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state, err := s.sample(input)
	if err != nil {
		return err
	}
	return s.renderer.Render(ctx, state, dst)
}

func (s *scene) RenderFrame(ctx context.Context, input nativemedia.FrameInput, dst *image.RGBA, target *nativeframe.Surface) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	state, err := s.sample(input)
	if err != nil {
		return false, err
	}
	if renderer, ok := s.renderer.(interface {
		RenderFrame(context.Context, nativerender.FrameState, *image.RGBA, *nativeframe.Surface) (bool, error)
	}); ok {
		return renderer.RenderFrame(ctx, state, dst, target)
	}
	return false, s.renderer.Render(ctx, state, dst)
}

func (s *scene) Readback(ctx context.Context, dst *image.RGBA) error {
	if renderer, ok := s.renderer.(interface {
		Readback(context.Context, *image.RGBA) error
	}); ok {
		return renderer.Readback(ctx, dst)
	}
	return nil
}

// PrepareSpeech reads only immutable configuration. It runs on the media
// preparation lane; session admission/history remains with sample's owner.
func (s *scene) PrepareSpeech(ctx context.Context, clip nativemedia.SpeechState) (*nativeanimation.PreparedSpeech, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	samples := make([]float32, len(clip.Samples))
	for i, v := range clip.Samples {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		samples[i] = float32(v) / 32768
	}
	visemes := make([]nativeanimation.NativeViseme, len(clip.Visemes))
	for i, v := range clip.Visemes {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		visemes[i] = nativeanimation.NativeViseme{ID: v.ID, OffsetMS: v.OffsetMS}
	}
	var performance *nativeanimation.Performance
	_ = json.Unmarshal(clip.Performance, &performance)
	if err := nativeanimation.ValidateLaughterSpan(clip.LaughterSpan, clip.Samples, clip.SampleRate); err != nil {
		return nil, err
	}
	nonverbal := false
	if s.config.LaughterV2 && performance != nil {
		for _, cue := range performance.Resolved {
			if cue.Kind == "laugh" && clip.LaughterSpan == nil {
				return nil, errors.New("complete laughter requires a bound PCM span")
			}
		}
	}
	if span := clip.LaughterSpan; span != nil {
		found := false
		if performance != nil {
			for _, cue := range performance.Resolved {
				if cue.Kind == "laugh" && cue.Amount != nil && *cue.Amount > 0 && math.Abs(cue.Time*float64(clip.SampleRate)-float64(span.Offset)) <= 1 && math.Abs(cue.Duration*float64(clip.SampleRate)-float64(span.Count)) <= 1 {
					found = true
				}
			}
		}
		if found {
			admitted := nativeanimation.NewEngine(s.config).AdmitPerformance(performance, clip.Text, 0)
			found = false
			if admitted != nil {
				for _, cue := range admitted.Resolved {
					if cue.Kind == "laugh" {
						found = true
					}
				}
			}
		}
		if !found {
			return nil, errors.New("laughter span has no matching admitted direction")
		}
		nonverbal = span.Offset == 0 && span.Count == len(samples)
	} else if performance != nil && len(performance.Resolved) == 1 {
		cue := performance.Resolved[0]
		duration := float64(len(samples)) / float64(clip.SampleRate)
		nonverbal = cue.Kind == "laugh" && cue.Time == 0 && duration <= 2.5 && math.Abs(cue.Duration-duration) < .002 && cue.Amount != nil && *cue.Amount > 0 && *cue.Amount <= 1
	}
	prepared, err := nativeanimation.PrepareSpeech(ctx, samples, clip.SampleRate, visemes, nativeanimation.SpeechSeed(clip.Text), s.config, nonverbal)
	if err != nil {
		return nil, fmt.Errorf("prepare native scene speech: %w", err)
	}
	if err = nativeanimation.AttachLaughterSpan(ctx, prepared, samples, clip.SampleRate, clip.LaughterSpan, s.config); err != nil {
		return nil, err
	}
	// Invalid optional performance does not prevent original PCM replay.
	if len(clip.Performance) > 0 {
		if err := json.Unmarshal(clip.Performance, &prepared.Performance); err != nil {
			prepared.Performance = nil
		}
	}
	return prepared, ctx.Err()
}
func (s *scene) sample(input nativemedia.FrameInput) (nativerender.FrameState, error) {
	if s.initialized && input.MediaTime < s.mediaTime {
		return nativerender.FrameState{}, errors.New("native scene media clock moved backwards")
	}
	if !s.initialized {
		s.mediaTime = input.MediaTime
		s.generation = input.Generation
		s.initialized = true
	}
	if input.Generation != s.generation {
		s.generation = input.Generation
		s.animationSession++
		s.speech = nil
		s.clipPrepared = nil
	}
	delta := input.MediaTime - s.mediaTime
	if !input.Paused {
		s.animationTime += delta
	}
	s.mediaTime = input.MediaTime
	clip := input.Speech
	if clip.Active && clip.Prepared == nil {
		return nativerender.FrameState{}, errors.New("native scene speech analysis is not ready")
	}
	if clip.Prepared != nil && clip.Prepared != s.clipPrepared {
		performance := s.engine.AdmitPerformance(clip.Prepared.Performance, clip.Text, float64(input.MediaTime)/float64(time.Millisecond))
		s.speech = nativeanimation.BindSpeech(clip.Prepared, performance)
		s.clipPrepared = clip.Prepared
		s.manualLaughter = clip.Preview
		s.animationSession++
	}
	position := clip.Position.Seconds()
	if !clip.Active && s.speech != nil {
		position = s.speech.Duration
	}
	speech := nativeanimation.SpeechState{Active: clip.Active, Waiting: clip.Waiting, Position: position, Session: s.animationSession, Speech: s.speech, ReplyID: clip.TurnID}
	settings := nativeanimation.Settings{ManualLaughter: s.config.LaughterV2 && s.manualLaughter, Activity: input.Activity, EmotionControl: input.EmotionControl, EmotionTime: input.EmotionTime, Movement: input.Settings.Movement, Blinking: input.Settings.Blinking, AutomaticExpressions: input.Settings.AutomaticExpressions, Smile: input.Settings.Smile, Brows: input.Settings.Brows, ReducedMotion: input.Settings.ReducedMotion, Paused: input.Paused, EmotionBaseline: input.Settings.EmotionBaseline}
	if clip.Preview && !s.config.LaughterV2 && !settings.ReducedMotion {
		settings.AutomaticExpressions = true
	}
	f := s.engine.Sample(float64(s.animationTime)/float64(time.Millisecond), speech, settings)
	if !clip.Active && !s.engine.LaughterPresenting() {
		s.manualLaughter = false
	}
	if input.PreviewSequence != s.previewSequence {
		s.previewSequence = input.PreviewSequence
		s.previewStart = s.animationTime
	}
	if input.PreviewKind != "" && !clip.Active && !clip.Waiting && !input.Settings.ReducedMotion {
		f = nativeanimation.PreviewFrameRepeated(f, input.PreviewKind, (s.animationTime - s.previewStart).Seconds(), input.PreviewAmount, input.PreviewRepetitions, s.config)
	}

	// Waiting for audio retains the original resting mouth and manual smile.
	// Articulation starts only when PCM playback becomes active.
	restingSmile := f.Emotion == (rendercontract.EmotionWeights{}) && !clip.Active && (!clip.Waiting || s.config.SpeakingSmile) && f.Laugh == 0
	if s.config.LaughterV2 && f.Laugh > 0 && !f.LaughSpeech && f.Emotion == (rendercontract.EmotionWeights{}) {
		restingSmile = true
	}
	if s.config.ReferenceLaugh && !s.config.LaughterV2 && clip.Preview && f.Laugh < 1./240 {
		restingSmile = true
	}
	return nativerender.FrameState{SpeechJaw: speechJawAt(speech, s.speechJawRig, clip.Preview, input.Settings.ReducedMotion, f.Laugh), LaughSpeech: f.LaughSpeech, JawOpen: f.JawOpen, LipOpening: f.LipOpening, LipTension: f.LipTension, Emotion: f.Emotion, Head: [3]float64{f.Head.Yaw, f.Head.Pitch, f.Head.Roll}, Breath: f.Breath, From: f.Mouth.From, To: f.Mouth.To, Amount: f.Mouth.Amount, Closure: f.EyeClosure, Gaze: f.Gaze, Smile: f.Smile, Brows: f.Brows, Laugh: f.Laugh, RestingSmile: restingSmile, Background: input.Settings.Background, Blur: input.Settings.Blur}, nil
}

// Called only on the video owner while its frame lock is held.
func (s *scene) LaughterPresenting() bool { return s.engine.LaughterPresenting() }
