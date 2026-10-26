package streaming

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"syntelle/render-contract"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
	"syntelle/api/internal/httpapi"
)

type Service struct {
	bootMu        sync.Mutex
	workerStarted time.Time
	mu            sync.Mutex
	ctx           context.Context // owned service lifecycle, never an HTTP request context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	closing       bool
	sessions      map[string]*streamSession
	worker        *peer
	workers       map[string]*cloudWorker
	store         *recordStore
	conversation  *conversation.Service
	resolve       ResolveFunc
	config        Config
	admission     *streamAdmission
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

func Open(parent context.Context, pool *pgxpool.Pool, conversations *conversation.Service, resolve ResolveFunc, config Config) (*Service, error) {
	if pool == nil || conversations == nil || resolve == nil {
		return nil, errors.New("streaming requires persistence, conversation service and authorization")
	}
	admission, err := newStreamAdmission(config.Admission)
	if err != nil {
		return nil, err
	}
	store, err := openStore(parent, pool)
	if err != nil {
		return nil, errors.New("stream persistence initialization failed")
	}
	if config.MaximumSessions <= 0 {
		config.MaximumSessions = 2
	}
	if config.PackageHash == "" {
		return nil, errors.New("a source-bound Syntelle package hash is required")
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Service{ctx: ctx, cancel: cancel, sessions: map[string]*streamSession{}, workers: map[string]*cloudWorker{}, store: store, conversation: conversations, resolve: resolve, config: config, admission: admission}
	s.purgeOldHistory(ctx)
	s.wg.Add(1)
	go s.expire()
	return s, nil
}

func (s *Service) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/conversations" || strings.HasPrefix(r.URL.Path, "/api/v1/conversations/") {
			s.history(w, r)
			return
		}
		if r.URL.Path == "/api/v1/stream-worker/ws" {
			if !localRequest(r) {
				http.Error(w, "Local service required.", 403)
				return
			}
			s.workerSocket(w, r)
			return
		}
		if r.URL.Path != "/api/v1/streams" && !strings.HasPrefix(r.URL.Path, "/api/v1/streams/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if !localRequest(r) {
			writeError(w, 403, "Local preview required.")
			return
		}
		if !s.allowStreamRequest(w, r, "") {
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/streams"), "/"), "/")
		if len(parts) == 1 && parts[0] == "status" && r.Method == http.MethodGet {
			s.mu.Lock()
			available := (s.worker != nil || httpapi.Cloud()) && !s.closing
			count := len(s.sessions)
			s.mu.Unlock()
			writeJSON(w, 200, map[string]any{"available": available, "configured": s.conversation.Speech != nil && s.conversation.Model != nil, "speech_configured": s.conversation.Speech != nil, "recognition": s.conversation.Recognition != nil, "capacity": s.config.MaximumSessions, "active": count, "time_limits_enabled": !s.config.DisableTimeLimits})
			return
		}
		if len(parts) == 2 && parts[1] == "ws" && r.Method == http.MethodGet {
			s.playerSocket(w, r, parts[0])
			return
		}
		principal, err := s.resolve(w, r, true)
		if err != nil {
			writeError(w, 401, "Sign in or start a visitor session.")
			return
		}
		if !s.allowStreamRequest(w, r, principal.ID) {
			return
		}
		if r.URL.Path == "/api/v1/streams" && r.Method == http.MethodPost {
			s.create(w, r, principal)
			return
		}
		if len(parts) < 1 || !identifier.MatchString(parts[0]) {
			writeError(w, 404, "Stream not found.")
			return
		}
		s.mu.Lock()
		session := s.sessions[parts[0]]
		authorized := session != nil && session.principal.ID == principal.ID
		s.mu.Unlock()
		if !authorized {
			writeError(w, 404, "Stream not found.")
			return
		}
		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			s.mu.Lock()
			result := s.response(session, "")
			s.mu.Unlock()
			writeJSON(w, 200, result)
		case len(parts) == 1 && r.Method == http.MethodDelete:
			s.closeSession(session.id, "stopped")
			w.WriteHeader(204)
		case len(parts) == 2 && parts[1] == "ticket" && r.Method == http.MethodPost:
			s.mu.Lock()
			ticket, err := issueTicket(session)
			s.mu.Unlock()
			if err != nil {
				writeError(w, 500, "Connection ticket could not be created.")
				return
			}
			writeJSON(w, 200, map[string]string{"ticket": ticket})
		case len(parts) == 2 && parts[1] == "commands" && r.Method == http.MethodPost:
			var command Command
			if decode(w, r, &command) != nil {
				writeError(w, 400, "Invalid stream command.")
				return
			}
			result, err := s.command(r.Context(), session, command)
			if err != nil {
				writeError(w, 409, err.Error())
				return
			}
			writeJSON(w, 202, result)
		default:
			writeError(w, 405, "Unsupported stream operation.")
		}
	})
}

