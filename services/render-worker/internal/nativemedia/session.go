package nativemedia

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"syntelle/render-worker/internal/nativeclock"
	"syntelle/render-worker/internal/nativeframe"
)

type session struct {
	activity          string
	coordinated       bool
	delivery          *pendingDelivery
	requestedSettings *rendercontract.Settings
	requestedRevision uint64
	emotionInstance   string

	emotionController nativeanimation.EmotionController
	emotionClock      float64
	emotionEpoch      uint64
	policyRevision    uint64

	emotionSuppressedTurn string
	previewEnd            string
	previewTurn           string
	previewKind           string
	previewAmount         float64
	previewRepetitions    int
	previewSequence       uint64
	sendMu                sync.Mutex
	frameMu               sync.Mutex
	presentation          SpeechState
	waiting               bool
	videoPackets          rtp.Packetizer
	audioPackets          rtp.Packetizer
	videoBase             uint32
	audioBase             uint32
	id                    string
	host                  *Host
	ctx                   context.Context
	cancel                context.CancelFunc
	done                  chan struct{}
	wg                    sync.WaitGroup
	mu                    sync.Mutex
	settings              rendercontract.Settings
	paused                bool
	generation            uint64
	revision              uint64
	turn                  string
	ignored               map[string]bool
	queue                 []*clip
	queuedBytes           int
	preparationWake       chan struct{}
	preparing             *preparationTask
	nextIndex             int
	complete              bool
	playing               *clip
	position              int
	started               bool
	mediaTime             time.Duration
	lastTick              time.Time
	lastRawTick           time.Duration
	calibrationStart      time.Duration
	calibrationEnd        time.Duration
	created               time.Time
	createdRaw            time.Duration
	scene                 Scene
	frame                 *image.RGBA
	video                 VideoEncoder
	audio                 AudioEncoder
	peerMu                sync.Mutex
	offerMu               sync.Mutex
	peer                  *webrtc.PeerConnection
	videoTrack            *webrtc.TrackLocalStaticRTP
	audioTrack            *webrtc.TrackLocalStaticRTP
	candidates            []webrtc.ICECandidateInit
	connected             atomic.Bool
	keyframe              atomic.Bool
	captured              atomic.Uint64
	encoded               atomic.Uint64
	audioFrames           atomic.Uint64
	videoBytes            atomic.Uint64
	audioBytes            atomic.Uint64
	renderNS              atomic.Uint64
	videoEncodeNS         atomic.Uint64
	audioEncodeNS         atomic.Uint64
	lateFrames            atomic.Uint64
	deviceFrames          atomic.Uint64
	repeatedVideoPTS      atomic.Uint64
}

func (h *Host) newSession(id string, settings rendercontract.Settings) (*session, error) {
	ctx, cancel := context.WithCancel(h.ctx)
	s := &session{emotionInstance: rand.Text(), id: id, host: h, ctx: ctx, cancel: cancel, done: make(chan struct{}), settings: settings, ignored: map[string]bool{}, preparationWake: make(chan struct{}, 1), created: time.Now(), createdRaw: nativeclock.Now(), frame: image.NewRGBA(image.Rect(0, 0, h.config.Width, h.config.Height))}
	var err error
	s.scene, err = h.config.SceneFactory(settings)
	if err == nil {
		s.video, err = h.config.Encoders.Video(h.config.Width, h.config.Height, h.config.FPS, 2500000)
	}
	if err == nil {
		s.audio, err = h.config.Encoders.Audio(sampleRate, 1, 32000)
	}
	if err != nil {
		cancel()
		s.release()
		return nil, fmt.Errorf("prepare native media session: %w", err)
	}
	return s, nil
}

func (s *session) release() {
	s.frameMu.Lock()
	defer s.frameMu.Unlock()
	if s.video != nil {
		if err := s.video.Close(); err != nil {
			s.host.emit(event("error", s.id, "", map[string]string{"message": "The private video encoder could not close."}))
		}
	}
	if s.audio != nil {
		if err := s.audio.Close(); err != nil {
			s.host.emit(event("error", s.id, "", map[string]string{"message": "The private audio encoder could not close."}))
		}
	}
	if closer, ok := s.scene.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			s.host.emit(event("error", s.id, "", map[string]string{"message": "The private character could not close."}))
		}
	}
}

