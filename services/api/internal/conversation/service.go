package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"syntelle/api/internal/httpapi"
	"syntelle/render-contract"
)

type Speech interface {
	Prepare(context.Context, string, string, string, string) (any, error)
}
type RecognitionEvent struct {
	Type, ID, Text       string
	OffsetMS, DurationMS float64
	StreamID             string          `json:"-"`
	Raw                  json.RawMessage `json:"raw,omitempty"`
}
type Recognition interface {
	Run(context.Context, string, <-chan []byte, func(RecognitionEvent)) error
}
type Event struct {
	EmotionDecision *rendercontract.EmotionDecision `json:"-"`

	EmotionActivation *rendercontract.EmotionActivation `json:"-"`
	Type              string                            `json:"type"`
	ConversationID    string                            `json:"conversation_id"`
	TurnID            string                            `json:"turn_id"`
	Character         string                            `json:"character,omitempty"`
	Language          string                            `json:"language,omitempty"`
	Voice             string                            `json:"voice,omitempty"`
	Instructions      string                            `json:"instructions,omitempty"`
	Text              string                            `json:"text,omitempty"`
	Reason            string                            `json:"reason,omitempty"`
	Audio             []byte                            `json:"audio_base64,omitempty"`
	Data              any                               `json:"data,omitempty"`
	Baseline          *ExpressionBaseline               `json:"expression_baseline,omitempty"`
}
type Service struct {
	Store       *Store
	Model       Model
	Speech      Speech
	Recognition Recognition
	mu          sync.Mutex
	connections map[*websocket.Conn]context.CancelFunc
	channels    map[*Channel]context.CancelFunc
	leases      map[string]chan struct{}
	closing     bool
	workers     sync.WaitGroup
	Performance *PerformanceProfile
	laughPolicy map[string]laughterPolicy
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

func New(store *Store, model Model, speech Speech, recognition Recognition) *Service {
	return &Service{Store: store, Model: model, Speech: speech, Recognition: recognition, connections: map[*websocket.Conn]context.CancelFunc{}, channels: map[*Channel]context.CancelFunc{}, leases: map[string]chan struct{}{}}
}
func (s *Service) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/conversation/status":
			w.Header().Set("Content-Type", "application/json")
			var settings any
			if s.Model != nil {
				settings = s.Model.Settings()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"configured": s.Model != nil && s.Speech != nil, "recognition": s.Recognition != nil, "policy": "local-owner-uncapped", "missing_model": s.Model == nil, "model": settings})
			return
		case "/api/v1/conversation/ws":
			if r.Method != http.MethodGet || !httpapi.AllowedOrigin(r) {
				http.Error(w, "Local owner origin required.", 403)
				return
			}
			s.socket(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type sessionContextKey struct{}

func IDs(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(sessionContextKey{}).(string)
	return id, ok
}
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closing = true
	for _, cancel := range s.channels {
		cancel()
	}
	for c, cancel := range s.connections {
		cancel()
		_ = c.CloseNow()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *Service) socket(w http.ResponseWriter, r *http.Request) {
	c, e := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"localhost:5173", "127.0.0.1:5173"}})
	if e != nil {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer c.CloseNow()
	c.SetReadLimit(64 << 10)
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	s.connections[c] = cancel
	s.workers.Add(1)
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.connections, c); s.mu.Unlock(); s.workers.Done() }()
	out := make(chan Event, 64)
	incoming := make(chan Event, 16)
	send := func(producer context.Context, event Event) {
		if err := enqueueOutput(producer, out, event, outputTimeout); errors.Is(err, errOutputCongested) {
			cancel()
		}
	}
	go func() {
		defer cancel()
		for {
			select {
			case event := <-out:
				b, _ := json.Marshal(event)
				writeCtx, stop := context.WithTimeout(ctx, outputTimeout)
				err := c.Write(writeCtx, websocket.MessageText, b)
				stop()
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		defer cancel()
		for {
			kind, b, err := c.Read(ctx)
			if err != nil {
				return
			}
			var event Event
			if kind != websocket.MessageText || json.Unmarshal(b, &event) != nil {
				return
			}
			select {
			case incoming <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	s.runSession(ctx, incoming, send)
}

// runSession is transport independent: legacy sockets and private render sessions
// share recognition, echo handling, cancellation and progressive speech.
func (s *Service) runSession(ctx context.Context, incoming <-chan Event, send func(context.Context, Event)) {
	var cid, language, voice, instructions string
	baseline := ExpressionBaseline{}
	var turnID string
	var turnCancel context.CancelFunc
	var turnDone chan struct{}
	var micID string
	var micCancel context.CancelFunc
	var micDone chan struct{}
	var pcm chan []byte
	recognized := make(chan RecognitionEvent, 32)
	seen := map[string]bool{}
	seenOrder := []string{}
	playing := false
	completedPlayback := false
	microphoneMS := 0.
	activityMS := -10000.
	windows := []playbackWindow{}
	gate := false
	utterance := recognitionUtterance{}
	settle := time.NewTimer(utterancePause)
	settle.Stop()
	defer settle.Stop()
	var settled <-chan time.Time
	maximumHold := time.NewTimer(utteranceMaximumHold)
	maximumHold.Stop()
	defer maximumHold.Stop()
	var held <-chan time.Time
	pauseUtterance := func() {
		settle.Stop()
		settled = nil
	}
	clearUtterance := func() {
		pauseUtterance()
		maximumHold.Stop()
		held = nil
	}
	armUtterance := func() {
		if utterance.id != "" {
			settle.Stop()
			settle.Reset(utterancePause)
			settled = settle.C
		}
	}
	holdUtterance := func() {
		maximumHold.Stop()
		maximumHold.Reset(utteranceMaximumHold)
		held = maximumHold.C
	}
	refreshUtteranceHold := func() {
		if utterance.id != "" {
			holdUtterance()
		}
	}
	emit := func(kind, id, text string, data any) {
		send(ctx, Event{Type: kind, ConversationID: cid, TurnID: id, Text: text, Data: data})
	}
	stopTurn := func(reason string) {
		if playing && len(windows) > 0 && windows[len(windows)-1].end < 0 {
			windows[len(windows)-1].end = microphoneMS
		}
		if turnCancel != nil {
			turnCancel()
			turnCancel = nil
		}
		if turnID != "" && !completedPlayback {
			emit("cancelled", turnID, "", nil)
			archiveCtx, stop := context.WithTimeout(context.Background(), time.Second)
			err := s.Store.Interrupt(archiveCtx, cid, turnID, reason)
			stop()
			if err != nil {
				classification := "database"
				if errors.Is(err, context.DeadlineExceeded) {
					classification = "deadline"
				} else if errors.Is(err, context.Canceled) {
					classification = "cancelled"
				}
				slog.Error("Interruption evidence could not be committed", "code", "interruption_archive_failed", "conversation_id", cid, "turn_id", turnID, "failure_class", classification)
				emit("persistence_warning", turnID, "Playback stopped, but this change could not be saved.", nil)
			}
		}
		playing = false
	}
	stopMic := func() {
		clearUtterance()
		utterance.take()
		if micCancel != nil {
			micCancel()
			micCancel = nil
		}
		pcm = nil
		micID = ""
	}
	defer func() {
		stopTurn("disconnect")
		stopMic()
		if turnDone != nil {
			<-turnDone
		}
		if micDone != nil {
			<-micDone
		}
		if cid != "" {
			s.mu.Lock()
			close(s.leases[cid])
			delete(s.leases, cid)
			s.mu.Unlock()
		}
	}()
	start := func(id, text string, literal bool) {
		if !identifier.MatchString(id) || strings.TrimSpace(text) == "" {
			emit("error", id, "Enter a message and try again.", nil)
			return
		}
		if s.Speech == nil || !literal && s.Model == nil {
			emit("error", id, "Configure the selected conversation model and Azure Speech on the local server.", nil)
			return
		}
		// Deduplicate BEFORE cancelling an active turn. Repeated submissions must
		// neither interrupt nor make another provider request.
		fresh, err := s.Store.BeginTurn(ctx, cid, id, text)
		if err != nil {
			emit("error", id, "The turn could not be saved.", nil)
			return
		}
		if !fresh {
			emit("duplicate", id, "This turn was already submitted.", nil)
			return
		}
		stopTurn("superseded")
		if turnDone != nil {
			<-turnDone
		}
		turnID = id
		completedPlayback = false
		playing = false
		gate = false
		work, stop := context.WithTimeout(ctx, 5*time.Minute)
		turnCancel = stop
		turnDone = make(chan struct{})
		done := turnDone
		emit("accepted", id, text, nil)
		performanceInstructions := instructions + s.directionContext(baseline, voice, language)
		go func() {
			defer close(done)
			defer stop()
			if literal {
				s.runLiteralTurn(work, cid, id, text, language, voice, func(event Event) { send(work, event) })
			} else {
				s.runTurn(work, cid, id, text, language, voice, performanceInstructions, func(event Event) { send(work, event) })
			}
		}()
	}
	flushUtterance := func() {
		clearUtterance()
		id, text := utterance.take()
		if micID != "" && id != "" {
			emit("transcript_final", id, text, nil)
			start(id, text, false)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-settled:
			// A continuation already queued by the SDK wins over the deadline.
			if len(recognized) > 0 {
				armUtterance()
				continue
			}
			flushUtterance()
		case <-held:
			// A queued SDK callback wins over the fallback deadline. Refresh the
			// watchdog so an active continuation is not split by timer selection.
			if len(recognized) > 0 {
				refreshUtteranceHold()
				continue
			}
			flushUtterance()
		case event, open := <-incoming:
			if !open {
				return
			}
			if event.Type == "hello" {
				if cid != "" || !identifier.MatchString(event.ConversationID) || !identifier.MatchString(event.TurnID) || (event.Character != "syntelle" && event.Character != "cafe" && event.Character != "woodland") || (event.Language != "en-US" && event.Language != "es-CO") || len([]rune(event.Instructions)) > 2000 || (event.Voice != "en-US-Ava:DragonHDLatestNeural" && event.Voice != "en-US-Andrew:DragonHDLatestNeural") {
					return
				}
				s.mu.Lock()
				previous := s.leases[event.ConversationID]
				s.mu.Unlock()
				if previous != nil {
					select {
					case <-previous:
					case <-time.After(3 * time.Second):
						send(ctx, Event{Type: "error", ConversationID: event.ConversationID, TurnID: event.TurnID, Text: "This conversation is open in another connection."})
						return
					case <-ctx.Done():
						return
					}
				}
				s.mu.Lock()
				if s.leases[event.ConversationID] != nil {
					s.mu.Unlock()
					return
				}
				s.leases[event.ConversationID] = make(chan struct{})
				s.mu.Unlock()
				cid = event.ConversationID
				language = event.Language
				voice = event.Voice
				instructions = event.Instructions
				turns, err := s.Store.Session(ctx, cid, event.Character, language, instructions)
				if err != nil {
					emit("error", event.TurnID, "Conversation could not be restored. Start a new chat.", nil)
					return
				}
				emit("ready", event.TurnID, "", turns)
				continue
			}
			if event.Type == "emotion_decision" && cid != "" && event.ConversationID == cid && event.EmotionDecision != nil {
				d := event.EmotionDecision
				if d.Version != rendercontract.EmotionPolicyVersion || !d.Effective.Valid() || d.Baseline.Normalized() != baseline.Emotion.Normalized() || d.Regulation.Normalized() != baseline.Regulation.Normalized() {
					continue
				}
				key := fmt.Sprintf("%s-emotion-policy-%s-%d-%d-%d", cid, d.Instance, d.Generation, d.PolicyRevision, d.Sequence)
				op, err := s.Store.Reserve(ctx, "emotion-decision", key, rendercontract.EmotionPolicyVersion, cid, d.TurnID, d, false)
				if err != nil {
					continue
				}
				if err = s.Store.Finish(ctx, op.ID, "received", d, nil, map[string]string{"source": "private-pcm-policy"}, nil); err != nil {
					continue
				}
				// Rejections are evidence, never new experienced context.
				if d.Reason == "accepted" || d.Reason == "sustained" || d.Reason == "intensity_limited" || d.Reason == "clock_transition" || d.Reason == "configuration" || d.Reason == "reply_ended" {
					value := d.Effective
					baseline.LastEmotion = &value
					baseline.EmotionPhase = d.Phase
				}
				continue
			}
			if cid == "" || event.ConversationID != cid || !identifier.MatchString(event.TurnID) {
				continue
			}
			switch event.Type {
			case "emotion_activated":
				if event.TurnID != turnID || event.EmotionActivation == nil || !event.EmotionActivation.Emotion.Valid() || event.EmotionActivation.PhraseIndex < 0 || event.EmotionActivation.PhraseIndex >= 32 || event.EmotionActivation.Baseline.Normalized() != baseline.Emotion.Normalized() {
					continue
				}
				activated := event.EmotionActivation
				key := fmt.Sprintf("%s-emotion-%d-%d", event.TurnID, activated.PhraseIndex, activated.Generation)
				op, err := s.Store.Reserve(ctx, "emotion-activation", key, "performance-intent/3", cid, event.TurnID, activated, false)
				if err != nil {
					emit("persistence_warning", event.TurnID, "Emotional performance state could not be saved.", nil)
					continue
				}
				if err = s.Store.Finish(ctx, op.ID, "received", activated, nil, map[string]string{"source": "private-pcm-activation"}, nil); err != nil {
					continue
				}
				value := activated.Emotion
				baseline.LastEmotion = &value
				baseline.EmotionPhase = ""
			case "expression_baseline":
				if event.Baseline != nil && event.Baseline.valid() {
					baseline = updateExpressionBaseline(baseline, *event.Baseline)
				}
			case "configure":
				if len([]rune(event.Instructions)) > 2000 || (event.Voice != "en-US-Ava:DragonHDLatestNeural" && event.Voice != "en-US-Andrew:DragonHDLatestNeural") {
					continue
				}
				if turnDone != nil {
					select {
					case <-turnDone:
					default:
						emit("error", event.TurnID, "Stop the active reply before changing instructions.", nil)
						continue
					}
				}
				instructions = event.Instructions
				baseline.LastEmotion = nil
				voice = event.Voice
				if err := s.Store.Instructions(ctx, cid, instructions); err != nil {
					emit("error", event.TurnID, "Instructions could not be saved.", nil)
					return
				}
			case "submit", "speak":
				if event.Baseline != nil && event.Baseline.valid() {
					baseline = updateExpressionBaseline(baseline, *event.Baseline)
				}
				clearUtterance()
				utterance.take()
				start(event.TurnID, event.Text, event.Type == "speak")
			case "cancel":
				if event.TurnID == turnID {
					stopTurn(event.Reason)
				}
			case "playback_started":
				if event.TurnID == turnID {
					playing = true
					completedPlayback = false
					gate = false
					windows = append(windows, playbackWindow{microphoneMS, -1})
					if len(windows) > 64 {
						windows = windows[len(windows)-64:]
					}
				}
			case "playback_ended", "playback_waiting":
				if event.TurnID == turnID {
					if len(windows) > 0 && windows[len(windows)-1].end < 0 {
						windows[len(windows)-1].end = microphoneMS
					}
					playing = false
					completedPlayback = event.Type == "playback_ended"
				}
			case "activity":
				if event.TurnID == turnID && playing {
					gate = true
					activityMS = microphoneMS
				}
			case "mic_stop":
				if event.TurnID == micID {
					stopMic()
				}
			case "mic_start":
				if pcm != nil || s.Recognition == nil {
					emit("error", event.TurnID, "Recognition is unavailable or already active.", nil)
					continue
				}
				if micDone != nil {
					<-micDone
				}
				micID = event.TurnID
				microphoneMS = 0
				activityMS = -10000
				windows = nil
				if playing {
					windows = append(windows, playbackWindow{0, -1})
				}
				micCtx, stop := context.WithCancel(ctx)
				micCancel = stop
				pcm = make(chan []byte, 20)
				stream := pcm
				streamID := micID
				micDone = make(chan struct{})
				done := micDone
				go func() {
					defer close(done)
					s.RunRecognition(micCtx, cid, streamID, language, stream, func(v RecognitionEvent) {
						v.StreamID = streamID
						select {
						case recognized <- v:
						case <-micCtx.Done():
						default:
							stop()
						}
					})
					stop()
				}()
				emit("listening", micID, "", nil)
			case "audio":
				if event.TurnID == micID && pcm != nil {
					if len(event.Audio) == 0 || len(event.Audio) > 6400 || len(event.Audio)%2 != 0 {
						return
					}
					microphoneMS += float64(len(event.Audio)) / 32
					select {
					case pcm <- event.Audio:
					default:
						stopMic()
						emit("error", event.TurnID, "Microphone stream fell behind; enable it again.", nil)
					}
				}
			}
		case event := <-recognized:
			if micID == "" || event.StreamID != micID {
				if event.Type == "final" {
					slog.Warn("Confirmed recognition did not match the active microphone", "code", "recognition_final_inactive_microphone", "conversation_id", cid, "result_id", event.ID)
				}
				continue
			}
			switch event.Type {
			case "partial":
				if strings.TrimSpace(event.Text) != "" {
					pauseUtterance()
					refreshUtteranceHold()
				}
				emit("transcript_partial", micID, utterance.preview(event.Text), nil)
			case "start":
				pauseUtterance()
				refreshUtteranceHold()
				emit("speech_detected", micID, "", nil)
			case "end":
				// A no-match result resolves an interim segment without inventing text.
				emit("transcript_partial", micID, utterance.text, nil)
				armUtterance()
			case "final":
				if !identifier.MatchString(event.ID) || seen[event.ID] {
					slog.Warn("Confirmed recognition was invalid or already seen", "code", "recognition_final_duplicate_or_invalid", "conversation_id", cid, "result_id", event.ID)
					continue
				}
				seen[event.ID] = true
				seenOrder = append(seenOrder, event.ID)
				if len(seenOrder) > 256 {
					delete(seen, seenOrder[0])
					seenOrder = seenOrder[1:]
				}
				// Timing overlap is only a reason to suppress speech while the
				// speaker is active. A delayed SDK result after playback has ended
				// may retain an overlapping offset but must remain eligible as a
				// follow-up turn.
				if playing && (!gate || overlapsPlayback(event, windows, microphoneMS, activityMS)) {
					slog.Info("Confirmed recognition suppressed during playback", "code", "recognition_final_playback", "conversation_id", cid, "result_id", event.ID)
					emit("transcript_partial", micID, utterance.text, nil)
					armUtterance()
					continue
				}
				if len(windows) > 0 {
					spoken, err := s.Store.PlaybackText(ctx, cid, turnID)
					if err != nil {
						emit("microphone_error", micID, "The interruption could not be checked. Use Stop reply before asking again.", nil)
						continue
					}
					if matchesPlayback(event.Text, spoken) {
						slog.Info("Confirmed recognition matched playback text", "code", "recognition_final_echo", "conversation_id", cid, "result_id", event.ID)
						continue
					}
				}
				utterance.append(event.ID, event.Text)
				slog.Info("Confirmed recognition queued for a turn", "code", "recognition_final_queued", "conversation_id", cid, "result_id", event.ID)
				emit("transcript_partial", micID, utterance.text, nil)
				holdUtterance()
				armUtterance()
			case "error":
				emit("microphone_error", micID, event.Text, nil)
				stopMic()
			}
		}
	}
}
func (s *Service) runTurn(ctx context.Context, cid, id, text, language, voice, instructions string, send func(Event)) {
	ctx = context.WithValue(ctx, sessionContextKey{}, cid)
	if director, ok := s.Model.(phraseDirector); ok && director.PhraseDirection() {
		s.runPhraseTurn(ctx, director, cid, id, language, voice, instructions, send)
		return
	}
	started := time.Now()
	output, spoken := "", ""
	status := "failed"
	reason := ""
	first := int64(-1)
	openingAt := int64(-1)
	var pipeline *speechPipeline
	var opening *openingJob
	defer func() {
		if opening != nil {
			opening.stop()
		}
		if pipeline != nil {
			pipeline.stop()
		}
	}()
	emit := func(kind, text string, data any) {
		send(Event{Type: kind, ConversationID: cid, TurnID: id, Text: text, Data: data})
	}
	defer func() {
		if status == "ready" {
			return
		}
		if ctx.Err() != nil {
			status = "interrupted"
			reason = "cancelled"
		}
		archive, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if e := s.Store.FinishTurn(archive, id, status, output, spoken, reason); e != nil {
			emit("error", "Conversation result could not be committed.", nil)
		}
	}()
	turns, e := s.Store.Turns(ctx, cid, 21)
	if e != nil {
		emit("error", "Conversation context could not be loaded.", nil)
		return
	}
	messages := ContextMessages(turns)
	request := map[string]any{"instructions": DefaultInstructions + "\n" + instructions + "\nSelected language: " + language, "input": messages, "settings": s.Model.Settings(), "language": language}
	var providerRequest any = request
	if builder, ok := s.Model.(interface{ Request(string, []Message) any }); ok {
		providerRequest = builder.Request(instructions+"\nSelected language: "+language, messages)
	}
	kind, adapter := "luna", "go-http/responses-v1"
	if provider, ok := s.Model.(interface{ Identity() (string, string) }); ok {
		kind, adapter = provider.Identity()
	}
	op, e := s.Store.Reserve(ctx, kind, id, "", cid, id, map[string]any{"body": providerRequest, "settings": s.Model.Settings()}, false)
	if e != nil {
		emit("error", "Model request could not be committed.", nil)
		return
	}
	// The opening is deliberately allowed before whole-envelope completion.
	// Persist its exact text/audio binding before release; failures stop the
	// remaining reply and cannot retract audio that has already been heard.
	publishPart := func(part SpeechPart, modelPending bool) error {
		evidence, err := speechDeliveryEvidence(part)
		if err != nil {
			return err
		}
		partOp, err := s.Store.Reserve(ctx, "speech-delivery", id+"-part-"+fmt.Sprint(part.Index), "progressive-speech/3", cid, id, map[string]any{"index": part.Index, "text": part.Text, "offset_ms": part.OffsetMS, "maximum_concurrent_synthesis": 2}, false)
		if err != nil {
			return err
		}
		if err = s.Store.Finish(ctx, partOp.ID, "received", evidence, nil, map[string]any{"ready_ms": time.Since(started).Milliseconds(), "model_pending": modelPending, "model_operation_id": op.ID}, nil); err != nil {
			return err
		}
		emit("speech_part", part.Text, part)
		return nil
	}
	onDelta := func(delta string) {
		if ctx.Err() != nil {
			return
		}
		if first < 0 {
			first = time.Since(started).Milliseconds()
		}
		emit("reply_delta", delta, nil)
		if early, ok := s.Model.(interface{ EarlySpeechText() bool }); ok && early.EarlySpeechText() && pipeline == nil {
			if speech, ok := s.Speech.(progressiveSpeech); ok {
				pipeline = startSpeechPipeline(ctx, speech, SpokenText(delta), language, voice, id, opening)
			}
		}
	}
	var result ModelResult
	if early, ok := s.Model.(interface {
		CompleteWithOpening(context.Context, string, []Message, func(string), func(string)) (ModelResult, error)
	}); ok {
		result, e = early.CompleteWithOpening(ctx, instructions+"\nSelected language: "+language, messages, onDelta, func(text string) {
			if speech, ok := s.Speech.(openingSpeech); ok && opening == nil {
				openingAt = time.Since(started).Milliseconds()
				opening = startOpeningSpeech(ctx, speech, text, language, voice, id, func(part SpeechPart) error { return publishPart(part, true) })
			}
		})
	} else {
		result, e = s.Model.Complete(ctx, instructions+"\nSelected language: "+language, messages, onDelta)
	}
	if e != nil && opening != nil {
		opening.cancel()
	}
	if e != nil && pipeline != nil {
		pipeline.cancel()
	}
	output = result.Text
	opStatus := "received"
	if e != nil {
		opStatus = "failed"
		if ctx.Err() != nil {
			opStatus = "cancelled"
		}
	}
	diagnostics := map[string]any{"first_delta_ms": first, "opening_prefetch_ms": openingAt, "elapsed_ms": time.Since(started).Milliseconds(), "sdk": adapter, "outcome": opStatus}
	if ctx.Err() != nil {
		diagnostics["billing_outcome"] = "unknown"
	}
	if archiveErr := s.Store.Finish(ctx, op.ID, opStatus, result, result.Usage, diagnostics, nil); archiveErr != nil {
		emit("error", "Model evidence could not be committed.", nil)
		return
	}
	if e != nil {
		if ctx.Err() == nil {
			emit("error", e.Error(), nil)
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	spoken = SpokenText(output)
	emit("reply_complete", output, map[string]string{"spoken": spoken})
	emit("preparing", "", nil)
	if speech, ok := s.Speech.(progressiveSpeech); ok && pipeline == nil {
		pipeline = startSpeechPipeline(ctx, speech, spoken, language, voice, id, opening)
	}
	var prepared any
	partCount := 0
	if pipeline != nil {
		if pipeline.text != spoken {
			emit("error", "The completed reply differs from its prepared text; playback was stopped.", nil)
			return
		}
		// Save accepted model text before publishing any audio. The turn remains
		// preparing until every part is durable, so restart recovery stays valid.
		if e = s.Store.FinishTurn(ctx, id, "preparing", output, spoken, ""); e != nil {
			emit("error", "Reply could not be committed.", nil)
			return
		}
		lastHead := -3.0
		for part := range pipeline.parts {
			partCount = part.Index + 1
			if ctx.Err() != nil {
				return
			}
			if part.Index == 0 && opening != nil && opening.published {
				continue // The same cached native recording was already delivered.
			}
			part.Speech, e = partPerformance(result.Performance, part, &lastHead)
			if e != nil {
				emit("error", "Speech direction could not be prepared.", nil)
				return
			}
			// Persist the exact payload (including per-part direction) before it
			// becomes audible. Native audio is already durable in the speech ledger.
			if publishPart(part, false) != nil {
				emit("error", "Speech delivery evidence could not be committed.", nil)
				return
			}
		}
		<-pipeline.done
		prepared, e = pipeline.result, pipeline.err
	} else {
		prepared, e = s.Speech.Prepare(ctx, spoken, language, voice, id)
	}
	if e != nil {
		if ctx.Err() == nil {
			emit("error", e.Error(), nil)
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	if result.Performance != nil {
		// Preserve the original speech operation/audio. Direction gets its own
		// immutable evidence record and an additive prepared-payload field.
		data, marshalErr := json.Marshal(prepared)
		var audio struct {
			Audio    []byte  `json:"audio_base64"`
			Duration float64 `json:"duration_ms"`
		}
		if marshalErr != nil || json.Unmarshal(data, &audio) != nil {
			emit("error", "Direction evidence could not be prepared.", nil)
			return
		}
		validation := ValidatePerformance(result.Performance.Intent, spoken)
		validation.Omitted = append(validation.Omitted, result.Performance.Omitted...)
		validation = ResolvePerformance(validation, audio.Audio, audio.Duration/1000)
		directionOp, archiveErr := s.Store.Reserve(ctx, "performance", id, "performance-intent/1", cid, id, result.Performance, false)
		if archiveErr == nil {
			archiveErr = s.Store.Finish(ctx, directionOp.ID, "received", validation, nil, map[string]string{"timing": "approximate", "clock": "audio"}, nil)
		}
		if archiveErr != nil {
			emit("error", "Direction evidence could not be committed.", nil)
			return
		}
		var payload map[string]any
		if json.Unmarshal(data, &payload) != nil {
			emit("error", "Prepared speech could not be decoded.", nil)
			return
		}
		payload["performance"] = validation
		prepared = payload
	}
	// Commit turn before publishing prepared audio. Playback interruption is a
	// separate persisted update and cannot erase the original model output.
	if e = s.Store.FinishTurn(ctx, id, "ready", output, spoken, ""); e != nil {
		emit("error", "Reply could not be committed.", nil)
		return
	}
	status = "ready"
	if pipeline != nil {
		emit("speech_complete", spoken, map[string]int{"parts": partCount})
	} else {
		emit("prepared_speech", spoken, prepared)
	}
}
func (s *Service) RunRecognition(ctx context.Context, cid, id, language string, pcm <-chan []byte, emit func(RecognitionEvent)) {
	op, e := s.Store.Reserve(ctx, "recognition", id, "", cid, "", map[string]any{"language": language, "sample_rate": 16000, "bits": 16, "channels": 1, "segmentation_silence_ms": 700, "turn_settle_ms": utterancePause.Milliseconds(), "turn_maximum_idle_hold_ms": utteranceMaximumHold.Milliseconds(), "turn_assembly": "confirmed-segments/2", "sdk": "1.52.0"}, false)
	if e != nil {
		emit(RecognitionEvent{Type: "error", Text: "Recognition request could not be committed."})
		return
	}
	if op.Status != "started" {
		emit(RecognitionEvent{Type: "error", Text: "This recognition session was already used. Enable a new session."})
		return
	}
	var recognitionMu sync.Mutex
	segments := 0
	audioMS := 0.
	archiveFailed := false
	e = s.Recognition.Run(ctx, language, pcm, func(event RecognitionEvent) {
		recognitionMu.Lock()
		defer recognitionMu.Unlock()
		if event.Type == "final" {
			if err := s.Store.RecognitionResult(context.WithoutCancel(ctx), op.ID, event); err != nil {
				archiveFailed = true
				emit(RecognitionEvent{Type: "error", Text: "Recognition transcript could not be committed."})
				return
			}
			segments++
		}
		if event.Type == "usage" {
			audioMS = event.DurationMS
			return
		}
		emit(event)
	})
	status := "received"
	if e != nil {
		status = "failed"
		if ctx.Err() != nil {
			status = "cancelled"
		}
	}
	recognitionMu.Lock()
	defer recognitionMu.Unlock()
	if archiveFailed {
		status = "failed"
	}
	err := s.Store.Finish(ctx, op.ID, status, map[string]any{"results_table": "syntelle_conversation.recognition_results"}, map[string]any{"recognized_segments": segments, "audio_ms": audioMS}, map[string]string{"outcome": status}, nil)
	if err != nil {
		emit(RecognitionEvent{Type: "error", Text: "Recognition evidence could not be committed."})
	} else if e != nil && ctx.Err() == nil {
		emit(RecognitionEvent{Type: "error", Text: "Azure recognition stopped; enable the microphone again. No retry was made."})
	}
}