func localRequest(r *http.Request) bool {
	if httpapi.Cloud() {
		return httpapi.CloudRequest(r)
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return false
	}
	return r.Header.Get("Origin") == "" || httpapi.AllowedOrigin(r)
}
func (s *Service) response(session *streamSession, ticket string) SessionResponse {
	ice, _ := rendercontract.ICEServers()
	startupTimeout := 12000
	if httpapi.Cloud() {
		startupTimeout = 95000
	}
	return SessionResponse{StartupTimeoutMS: startupTimeout, ICEServers: ice, ID: session.id, Ticket: ticket, State: session.state, Settings: session.settings, Configured: s.conversation.Model != nil && s.conversation.Speech != nil, SpeechConfigured: s.conversation.Speech != nil, Recognition: s.conversation.Recognition != nil, RemainingSeconds: max(0, int(math.Ceil(time.Until(session.deadline).Seconds()))), TimeLimitsEnabled: !s.config.DisableTimeLimits}
}
func (s *Service) create(w http.ResponseWriter, r *http.Request, principal accounts.Principal) {
	input := struct {
		Settings               accounts.Settings `json:"settings"`
		ResumeFrom             string            `json:"resume_from,omitempty"`
		ContinueConversationID string            `json:"continue_conversation_id,omitempty"`
	}{Settings: accounts.Defaults()}
	if decode(w, r, &input) != nil {
		writeError(w, 400, "Invalid stream settings.")
		return
	}
	if input.ResumeFrom != "" && input.ContinueConversationID != "" {
		writeError(w, 400, "Choose one conversation to continue.")
		return
	}
	if input.ContinueConversationID != "" {
		if principal.Kind != "account" || !identifier.MatchString(input.ContinueConversationID) {
			writeError(w, 404, "Conversation not found.")
			return
		}
		stored, loadErr := s.store.continuation(r.Context(), input.ContinueConversationID, principal.AccountID)
		if loadErr != nil || json.Unmarshal(stored, &input.Settings) != nil {
			writeError(w, 404, "Conversation not found.")
			return
		}
		s.mu.Lock()
		active := false
		for _, existing := range s.sessions {
			if existing.cid == input.ContinueConversationID {
				active = true
				break
			}
		}
		s.mu.Unlock()
		if active {
			writeError(w, 409, "This conversation is already active.")
			return
		}
	}
	input.Settings.Voice = "en-US-Ava:DragonHDLatestNeural"
	if err := input.Settings.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	id, err := randomID("stream_")
	if err != nil {
		writeError(w, 500, "Stream identity could not be created.")
		return
	}
	cid, err := randomID("convo_")
	if err != nil {
		writeError(w, 500, "Stream identity could not be created.")
		return
	}
	if input.ContinueConversationID != "" {
		cid = input.ContinueConversationID
	}
	var hadMessages bool
	if input.ResumeFrom != "" {
		if !identifier.MatchString(input.ResumeFrom) {
			writeError(w, 404, "Paused stream not found.")
			return
		}
		var language string
		cid, language, hadMessages, err = s.store.paused(r.Context(), input.ResumeFrom, principal.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, 404, "Paused stream not found.")
			} else {
				writeError(w, 500, "Paused stream could not be restored. Try again.")
			}
			return
		}
		if language != input.Settings.Language {
			writeError(w, 409, "Start a new chat to change language after a pause.")
			return
		}
		id = input.ResumeFrom
	}
	ctx, cancel := context.WithCancel(s.ctx)
	session := &streamSession{emotionPolicyRevision: 1, id: id, cid: cid, principal: principal, settings: input.Settings, state: "starting", created: time.Now(), lastSeen: time.Now(), cancel: cancel, commandMu: make(chan struct{}, 1)}
	if hadMessages {
		session.lastMessage = session.created
	}
	ticket, err := issueTicket(session)
	if err != nil {
		cancel()
		writeError(w, 500, "Connection ticket could not be created.")
		return
	}
	if err = s.reserveSession(session); err != nil {
		cancel()
		if errors.Is(err, errPrincipalSessionLimit) {
			limited(w, 5, err.Error())
		} else {
			writeError(w, 503, err.Error())
		}
		return
	}
	var deadline time.Time
	if s.config.DisableTimeLimits {
		err = s.store.allocateUnmetered(r.Context(), session, s.config.PackageHash, input.ResumeFrom != "")
	} else {
		deadline, err = s.store.allocate(r.Context(), session, s.config.PackageHash, input.ResumeFrom != "")
	}
	if err != nil {
		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()
		cancel()
		if errors.Is(err, errDemoTime) || errors.Is(err, errSessionTime) {
			writeError(w, 429, err.Error())
		} else {
			writeError(w, 500, "Stream could not be saved.")
		}
		return
	}
	if !deadline.IsZero() {
		leaseCtx, leaseCancel := context.WithDeadline(ctx, deadline)
		s.mu.Lock()
		session.deadline = deadline
		session.cancel = func() { leaseCancel(); cancel() }
		s.mu.Unlock()
		ctx = leaseCtx
	}
	channel, err := s.conversation.OpenChannel(ctx, func(event conversation.Event) { s.conversationEvent(session, event) })
	if err != nil {
		s.closeSession(id, "conversation unavailable")
		writeError(w, 503, "Conversation service is unavailable.")
		return
	}
	s.mu.Lock()
	session.channel = channel
	s.mu.Unlock()
	helloID, _ := randomID("hello_")
	err = channel.Send(ctx, conversation.Event{Type: "hello", ConversationID: cid, TurnID: helloID, Character: "syntelle", Language: input.Settings.Language, Voice: input.Settings.Voice, Instructions: input.Settings.Instructions})
	if err != nil {
		s.closeSession(id, "conversation unavailable")
		writeError(w, 503, "Conversation service is unavailable.")
		return
	}
	if httpapi.Cloud() {
		s.mu.Lock()
		if s.closing || s.sessions[id] != session {
			s.mu.Unlock()
			writeError(w, 503, "Stream closed.")
			return
		}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.bootSession(ctx, session)
	} else if !s.deliverCreate(session) {
		s.closeSession(id, "renderer unavailable")
		writeError(w, 503, "The private renderer disconnected.")
		return
	}
	s.mu.Lock()
	result := s.response(session, ticket)
	s.mu.Unlock()
	writeJSON(w, 201, result)
}