func (s *session) fail(err error) {
	if s.ctx.Err() != nil {
		return
	}
	slog.Warn("Native media session stopped", "session_id", s.id, "error", err)
	s.host.emit(event("error", s.id, "", map[string]string{"message": "The private native renderer failed."}))
	s.cancel()
}

func (s *session) interrupt() {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.mu.Lock()
	s.invalidateDeliveryLocked("interrupted")
	s.emotionController.End(s.emotionClock, true)
	s.emotionEpoch++
	s.rememberTurn()
	s.previewEnd = ""
	s.previewTurn = ""
	s.previewKind = ""
	s.waiting = false
	s.generation++
	s.revision++
	s.turn = ""
	s.queue = nil
	// An interrupted in-flight analysis keeps its PCM reservation until it has
	// observed cancellation and released its temporary conversion buffer.
	s.queuedBytes = s.cancelPreparationLocked()
	s.nextIndex = 0
	s.complete = false
	s.playing = nil
	s.position = 0
	s.started = false
	s.presentation = SpeechState{}
	s.mu.Unlock()
	s.keyframe.Store(true)
}

func (s *session) configure(data json.RawMessage) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	var wrapped struct {
		Settings       json.RawMessage `json:"settings"`
		PolicyRevision uint64          `json:"emotion_policy_revision"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return errors.New("invalid native settings")
	}
	if len(wrapped.Settings) > 0 {
		data = wrapped.Settings
	}
	next := s.settings
	if err := json.Unmarshal(data, &next); err != nil {
		return errors.New("invalid native settings")
	}
	if err := next.Validate(); err != nil {
		return err
	}
	var optional struct {
		Paused *bool `json:"paused"`
	}
	if err := json.Unmarshal(data, &optional); err != nil {
		return errors.New("invalid native pause setting")
	}
	alreadyRequested := s.requestedSettings != nil && performanceSettingsEqual(next, *s.requestedSettings) && wrapped.PolicyRevision == s.requestedRevision
	if alreadyRequested {
		next = preservePerformanceSettings(next, s.settings)
	}
	if s.coordinated && next.ReducedMotion != s.settings.ReducedMotion {
		s.invalidateDeliveryLocked("accessibility_changed")
		s.requestedSettings = nil
	}
	if s.turn == "" {
		s.requestedSettings = nil
	}
	if !alreadyRequested && s.coordinated && s.turn != "" && !performanceSettingsEqual(next, s.settings) && next.ReducedMotion == s.settings.ReducedMotion {
		copy := next
		s.requestedSettings = &copy
		s.requestedRevision = wrapped.PolicyRevision
		if s.requestedRevision == 0 {
			s.requestedRevision = max(s.policyRevision, s.requestedRevision) + 1
		}
		s.invalidateDeliveryLocked("superseded_by_settings")
		next = preservePerformanceSettings(next, s.settings)
	} else if !alreadyRequested && s.coordinated && s.requestedSettings != nil && performanceSettingsEqual(next, s.settings) {
		// A -> B -> A is a distinct requested revision, not revival of B.
		copy := next
		s.requestedSettings = &copy
		s.requestedRevision = wrapped.PolicyRevision
		s.invalidateDeliveryLocked("superseded_by_settings")
	}
	if next.EmotionBaseline.Normalized() != s.settings.EmotionBaseline.Normalized() || next.EmotionRegulation.Normalized() != s.settings.EmotionRegulation.Normalized() {
		s.emotionSuppressedTurn = s.turn
	}
	if next.EmotionBaseline.Normalized() != s.settings.EmotionBaseline.Normalized() || next.EmotionRegulation.Normalized() != s.settings.EmotionRegulation.Normalized() || next.AutomaticExpressions != s.settings.AutomaticExpressions || next.ReducedMotion != s.settings.ReducedMotion {
		s.emotionEpoch++
		if wrapped.PolicyRevision > 0 {
			s.policyRevision = wrapped.PolicyRevision
		} else {
			s.policyRevision++
		}
	}
	s.settings = next
	if optional.Paused != nil {
		if *optional.Paused && !s.paused {
			// The presentation normally describes the start of the last emitted
			// 20 ms block. Freeze at its already-emitted source cursor now;
			// a later silence tick must not move the acknowledged pause position.
			if s.playing != nil {
				s.presentation = s.clipState(s.playing, s.position)
			} else if s.presentation.Active {
				s.presentation.Position = s.presentation.Duration
			}
		}
		s.paused = *optional.Paused
	}
	s.revision++
	return nil
}

func (s *session) conversation(raw json.RawMessage) error {
	var e struct {
		Type   string          `json:"type"`
		TurnID string          `json:"turn_id"`
		Text   string          `json:"text"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return errors.New("invalid private conversation event")
	}
	if e.Type == "accepted" {
		s.mu.Lock()
		blocked := s.ignored[e.TurnID] || s.turn == e.TurnID
		s.mu.Unlock()
		if e.TurnID == "" || blocked {
			return nil
		}
		s.interrupt()
		s.mu.Lock()
		s.turn = e.TurnID
		s.revision++
		s.mu.Unlock()
		return nil
	}
	s.mu.Lock()
	generation := s.generation
	valid := e.TurnID != "" && e.TurnID == s.turn && !s.ignored[e.TurnID]
	s.mu.Unlock()
	if !valid {
		return nil
	}
	if e.Type == "cancelled" || e.Type == "error" {
		s.interrupt()
		return nil
	}
	if e.Type == "speech_complete" {
		var complete struct {
			Parts *int `json:"parts"`
		}
		if err := json.Unmarshal(e.Data, &complete); err != nil {
			return errors.New("invalid speech completion")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if generation != s.generation || e.TurnID != s.turn {
			return nil
		}
		if complete.Parts != nil && (*complete.Parts < 1 || *complete.Parts != s.nextIndex) {
			return errors.New("speech completion does not match received parts")
		}
		s.complete = true
		s.revision++
		return nil
	}
	var prepared preparedSpeech
	var text string
	index := 0
	direct := e.Type == "prepared_speech"
	if direct {
		if err := json.Unmarshal(e.Data, &prepared); err != nil {
			return errors.New("invalid prepared speech")
		}
		text = e.Text
	} else if e.Type == "speech_part" {
		var part speechPart
		if err := json.Unmarshal(e.Data, &part); err != nil {
			return errors.New("invalid prepared speech part")
		}
		prepared = part.Speech
		text = part.Text
		index = part.Index
	} else {
		return nil
	}
	decoded, err := decodeClip(prepared, text)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation || e.TurnID != s.turn {
		return nil
	}
	var metadata struct {
		Intent *struct {
			Version string `json:"version"`
		} `json:"intent"`
	}
	_ = json.Unmarshal(decoded.performance, &metadata)
	if metadata.Intent != nil && metadata.Intent.Version == rendercontract.CoordinatedPerformanceVersion && decoded.delivery == nil {
		return errors.New("coordinated speech requires an approved delivery")
	}
	if decoded.delivery != nil && !s.deliveryMatchesLocked(decoded.delivery, text, index) {
		if s.delivery != nil && decoded.delivery.Token == s.delivery.reservation.Token && s.emotionClock > s.delivery.expires {
			s.invalidateDeliveryLocked("expired_delivery")
		}
		return nil
	}
	if index < s.nextIndex {
		return nil
	}
	parts := len(s.queue)
	if s.playing != nil {
		parts++
	}
	if s.preparing != nil && s.preparing.detached {
		parts++
	}
	if index != s.nextIndex || parts >= maxQueuedParts || s.queuedBytes+len(decoded.samples)*2 > maxQueuedPCMBytes {
		return errors.New("speech queue capacity or ordering exceeded")
	}
	decoded.phraseIndex = index
	_, decoded.needsPreparation = s.scene.(SpeechPreparer)
	s.queue = append(s.queue, decoded)
	s.queuedBytes += len(decoded.samples) * 2
	s.nextIndex++
	s.complete = s.complete || direct
	s.revision++
	s.notifyPreparationLocked()
	return nil
}

