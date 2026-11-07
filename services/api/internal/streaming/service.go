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
	characterMu   sync.Mutex // serialize admitted custom creation against committed deletion
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
	conversations.Delivery = s
	s.purgeOldHistory(ctx)
	s.wg.Add(1)
	go s.expire()
	return s, nil
}

func (s *Service) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/widgets/") {
			s.widget(w, r)
			return
		}
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
			writeJSON(w, 200, map[string]any{"laughter": map[string]bool{"en-US": s.conversation.HasLaughter(s.conversation.RuntimeVoice(), "en-US"), "es-CO": s.conversation.HasLaughter(s.conversation.RuntimeVoice(), "es-CO")}, "available": available, "configured": s.conversation.Speech != nil && s.conversation.Model != nil, "speech_configured": s.conversation.Speech != nil, "recognition": s.conversation.Recognition != nil, "capacity": s.config.MaximumSessions, "active": count, "time_limits_enabled": !s.config.DisableTimeLimits})
			return
		}
		if len(parts) == 2 && parts[1] == "ws" && r.Method == http.MethodGet {
			s.playerSocket(w, r, parts[0])
			return
		}
		principal, err := s.resolveStreamPrincipal(w, r)
		if err != nil {
			writeError(w, 401, "Sign in or start a visitor session.")
			return
		}
		if !s.allowStreamRequest(w, r, principal.ID) {
			return
		}
		if r.URL.Path == "/api/v1/streams" && r.Method == http.MethodPost {
			if principal.Kind == "widget" {
				writeError(w, 403, "Widget permissions are limited to one stream.")
				return
			}
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
	settings := session.settings
	if session.principal.IntegrationID != "" {
		settings.Instructions = ""
	}
	return SessionResponse{EmotionsAvailable: !session.basicCharacter, LaughterAvailable: !session.basicCharacter && s.conversation.HasLaughter(session.settings.Voice, session.settings.Language), StartupTimeoutMS: startupTimeout, ICEServers: ice, ID: session.id, Ticket: ticket, State: session.state, Settings: settings, Configured: s.conversation.Model != nil && s.conversation.Speech != nil, SpeechConfigured: s.conversation.Speech != nil, Recognition: s.conversation.Recognition != nil && (session.capabilities == nil || session.capabilities["message"]), RemainingSeconds: max(0, int(math.Ceil(time.Until(session.deadline).Seconds()))), TimeLimitsEnabled: !s.config.DisableTimeLimits || session.principal.IntegrationID != ""}
}

type streamCreateInput struct {
	Settings               accounts.Settings `json:"settings"`
	ResumeFrom             string            `json:"resume_from,omitempty"`
	ContinueConversationID string            `json:"continue_conversation_id,omitempty"`
	IntegrationID          string            `json:"integration_id,omitempty"`
}