func (s *Service) command(ctx context.Context, session *streamSession, c Command) (commandResult, error) {
	if c.Settings != nil {
		next := *c.Settings
		next.Voice = "en-US-Ava:DragonHDLatestNeural"
		c.Settings = &next
	}
	result := commandResult{ID: c.ID}
	if s.config.ValidatePrincipal != nil {
		if err := s.config.ValidatePrincipal(ctx, session.principal); err != nil {
			s.closeSession(session.id, "authorization expired")
			return result, errors.New("Stream authorization expired. Sign in or connect again.")
		}
	}
	if !identifier.MatchString(c.ID) {
		return result, errors.New("A unique command identifier is required.")
	}
	if c.Type != "message" && c.Type != "speak" && c.Type != "interrupt" && c.Type != "configure" && c.Type != "preview" {
		return result, errors.New("Unknown command type.")
	}
	if (c.Type == "message" || c.Type == "speak") && (strings.TrimSpace(c.Text) == "" || len([]rune(c.Text)) > 65536) {
		return result, errors.New("Enter nonempty text within the per-message safety bound.")
	}
	if c.Type == "configure" && (c.Settings == nil || c.Settings.Validate() != nil) {
		return result, errors.New("Invalid Syntelle settings.")
	}
	if c.Type == "preview" && (rendercontract.GestureRepetitions(c.Text, c.Repetitions) == 0 || !validPreview(c.Text) || c.Amount != nil && (*c.Amount < 0 || *c.Amount > 1 || math.IsNaN(*c.Amount) || math.IsInf(*c.Amount, 0))) {
		return result, errors.New("Unknown expression preview.")
	}
	select {
	case session.commandMu <- struct{}{}:
		defer func() { <-session.commandMu }()
	case <-ctx.Done():
		return result, ctx.Err()
	}
	s.mu.Lock()
	active := s.sessions[session.id] == session
	state := session.state
	channel := session.channel
	settings := session.settings
	previous := session.currentTurn
	if previous == "" {
		previous = session.pendingTurn
	}
	s.mu.Unlock()
	if !active || channel == nil {
		return result, errors.New("This stream has ended.")
	}
	if state == "starting" {
		return result, errors.New("Wait for the renderer to become ready.")
	}
	if c.Type == "preview" && state != "idle" {
		return result, errors.New("Wait until the reply ends before previewing.")
	}
	modelChange := c.Settings != nil && (settings.Instructions != c.Settings.Instructions || settings.Voice != c.Settings.Voice)
	if c.Type == "configure" {
		if settings.Language != c.Settings.Language {
			return result, errors.New("Start a new stream to change recognition language.")
		}
		if modelChange && previous != "" {
			return result, errors.New("Stop the active reply before changing instructions or voice.")
		}
	}
	hash := sha256.Sum256([]byte(session.id + ":" + c.ID))
	turnID := "turn_" + hex.EncodeToString(hash[:16])
	result.TurnID = turnID
	fresh, err := s.store.command(ctx, session.id, c, turnID)
	if err != nil {
		return result, errors.New("Command could not be saved or its identifier has different content.")
	}
	if !fresh {
		result.Duplicate = true
		return result, nil
	}
	s.mu.Lock()
	if c.Type == "message" || c.Type == "speak" {
		if s.sessions[session.id] == session {
			session.lastMessage = time.Now()
		}
		ignoreTurn(session, previous)
		session.pendingTurn = turnID
		session.currentTurn = ""
	}
	s.mu.Unlock()
	switch c.Type {
	case "message", "speak":
		if previous != "" {
			_ = s.store.finish(ctx, session.id, previous, "interrupted")
		}
		s.toWorker(envelope("interrupt", session.id, map[string]string{"reason": "superseded"}))
		kind := "submit"
		if c.Type == "speak" {
			kind = "speak"
		}
		err = channel.Send(ctx, conversation.Event{Type: kind, ConversationID: session.cid, TurnID: turnID, Text: c.Text, Baseline: &conversation.ExpressionBaseline{Smile: settings.Smile, Brows: settings.Brows, Emotion: settings.EmotionBaseline, Regulation: settings.EmotionRegulation}})
		s.setState(session, "preparing", "")
	case "interrupt":
		s.mu.Lock()
		ignoreTurn(session, previous)
		session.currentTurn = ""
		session.pendingTurn = ""
		s.mu.Unlock()
		s.toWorker(envelope("interrupt", session.id, map[string]string{"reason": "interrupted"}))
		if previous != "" {
			err = channel.Send(ctx, conversation.Event{Type: "cancel", ConversationID: session.cid, TurnID: previous, Reason: "user interruption"})
		}
		if previous != "" {
			_ = s.store.finish(ctx, session.id, previous, "interrupted")
		}
		s.setState(session, "idle", "")
		_ = s.store.finish(ctx, session.id, turnID, "complete")
	case "preview":
		if c.Text == "laugh" {
			var audio any
			audio, err = s.conversation.LaughterPreview(settings.Voice, settings.Language)
			if err != nil {
				break
			}
			s.mu.Lock()
			session.currentTurn = turnID
			session.previewTurn = turnID
			s.mu.Unlock()
			s.toPlayer(session, envelope("preview_audio", session.id, map[string]string{"turn_id": turnID}))
			s.toWorker(envelope("preview", session.id, map[string]any{"kind": "laugh", "turn_id": turnID, "speech": audio}))
		} else {
			s.toWorker(envelope("preview", session.id, map[string]any{"kind": c.Text, "amount": c.Amount, "repetitions": c.Repetitions}))
		}
		_ = s.store.finish(ctx, session.id, turnID, "complete")
	case "configure":
		if c.Settings != nil {
			err = channel.Send(ctx, conversation.Event{Type: "expression_baseline", ConversationID: session.cid, TurnID: turnID, Baseline: &conversation.ExpressionBaseline{Smile: c.Settings.Smile, Brows: c.Settings.Brows, Emotion: c.Settings.EmotionBaseline, Regulation: c.Settings.EmotionRegulation}})
		}
		if modelChange {
			err = channel.Send(ctx, conversation.Event{Type: "configure", ConversationID: session.cid, TurnID: turnID, Instructions: c.Settings.Instructions, Voice: c.Settings.Voice})
		}
		if err == nil {
			err = s.store.settings(ctx, session.id, *c.Settings)
		}
		if err == nil {
			s.mu.Lock()
			if settings.EmotionBaseline.Normalized() != c.Settings.EmotionBaseline.Normalized() || settings.EmotionRegulation.Normalized() != c.Settings.EmotionRegulation.Normalized() || settings.AutomaticExpressions != c.Settings.AutomaticExpressions || settings.ReducedMotion != c.Settings.ReducedMotion {
				session.emotionPolicyRevision++
			}
			session.settings = *c.Settings
			s.mu.Unlock()
			s.toWorker(envelope("configure", session.id, map[string]any{"settings": *c.Settings, "emotion_policy_revision": session.emotionPolicyRevision}))
			_ = s.store.finish(ctx, session.id, turnID, "complete")
		}
	}
	if err != nil {
		_ = s.store.finish(ctx, session.id, turnID, "failed")
		s.mu.Lock()
		if session.pendingTurn == turnID {
			session.pendingTurn = ""
		}
		s.mu.Unlock()
		s.setState(session, "idle", "")
		return result, errors.New("The command could not be delivered. It was not retried.")
	}
	return result, nil
}