func (s *session) frameInput() FrameInput {
	s.mu.Lock()
	defer s.mu.Unlock()
	mediaTime := s.mediaTime
	if !s.lastTick.IsZero() {
		mediaTime += s.tickElapsedLocked()
	}
	state := s.presentation
	state.Revision = s.revision
	if !state.Active {
		state.TurnID = s.turn
		state.Waiting = s.turn != "" && (!s.complete || s.preparationPendingLocked())
	}
	if state.Active && !s.paused && !s.lastTick.IsZero() {
		state.Position = min(state.Duration, state.Position+s.tickElapsedLocked())
	}
	var controlled *nativeanimation.EmotionSnapshot
	clock := s.emotionClock
	if !s.paused {
		clock += float64(s.tickElapsedLocked()) / float64(time.Millisecond)
	}
	capability, available := s.scene.(interface{ EmotionsAvailable() bool })
	unified := false
	if value, ok := s.scene.(interface{ UnifiedLaughterAvailable() bool }); ok {
		unified = value.UnifiedLaughterAvailable()
	}
	if available && capability.EmotionsAvailable() && (!state.Preview || unified) && (s.coordinated || s.settings.EmotionRegulation.Normalized().Enabled || s.emotionController.Controlled(state.TurnID)) {
		copyController := s.emotionController
		copyController.Configure(clock, s.settings, state.TurnID)
		value := copyController.Motion
		controlled = &value
	}
	return FrameInput{Activity: s.activity, EmotionControl: controlled, EmotionTime: clock, PreviewKind: s.previewKind, PreviewAmount: s.previewAmount, PreviewRepetitions: s.previewRepetitions, PreviewSequence: s.previewSequence, MediaTime: mediaTime, Generation: s.generation, Settings: s.settings, Paused: s.paused, Speech: state}
}