func (s *Service) create(w http.ResponseWriter, r *http.Request, principal accounts.Principal) {
	input := streamCreateInput{Settings: accounts.Defaults()}
	if decode(w, r, &input) != nil {
		writeError(w, 400, "Invalid stream settings.")
		return
	}
	s.createInput(w, r, principal, input)
}
func (s *Service) createInput(w http.ResponseWriter, r *http.Request, principal accounts.Principal, input streamCreateInput) {
	var integration *accounts.PublishedIntegration
	if principal.IntegrationID != "" {
		if input.IntegrationID != "" && input.IntegrationID != principal.IntegrationID || input.ResumeFrom != "" || input.ContinueConversationID != "" || s.config.ResolveIntegration == nil {
			writeError(w, 403, "Invalid integration session.")
			return
		}
		i, err := s.config.ResolveIntegration(r.Context(), principal, principal.IntegrationID)
		if err != nil {
			writeError(w, 403, "Integration is unavailable.")
			return
		}
		integration = &i
		input.Settings = i.Settings
	} else if input.IntegrationID != "" {
		writeError(w, 403, "An integration credential is required.")
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
	input.Settings.Voice = s.conversation.RuntimeVoice()
	if err := input.Settings.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if input.Settings.Character != accounts.Defaults().Character {
		s.characterMu.Lock()
		defer s.characterMu.Unlock()
	}
	var packageHash string
	var err error
	if integration != nil {
		packageHash = integration.Publication.PackageHash
	} else {
		packageHash, err = s.resolvePackage(r.Context(), principal, input.Settings.Character)
	}
	if err != nil {
		writeError(w, 404, "Prepared character not found.")
		return
	}
	basic := input.Settings.Character != accounts.Defaults().Character
	characterName := ""
	if basic {
		normalizeBasicSettings(&input.Settings)
		if integration != nil {
			characterName = integration.Publication.Name
		} else {
			characterName, err = s.store.characterName(r.Context(), principal.AccountID, input.Settings.Character, packageHash)
		}
		if err != nil {
			writeError(w, 404, "Prepared character not found.")
			return
		}
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
		if err := s.store.matchesPausedCharacter(r.Context(), input.ResumeFrom, principal.ID, input.Settings.Character, packageHash); err != nil {
			writeError(w, 409, "Start a new chat to change character after a pause.")
			return
		}
		id = input.ResumeFrom
	}
	ctx, cancel := context.WithCancel(s.ctx)
	session := &streamSession{packageHash: packageHash, basicCharacter: basic, emotionalPresence: !basic && !httpapi.Cloud() && s.conversation != nil && s.conversation.Performance != nil && s.conversation.Performance.EmotionalPresence != nil, emotionPolicyRevision: 1, id: id, cid: cid, principal: principal, settings: input.Settings, state: "starting", created: time.Now(), lastSeen: time.Now(), cancel: cancel, commandMu: make(chan struct{}, 1)}
	capability := ""
	if integration != nil {
		session.nextUsageRenewal = time.Now().Add(10 * time.Second)
		session.capabilities = map[string]bool{}
		for _, v := range integration.Capabilities {
			session.capabilities[v] = true
		}
		if principal.Kind == "widget" {
			capability, err = randomID("syntelle_wp_")
			if err != nil {
				cancel()
				writeError(w, 500, "Player permission could not be created.")
				return
			}
			session.capabilityHash = sha256.Sum256([]byte(capability))
		}
	}
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
	if integration != nil {
		if s.config.ReserveIntegrationUsage == nil || s.config.ReleaseIntegrationUsage == nil || s.config.RenewIntegrationUsage == nil || s.config.StartIntegrationUsage == nil || s.config.ValidatePrincipal == nil {
			err = errors.New("Paid workspace usage is unavailable.")
		} else {
			err = s.config.ValidatePrincipal(r.Context(), principal)
			if err == nil {
				deadline, err = s.config.ReserveIntegrationUsage(r.Context(), principal.WorkspaceID, principal.IntegrationID, id)
			}
		}
		if err == nil && !deadline.After(time.Now()) {
			err = errors.New("Invalid paid usage lease.")
		}
		if err == nil {
			err = s.store.allocateIntegration(r.Context(), session, packageHash)
		}
	} else if s.config.DisableTimeLimits {
		err = s.store.allocateUnmetered(r.Context(), session, packageHash, input.ResumeFrom != "")
	} else {
		deadline, err = s.store.allocate(r.Context(), session, packageHash, input.ResumeFrom != "")
	}
	if err != nil {
		if integration != nil && s.config.ReleaseIntegrationUsage != nil {
			releaseCtx, releaseCancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
			_ = s.config.ReleaseIntegrationUsage(releaseCtx, id)
			releaseCancel()
		}
		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()
		cancel()
		if integration != nil {
			writeError(w, 429, "This workspace has no available usage or simultaneous session slot.")
		} else if errors.Is(err, errDemoTime) || errors.Is(err, errSessionTime) {
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
	if integration != nil {
		if err = s.config.ValidatePrincipal(ctx, principal); err != nil {
			s.closeSession(id, "authorization revoked")
			writeError(w, 403, "Integration is no longer available.")
			return
		}
		s.mu.Lock()
		active := s.sessions[id] == session
		s.mu.Unlock()
		if !active || ctx.Err() != nil {
			s.closeSession(id, "authorization revoked")
			writeError(w, 403, "Integration is no longer available.")
			return
		}
	}
	channel, err := s.conversation.OpenCharacterChannel(ctx, basic, packageHash, input.Settings.Character, characterName, func(event conversation.Event) { s.conversationEvent(session, event) })
	if err != nil {
		s.closeSession(id, "conversation unavailable")
		writeError(w, 503, "Conversation service is unavailable.")
		return
	}
	s.mu.Lock()
	session.channel = channel
	s.mu.Unlock()
	helloID, _ := randomID("hello_")
	err = channel.Send(ctx, conversation.Event{Type: "hello", ConversationID: cid, TurnID: helloID, Character: input.Settings.Character, Language: input.Settings.Language, Voice: input.Settings.Voice, Instructions: input.Settings.Instructions})
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
	result.Capability = capability
	s.mu.Unlock()
	writeJSON(w, 201, result)
}

func (s *Service) command(ctx context.Context, session *streamSession, c Command) (commandResult, error) {
	if session.capabilities != nil && (!session.capabilities[c.Type] || c.Settings != nil) {
		return commandResult{ID: c.ID}, errors.New("This action is not allowed by the integration.")
	}
	if session.principal.IntegrationID != "" && (c.Type == "message" || c.Type == "speak") && s.admission != nil {
		if s.admission.allow("integration-turn", session.principal.IntegrationID, 60, time.Now()) != 0 || s.admission.allow("integration-player-turn", session.principal.ID, 12, time.Now()) != 0 {
			return commandResult{ID: c.ID}, errors.New("Please wait before sending another message.")
		}
	}
	if c.Settings != nil {
		next := *c.Settings
		next.Voice = s.conversation.RuntimeVoice()
		if session.basicCharacter {
			normalizeBasicSettings(&next)
		}
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
	if c.Type != "message" && c.Type != "speak" && c.Type != "interrupt" && c.Type != "configure" && c.Type != "preview" && c.Type != "suspend" {
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
	if state == "starting" && c.Type != "suspend" {
		return result, errors.New("Wait for the renderer to become ready.")
	}
	if c.Type == "preview" && state != "idle" {
		return result, errors.New("Wait until the reply ends before previewing.")
	}
	modelChange := c.Settings != nil && (settings.Instructions != c.Settings.Instructions || settings.Voice != c.Settings.Voice)
	if c.Type == "preview" && session.basicCharacter && c.Text == "laugh" {
		return result, errors.New("Laughter is unavailable for this character.")
	}
	if c.Type == "configure" {
		if settings.Character != c.Settings.Character {
			return result, errors.New("Start a new stream to change character.")
		}
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
	case "suspend":
		// The terminal paused notice, rather than a queued command receipt,
		// confirms that continuation is durable before the player disconnects.
		err = s.finishSession(session.id, navigationReason, time.Time{})
		if err != nil {
			slog.Warn("Stream navigation suspension failed", "code", "navigation_suspend_failed", "stream_id", session.id)
		}
		return result, err
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
		err = s.store.settings(ctx, session.id, *c.Settings)
		if err == nil {
			s.mu.Lock()
			policyChanged := settings.EmotionBaseline.Normalized() != c.Settings.EmotionBaseline.Normalized() || settings.EmotionRegulation.Normalized() != c.Settings.EmotionRegulation.Normalized() || settings.AutomaticExpressions != c.Settings.AutomaticExpressions || settings.ReducedMotion != c.Settings.ReducedMotion
			if session.requestedPolicyRevision == 0 {
				session.requestedPolicyRevision = max(1, session.emotionPolicyRevision)
			}
			if policyChanged {
				session.requestedPolicyRevision++
			}
			deferred := session.coordinated && previous != "" && policyChanged && settings.ReducedMotion == c.Settings.ReducedMotion
			if deferred {
				if session.appliedSettings == nil {
					copy := settings
					session.appliedSettings = &copy
				}
			} else {
				copy := *c.Settings
				session.appliedSettings = &copy
				session.emotionPolicyRevision = session.requestedPolicyRevision
			}
			revision := session.requestedPolicyRevision
			session.settings = *c.Settings
			s.mu.Unlock()
			if !deferred {
				err = channel.Send(ctx, conversation.Event{Type: "expression_baseline", ConversationID: session.cid, TurnID: turnID, Baseline: &conversation.ExpressionBaseline{Smile: c.Settings.Smile, Brows: c.Settings.Brows, Emotion: c.Settings.EmotionBaseline, Regulation: c.Settings.EmotionRegulation}})
			}
			if modelChange && err == nil {
				err = channel.Send(ctx, conversation.Event{Type: "configure", ConversationID: session.cid, TurnID: turnID, Instructions: c.Settings.Instructions, Voice: c.Settings.Voice})
			}
			if err == nil && !s.toWorker(envelope("configure", session.id, map[string]any{"settings": *c.Settings, "emotion_policy_revision": revision})) {
				err = errors.New("private settings delivery unavailable")
			}
			if err == nil {
				err = s.store.finish(ctx, session.id, turnID, "complete")
			}
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
		if kind == "session" && session.principal.SessionID == id || (kind == "key" || kind == "workspace_key") && session.principal.KeyID == id || kind == "workspace" && session.principal.WorkspaceID == id || kind == "integration" && session.principal.IntegrationID == id || kind == "publication" && session.principal.PublicationID == id {
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
	if err := s.finishSession(id, reason, inactiveAt); err != nil {
		slog.Warn("Stream closure could not be persisted", "code", "stream_close_archive_failed", "stream_id", id)
	}
}

func (s *Service) finishSession(id, reason string, inactiveAt time.Time) error {
	s.mu.Lock()
	session := s.sessions[id]
	if session == nil {
		s.mu.Unlock()
		if reason == navigationReason {
			return errors.New("This stream has ended.")
		}
		return nil
	}
	if !inactiveAt.IsZero() && reason == inactivityReason && inactiveAt.Before(session.inactivityDeadline()) {
		s.mu.Unlock()
		return nil
	}
	if reason == sessionLimitReason && (session.deadline.IsZero() || inactiveAt.Before(session.deadline)) {
		s.mu.Unlock()
		return nil
	}
	paused := reason == inactivityReason || reason == navigationReason
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
	persistErr := s.store.state(ctx, id, finalState, reason, true)
	if session.principal.IntegrationID != "" && s.config.ReleaseIntegrationUsage != nil {
		billingCtx, billingCancel := context.WithTimeout(context.WithoutCancel(s.ctx), 2*time.Second)
		billingErr := s.config.ReleaseIntegrationUsage(billingCtx, id)
		billingCancel()
		if persistErr == nil {
			persistErr = billingErr
		}
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
			data := map[string]string{"reason": reason}
			if persistErr != nil {
				kind = "error"
				data = map[string]string{"code": "suspension_failed", "message": "Conversation suspension could not be saved. Reconnect explicitly to continue."}
			}
			b, _ := json.Marshal(envelope(kind, id, data))
			_ = player.conn.Write(notice, websocket.MessageText, b)
			stop()
		}
		player.cancel()
		_ = player.conn.CloseNow()
	}
	if persistErr != nil {
		return errors.New("Conversation suspension could not be saved. Reconnect explicitly to continue.")
	}
	return nil
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
			s.renewIntegrationUsage(now)
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