func (s *Service) Revoke(kind, id string) {
	s.mu.Lock()
	ids := []string{}
	for _, session := range s.sessions {
		if kind == "session" && session.principal.SessionID == id || kind == "key" && session.principal.KeyID == id {
			ids = append(ids, session.id)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		s.closeSession(id, "authorization revoked")
	}
}
func ignoreTurn(session *streamSession, id string) {
	if id == "" {
		return
	}
	session.ignoredTurns = append(session.ignoredTurns, id)
	if len(session.ignoredTurns) > 64 {
		session.ignoredTurns = session.ignoredTurns[len(session.ignoredTurns)-64:]
	}
}
func (s *Service) closeSession(id, reason string) {
	s.endSession(id, reason, time.Time{})
}

func (s *Service) endSession(id, reason string, inactiveAt time.Time) {
	s.mu.Lock()
	session := s.sessions[id]
	if session == nil {
		s.mu.Unlock()
		return
	}
	if !inactiveAt.IsZero() && reason == inactivityReason && inactiveAt.Before(session.inactivityDeadline()) {
		s.mu.Unlock()
		return
	}
	if reason == sessionLimitReason && (session.deadline.IsZero() || inactiveAt.Before(session.deadline)) {
		s.mu.Unlock()
		return
	}
	paused := reason == inactivityReason
	ended := reason == sessionLimitReason
	delete(s.sessions, id)
	session.state = "closed"
	if paused {
		session.state = "paused"
	}
	if ended {
		session.state = "ended"
	}
	finalState := session.state
	session.cancel()
	channel := session.channel
	player := session.player
	worker := s.worker
	if session.workerID != "" {
		worker = nil
		if current := s.workers[session.workerID]; current != nil {
			worker = current.peer
		}
	}
	s.mu.Unlock()
	if channel != nil {
		channel.Close()
	}
	worker.send(envelope("stop", id, map[string]string{"reason": reason}))
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), time.Second)
	defer cancel()
	if err := s.store.state(ctx, id, finalState, reason, true); err != nil {
		slog.Warn("Stream closure could not be persisted", "code", "stream_close_archive_failed", "stream_id", id)
	}
	if player != nil {
		if paused || ended {
			// Queued output need not drain on cancellation. Notify directly
			// with a bounded write, after stopping work and persisting the pause.
			notice, stop := context.WithTimeout(context.WithoutCancel(s.ctx), time.Second)
			kind := "paused"
			if ended {
				kind = "ended"
			}
			b, _ := json.Marshal(envelope(kind, id, map[string]string{"reason": reason}))
			_ = player.conn.Write(notice, websocket.MessageText, b)
			stop()
		}
		player.cancel()
		_ = player.conn.CloseNow()
	}
}
func (s *Service) setState(session *streamSession, state, reason string) {
	s.mu.Lock()
	if s.sessions[session.id] != session {
		s.mu.Unlock()
		return
	}
	session.state = state
	player := session.player
	s.mu.Unlock()
	if player != nil {
		player.send(envelope("state", session.id, map[string]string{"state": state, "reason": reason}))
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), time.Second)
	defer cancel()
	if err := s.store.state(ctx, session.id, state, reason, false); err != nil {
		slog.Warn("Stream state could not be persisted", "code", "stream_state_archive_failed", "stream_id", session.id)
	}
}
func (s *Service) expire() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-cleanup.C:
			ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
			s.purgeOldHistory(ctx)
			cancel()
		case now := <-ticker.C:
			s.expireAt(now)
		}
	}
}
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closing = true
	ids := []string{}
	for id := range s.sessions {
		ids = append(ids, id)
	}
	worker := s.worker
	cloudWorkers := make([]*peer, 0, len(s.workers))
	for _, current := range s.workers {
		cloudWorkers = append(cloudWorkers, current.peer)
	}
	s.mu.Unlock()
	s.cancel()
	for _, id := range ids {
		s.closeSession(id, "service shutdown")
	}
	if worker != nil {
		worker.cancel()
		_ = worker.conn.CloseNow()
	}
	for _, current := range cloudWorkers {
		current.cancel()
		_ = current.conn.CloseNow()
	}
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func randomID(prefix string) (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}
func issueTicket(session *streamSession) (string, error) {
	ticket, err := randomID("")
	if err != nil {
		return "", err
	}
	session.ticketHash = sha256.Sum256([]byte(ticket))
	session.ticketExpires = time.Now().Add(time.Minute)
	session.ticketUsed = false
	return ticket, nil
}
func consumeTicket(session *streamSession, ticket string, now time.Time) bool {
	hash := sha256.Sum256([]byte(ticket))
	valid := !session.ticketUsed && now.Before(session.ticketExpires) && subtle.ConstantTimeCompare(hash[:], session.ticketHash[:]) == 1
	if valid {
		session.ticketUsed = true
	}
	return valid
}
func decode(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func validPreview(kind string) bool {
	switch kind {
	case "nod", "double_nod", "shake", "emphasis", "tilt", "brows", "smile", "laugh":
		return true
	}
	return false
}