func (s *session) videoLoop() {
	defer s.wg.Done()
	interval := time.Second / time.Duration(s.host.config.FPS)
	ticker := nativeclock.NewPacer(interval)
	defer ticker.Close()
	// Keep one frame in flight. Variable rendering/encoding work must not become
	// variable network delivery cadence; timestamps still use the PCM epoch.
	pacer := nativeclock.NewPacer(interval)
	defer pacer.Close()
	var lastPTS time.Duration
	hasLastPTS := false
	for {
		tick, ok := ticker.Next(s.ctx)
		if !ok {
			return
		}
		{
			if !s.connected.Load() {
				continue
			}
			input := s.frameInput()
			if s.keyframe.Swap(false) {
				if err := s.video.ForceKeyFrame(); err != nil {
					s.fail(err)
					return
				}
			}
			var target *nativeframe.Surface
			if encoder, ok := s.video.(surfaceEncoder); ok {
				start := nativeclock.Now()
				surface, available, err := encoder.Surface()
				s.videoEncodeNS.Add(uint64((nativeclock.Now() - start)))
				if err != nil {
					s.fail(err)
					return
				}
				if available {
					target = &surface
				}
			}
			if target != nil && s.host.config.OfflineCalibration {
				start, end := s.calibration()
				target.Marker = start > 0 && input.MediaTime >= start && input.MediaTime < end
			}
			start := nativeclock.Now()
			s.frameMu.Lock()
			var err error
			usedDevice := false
			if scene, ok := s.scene.(deviceScene); ok {
				usedDevice, err = scene.RenderFrame(s.ctx, input, s.frame, target)
			} else {
				err = s.scene.Render(s.ctx, input, s.frame)
			}
			if err == nil && !usedDevice && s.host.config.OfflineCalibration {
				start, end := s.calibration()
				timingMarker(s.frame, input.MediaTime, start, end)
			}
			presenting := false
			if owner, ok := s.scene.(interface{ LaughterPresenting() bool }); ok {
				presenting = owner.LaughterPresenting()
			}
			s.frameMu.Unlock()
			s.renderNS.Add(uint64((nativeclock.Now() - start)))
			if err != nil {
				s.fail(err)
				return
			}
			s.captured.Add(1)
			if usedDevice {
				s.deviceFrames.Add(1)
			}
			if (nativeclock.Now() - start) > interval {
				s.lateFrames.Add(1)
			}
			start = nativeclock.Now()
			var data []byte
			if usedDevice {
				data, err = s.video.(surfaceEncoder).EncodeSurface()
				if errors.Is(err, errSurfaceRetry) {
					s.frameMu.Lock()
					err = s.scene.(deviceScene).Readback(s.ctx, s.frame)
					s.frameMu.Unlock()
					if err == nil {
						data, err = s.video.Encode(s.frame)
					}
				}
			} else {
				data, err = s.video.Encode(s.frame)
			}
			s.videoEncodeNS.Add(uint64((nativeclock.Now() - start)))
			if err != nil {
				s.fail(err)
				return
			}
			if len(data) == 0 {
				continue
			}
			if !pacer.WaitUntil(s.ctx, tick+interval) {
				return
			}
			sent, err := s.sendEncoded(data, true, input.MediaTime, input.Generation)
			if err != nil {
				s.fail(err)
				return
			}
			if !sent {
				s.keyframe.Store(true)
				continue
			}
			s.finishPreviewPresentation(input.Generation, presenting)
			s.encoded.Add(1)
			if hasLastPTS && input.MediaTime <= lastPTS {
				s.repeatedVideoPTS.Add(1)
			}
			lastPTS = input.MediaTime
			hasLastPTS = true
		}
	}
}

func (s *session) audioLoop() {
	defer s.wg.Done()
	ticker := nativeclock.NewPacer(20 * time.Millisecond)
	defer ticker.Close()
	pcm := make([]int16, audioSamples)
	for {
		if _, ok := ticker.Next(s.ctx); !ok {
			return
		}
		if !s.connected.Load() {
			continue
		}
		// Keep configuration acceptance outside the first-PCM send/commit transaction.
		s.sendMu.Lock()
		block := s.nextAudioBlock(time.Now(), pcm)
		if s.host.config.OfflineCalibration {
			start, end := s.calibration()
			timingTone(pcm, block.pts, start, end)
		}
		start := nativeclock.Now()
		data, err := s.audio.Encode(pcm)
		s.audioEncodeNS.Add(uint64(nativeclock.Now() - start))
		if err != nil {
			s.sendMu.Unlock()
			s.fail(err)
			return
		}
		sent, err := s.sendEncodedLocked(data, false, block.pts, block.generation)
		if err != nil {
			s.sendMu.Unlock()
			s.fail(err)
			return
		}
		decisions := s.commitEmotions(block, sent)
		s.sendMu.Unlock()
		if !sent {
			if block.delivery != nil {
				s.fail(errors.New("reserved phrase audio was not published"))
				return
			}
			continue
		}
		for _, decision := range decisions {
			s.host.emit(event("emotion_decision", s.id, decision.TurnID, decision))
		}
		for _, notification := range block.events {
			if s.deferPreviewEnd(notification, block.generation) {
				continue
			}
			s.host.emit(event("playback", s.id, notification.turn, map[string]string{"event": notification.kind, "turn_id": notification.turn}))
		}
		for _, activation := range block.activations {
			s.mu.Lock()
			valid := activation.Generation == s.generation && s.emotionSuppressedTurn != activation.TurnID && activation.Baseline == s.settings.EmotionBaseline.Normalized() && !s.settings.EmotionRegulation.Normalized().Enabled
			s.mu.Unlock()
			if valid {
				s.host.emit(event("emotion_activated", s.id, activation.TurnID, activation))
			}
		}
		s.audioFrames.Add(1)
	}
}

func (s *session) stats() map[string]any {
	input := s.frameInput()
	connection := "new"
	reports := []any{
		map[string]any{"id": "native-video", "type": "outbound-rtp", "kind": "video", "codecId": "native-h264", "bytesSent": s.videoBytes.Load(), "framesEncoded": s.encoded.Load(), "framesSent": s.encoded.Load(), "frameWidth": s.host.config.Width, "frameHeight": s.host.config.Height, "totalEncodeTime": float64(s.videoEncodeNS.Load()) / float64(time.Second)},
		map[string]any{"id": "native-audio", "type": "outbound-rtp", "kind": "audio", "codecId": "native-opus", "bytesSent": s.audioBytes.Load()},
		map[string]any{"id": "native-h264", "type": "codec", "mimeType": "video/H264", "clockRate": 90000},
		map[string]any{"id": "native-opus", "type": "codec", "mimeType": "audio/opus", "clockRate": 48000, "channels": 1},
	}
	s.peerMu.Lock()
	pc := s.peer
	s.peerMu.Unlock()
	if pc != nil {
		connection = pc.ConnectionState().String()
		for _, value := range pc.GetStats() {
			switch value.(type) {
			case webrtc.RemoteInboundRTPStreamStats, webrtc.ICECandidatePairStats:
				reports = append(reports, value)
			}
		}
	}
	renderer := s.rendererBackend()
	var stages map[string]any
	if measured, ok := s.scene.(interface{ RenderMetrics() map[string]any }); ok {
		stages = measured.RenderMetrics()
	}
	return map[string]any{"width": s.host.config.Width, "height": s.host.config.Height, "captured_frames": s.captured.Load(), "encoded_frames": s.encoded.Load(), "audio_frames": s.audioFrames.Load(), "elapsed_ms": float64(nativeclock.Now()-s.createdRaw) / float64(time.Millisecond), "audio_context_seconds": input.MediaTime.Seconds(), "speech_position_seconds": input.Speech.Position.Seconds(), "speech_generation": input.Generation, "connection": connection, "renderer": renderer, "device_surface_frames": s.deviceFrames.Load(), "repeated_video_timestamps": s.repeatedVideoPTS.Load(), "codec": "H264/Opus", "render_total_ms": float64(s.renderNS.Load()) / float64(time.Millisecond), "video_encode_total_ms": float64(s.videoEncodeNS.Load()) / float64(time.Millisecond), "audio_encode_total_ms": float64(s.audioEncodeNS.Load()) / float64(time.Millisecond), "late_render_frames": s.lateFrames.Load(), "render_stages": stages, "reports": reports}
}

func (s *session) rendererBackend() string {
	if backend, ok := s.scene.(interface{ RendererBackend() string }); ok {
		return backend.RendererBackend()
	}
	return "native-cpu/1"
}

type playbackNotification struct{ kind, turn string }
type audioBlock struct {
	delivery       *pendingDelivery
	applied        *rendercontract.Settings
	policyRevision uint64
	controller     nativeanimation.EmotionController
	clock          float64
	epoch          uint64
	policy         bool
	decisions      []rendercontract.EmotionDecision

	activations []rendercontract.EmotionActivation
	generation  uint64
	pts         time.Duration
	events      []playbackNotification
}

// Adjacent prepared phrases fill the same Opus block without adding per-phrase
// silence. Only the final utterance tail is zero padded to the codec frame size.
func (s *session) nextAudioBlock(now time.Time, pcm []int16) (block audioBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(pcm)
	s.mediaTime += 20 * time.Millisecond
	s.lastTick = now
	if s.createdRaw != 0 {
		s.lastRawTick = nativeclock.Now()
	}
	block = audioBlock{generation: s.generation, pts: s.mediaTime, epoch: s.emotionEpoch, clock: s.emotionClock}
	capability, emotional := s.scene.(interface{ EmotionsAvailable() bool })
	block.policy = emotional && capability.EmotionsAvailable()
	block.controller = s.emotionController
	if s.policyRevision == 0 {
		s.policyRevision = 1
	}
	if !s.paused {
		block.clock += 20
	}
	if block.policy && block.controller.Configure(block.clock, s.settings, s.turn) {
		block.decisions = append(block.decisions, block.controller.Evidence(nil, "configuration", "", -1, s.generation))
	}
	defer func() {
		if block.policy && block.controller.Advance(block.clock, s.turn != "" && (s.started || !s.complete)) {
			block.decisions = append(block.decisions, block.controller.Evidence(nil, "clock_transition", "", -1, s.generation))
		}
		for i := range block.decisions {
			block.decisions[i].PolicyRevision = s.policyRevision
			if block.policyRevision > 0 {
				block.decisions[i].PolicyRevision = block.policyRevision
			}
			block.decisions[i].Instance = s.emotionInstance
		}
	}()
	if s.paused {
		return block
	}
	s.presentation = SpeechState{Revision: s.revision, TurnID: s.turn, Waiting: s.turn != "" && (!s.complete || s.preparationPendingLocked()), SampleRate: sampleRate}
	filled := 0
	for filled < len(pcm) {
		if s.playing == nil {
			if s.preparationPendingLocked() {
				if filled == 0 && s.started && !s.waiting {
					block.events = append(block.events, playbackNotification{"playback_waiting", s.turn})
					s.waiting = true
				}
				break
			}
			if len(s.queue) == 0 {
				if filled == 0 && s.started {
					if s.complete {
						if block.policy && block.controller.HasPlayedProposal(s.turn) {
							block.controller.End(block.clock, false)
							block.decisions = append(block.decisions, block.controller.Evidence(nil, "reply_ended", s.turn, -1, s.generation))
						}
						block.events = append(block.events, playbackNotification{"playback_ended", s.turn})
						s.rememberTurn()
						s.started = false
						s.turn = ""
						if s.requestedSettings != nil {
							copy := *s.requestedSettings
							block.applied = &copy
							block.policyRevision = s.requestedRevision
							block.controller.Configure(block.clock, copy, "")
							block.decisions = append(block.decisions, block.controller.Evidence(nil, "configuration", "", -1, s.generation))
						}
						s.revision++
					} else if !s.waiting {
						block.events = append(block.events, playbackNotification{"playback_waiting", s.turn})
						s.waiting = true
					}
				}
				break
			}
			candidate := s.queue[0]
			if candidate.delivery != nil && !s.deliveryMatchesLocked(candidate.delivery, candidate.text, candidate.phraseIndex) {
				s.invalidateDeliveryLocked("expired_delivery")
				break
			}
			s.playing = candidate
			s.queue = s.queue[1:]
			s.position = 0
			s.started = true
			s.waiting = false
			s.revision++
			block.events = append(block.events, playbackNotification{"playback_started", s.turn})
			if s.playing.delivery != nil {
				block.delivery = s.delivery
				copy := s.delivery.settings
				block.applied = &copy
				block.policyRevision = s.delivery.reservation.Request.Revision
				block.decisions = append(block.decisions, block.controller.ActivateReserved(block.clock, s.delivery.policy))
			} else if emotional && capability.EmotionsAvailable() && !s.playing.preview && s.playing.prepared != nil {
				p := s.playing.prepared.Performance
				if p != nil && p.Intent != nil && rendercontract.NaturalPerformance(p.Intent.Version) {
					if s.settings.EmotionRegulation.Normalized().Enabled || block.controller.Controlled(s.turn) {
						block.decisions = append(block.decisions, block.controller.Propose(block.clock, p.Intent.Emotion, s.turn, s.playing.phraseIndex, s.generation))
					} else if s.settings.AutomaticExpressions && !s.settings.ReducedMotion && s.emotionSuppressedTurn != s.turn && p.Intent.Emotion != nil && p.Intent.Emotion.Valid() {
						block.activations = append(block.activations, rendercontract.EmotionActivation{TurnID: s.turn, PhraseIndex: s.playing.phraseIndex, Generation: s.generation, Emotion: *p.Intent.Emotion, Baseline: s.settings.EmotionBaseline.Normalized()})
					}
				}
			}
		}
		if filled == 0 {
			s.presentation = s.clipState(s.playing, s.position)
		}
		count := copy(pcm[filled:], s.playing.samples[s.position:])
		s.position += count
		filled += count
		if s.position == len(s.playing.samples) {
			s.queuedBytes -= len(s.playing.samples) * 2
			s.playing = nil
			s.position = 0
			s.revision++
		}
	}
	return block
}

func (s *session) clipState(value *clip, position int) SpeechState {
	return SpeechState{LaughterSpan: value.laughterSpan, Preview: value.preview, Active: true, Revision: s.revision, TurnID: s.turn, Position: time.Duration(position) * time.Second / sampleRate, Duration: value.duration, Samples: value.samples, SampleRate: sampleRate, Visemes: value.visemes, Text: value.text, Performance: value.performance, Prepared: value.prepared}
}
func (s *session) rememberTurn() {
	if s.turn == "" {
		return
	}
	s.ignored[s.turn] = true
	if len(s.ignored) > 128 {
		s.ignored = map[string]bool{s.turn: true}
	}
}

func (s *session) tickElapsedLocked() time.Duration {
	if s.lastRawTick != 0 {
		return min(max(0, nativeclock.Now()-s.lastRawTick), 20*time.Millisecond)
	}
	return min(time.Since(s.lastTick), 20*time.Millisecond)
}

func (s *session) preview(raw json.RawMessage) error {
	var p struct {
		Kind        string          `json:"kind"`
		Repetitions *int            `json:"repetitions"`
		Amount      *float64        `json:"amount"`
		TurnID      string          `json:"turn_id"`
		Speech      *preparedSpeech `json:"speech"`
	}
	if json.Unmarshal(raw, &p) != nil || !nativeanimation.ValidPreview(p.Kind) {
		return errors.New("invalid expression preview")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.playing != nil || s.preparing != nil || len(s.queue) > 0 || s.presentation.Active {
		return errors.New("expression preview requires idle playback")
	}
	if p.Speech != nil {
		if p.Kind != "laugh" || p.TurnID == "" || len(p.TurnID) > 128 {
			return errors.New("invalid laughter preview identity")
		}
		p.Speech.Preview = true
		decoded, err := decodeClip(*p.Speech, "Cached laughter preview")
		if err != nil || decoded.duration > 2500*time.Millisecond {
			return errors.New("invalid bounded laughter preview")
		}
		s.previewKind = ""
		s.generation++
		s.turn = p.TurnID
		s.previewEnd = ""
		s.previewTurn = ""
		if value, ok := s.scene.(interface{ UnifiedLaughterAvailable() bool }); ok && value.UnifiedLaughterAvailable() {
			s.previewTurn = p.TurnID
		}
		s.queue = []*clip{decoded}
		s.queuedBytes = len(decoded.samples) * 2
		s.nextIndex = 1
		s.complete = true
		s.revision++
		_, decoded.needsPreparation = s.scene.(SpeechPreparer)
		s.notifyPreparationLocked()
		return nil
	}
	amount := 1.0
	if p.Amount != nil {
		amount = *p.Amount
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > 1 {
			return errors.New("invalid preview intensity")
		}
	}
	repeats := rendercontract.GestureRepetitions(p.Kind, p.Repetitions)
	if repeats == 0 {
		return errors.New("invalid preview repetitions")
	}
	s.previewRepetitions = repeats
	s.previewAmount = amount
	s.previewKind = p.Kind
	s.previewSequence++
	s.revision++
	return nil
}

// PCM completion and visual completion share the existing notification contract.
// A preview's final notification belongs to the first successfully sent settled frame.
func (s *session) deferPreviewEnd(n playbackNotification, generation uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.kind != "playback_ended" || n.turn != s.previewTurn || generation != s.generation {
		return false
	}
	s.previewEnd = n.turn
	return true
}
func (s *session) finishPreviewPresentation(generation uint64, presenting bool) {
	s.mu.Lock()
	turn := ""
	if !s.paused && !presenting && generation == s.generation && s.previewEnd != "" {
		turn = s.previewEnd
		s.previewEnd = ""
		s.previewTurn = ""
	}
	s.mu.Unlock()
	if turn != "" {
		s.host.emit(event("playback", s.id, turn, map[string]string{"event": "playback_ended", "turn_id": turn}))
	}
}
