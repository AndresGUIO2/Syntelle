package streaming

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"syntelle/api/internal/httpapi"
	"syntelle/render-contract"
	"time"

	"github.com/coder/websocket"
	"syntelle/api/internal/conversation"
)

type peer struct {
	coordinated bool
	conn        *websocket.Conn
	out         chan Envelope
	ctx         context.Context
	cancel      context.CancelFunc
}

type cloudWorker struct {
	peer    *peer
	started time.Time
}

func (p *peer) send(event Envelope) bool {
	if p == nil || p.ctx.Err() != nil {
		return false
	}
	select {
	case p.out <- event:
		return true
	default:
		p.cancel()
		_ = p.conn.CloseNow()
		return false
	}
}
func (s *Service) toWorker(event Envelope) bool {
	s.mu.Lock()
	worker := s.worker
	if session := s.sessions[event.SessionID]; session != nil && session.workerID != "" {
		if current := s.workers[session.workerID]; current != nil {
			worker = current.peer
		} else {
			worker = nil
		}
	}
	s.mu.Unlock()
	if event.Type == "activity" && worker != nil && !worker.coordinated {
		return true
	}
	if event.Type == "performance_preflight" && (worker == nil || !worker.coordinated) {
		return false
	}
	return worker.send(event)
}
func (s *Service) toPlayer(session *streamSession, event Envelope) {
	s.mu.Lock()
	if s.sessions[session.id] != session {
		s.mu.Unlock()
		return
	}
	player := session.player
	if player == nil && event.Type == "signal" && len(session.signals) < 64 {
		session.signals = append(session.signals, event)
	}
	s.mu.Unlock()
	if player != nil {
		player.send(event)
	}
}
func (s *Service) trackPeer(parent context.Context, c *websocket.Conn) *peer {
	ctx, cancel := context.WithCancel(parent)
	p := &peer{conn: c, out: make(chan Envelope, 32), ctx: ctx, cancel: cancel}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		cancel()
		return nil
	}
	s.wg.Add(2)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		defer c.CloseNow()
		for {
			select {
			case <-ctx.Done():
				return
			case message := <-p.out:
				b, err := json.Marshal(message)
				if err != nil {
					return
				}
				write, stop := context.WithTimeout(ctx, 5*time.Second)
				err = c.Write(write, websocket.MessageText, b)
				stop()
				if err != nil {
					return
				}
			}
		}
	}()
	return p
}
func (s *Service) workerSocket(w http.ResponseWriter, r *http.Request) {
	allocationID := r.Header.Get("X-Syntelle-Allocation")
	if httpapi.Cloud() {
		var valid bool
		err := s.store.pool.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM syntelle_streaming.worker_allocations WHERE id=$1 AND expires_at>now()) OR EXISTS(SELECT 1 FROM syntelle_streaming.worker_allocation WHERE id=$1 AND expires_at>now())", allocationID).Scan(&valid)
		if err != nil || !valid {
			writeError(w, 403, "Worker allocation required.")
			return
		}
	}
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if r.Method != http.MethodGet || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || s.config.WorkerToken == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(s.config.WorkerToken)) != 1 {
		writeError(w, 403, "Private renderer authorization required.")
		return
	}
	s.mu.Lock()
	busy := s.worker != nil
	if httpapi.Cloud() {
		busy = s.workers[allocationID] != nil
	}
	s.mu.Unlock()
	if busy {
		writeError(w, 409, "A renderer is already connected.")
		return
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	p := s.trackPeer(s.ctx, c)
	if p == nil {
		return
	}
	p.coordinated = r.Header.Get("X-Syntelle-Performance") == rendercontract.DeliveryVersion
	defer s.wg.Done()
	defer p.cancel()
	c.SetReadLimit(128 << 10)
	s.mu.Lock()
	if (!httpapi.Cloud() && s.worker != nil) || (httpapi.Cloud() && s.workers[allocationID] != nil) {
		s.mu.Unlock()
		return
	}
	if httpapi.Cloud() {
		if s.workers == nil {
			s.workers = map[string]*cloudWorker{}
		}
		s.workers[allocationID] = &cloudWorker{peer: p, started: time.Now()}
	} else {
		s.worker = p
		s.workerStarted = time.Now()
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if httpapi.Cloud() {
			if current := s.workers[allocationID]; current != nil && current.peer == p {
				delete(s.workers, allocationID)
			}
		} else if s.worker == p {
			s.worker = nil
		}
		ids := []string{}
		for id, session := range s.sessions {
			if !httpapi.Cloud() || session.workerID == allocationID {
				ids = append(ids, id)
			}
		}
		s.mu.Unlock()
		for _, id := range ids {
			s.closeSession(id, "private renderer disconnected")
		}
		if httpapi.Cloud() {
			archive, stop := context.WithTimeout(context.WithoutCancel(s.ctx), time.Second)
			defer stop()
			_, _ = s.store.pool.Exec(archive, "UPDATE syntelle_streaming.worker_allocations SET expires_at=now() WHERE id=$1", allocationID)
		}
	}()
	maximum := s.config.MaximumSessions
	if httpapi.Cloud() {
		maximum = cloudWorkerCapacity
	}
	p.send(envelope("registered", "", map[string]int{"maximum_sessions": maximum}))
	for {
		var event Envelope
		kind, b, err := c.Read(p.ctx)
		if err != nil {
			if p.ctx.Err() == nil {
				slog.Warn("Private renderer transport ended", "error", err)
			}
			return
		}
		if kind != websocket.MessageText || json.Unmarshal(b, &event) != nil {
			slog.Warn("Private renderer sent an invalid envelope", "message_type", kind, "bytes", len(b), "valid_json", json.Valid(b))
			return
		}
		s.mu.Lock()
		session := s.sessions[event.SessionID]
		if httpapi.Cloud() && session != nil && session.workerID != allocationID {
			session = nil
		}
		s.mu.Unlock()
		if session == nil {
			continue
		}
		switch event.Type {
		case "performance_notice":
			var notice rendercontract.DeliveryNotice
			if json.Unmarshal(event.Data, &notice) != nil {
				continue
			}
			if !s.deliveryNotice(session, notice) {
				continue
			}
			if notice.Applied != nil && notice.Applied.Validate() == nil && session.channel != nil {
				v := notice.Applied
				if err = session.channel.Send(p.ctx, conversation.Event{Type: "expression_baseline", ConversationID: session.cid, TurnID: "settings_applied", Baseline: &conversation.ExpressionBaseline{Smile: v.Smile, Brows: v.Brows, Emotion: v.EmotionBaseline, Regulation: v.EmotionRegulation}}); err != nil {
					s.closeSession(session.id, "performance state delivery failed")
				}
			}
		case "signal":
			s.toPlayer(session, event)
		case "ready":
			s.setState(session, "idle", "")
			s.toPlayer(session, event)
		case "playback":
			var playback struct {
				Event  string `json:"event"`
				TurnID string `json:"turn_id"`
			}
			if json.Unmarshal(event.Data, &playback) != nil {
				continue
			}
			s.mu.Lock()
			current := session.currentTurn
			isPreview := current == session.previewTurn
			channel := session.channel
			s.mu.Unlock()
			if playback.TurnID != current || current == "" || channel == nil {
				continue
			}
			if playback.Event != "playback_started" && playback.Event != "playback_waiting" && playback.Event != "playback_ended" {
				continue
			}
			if !isPreview {
				if err = channel.Send(p.ctx, conversation.Event{Type: playback.Event, ConversationID: session.cid, TurnID: current}); err != nil {
					s.closeSession(session.id, "playback control failed")
					continue
				}
			}
			switch playback.Event {
			case "playback_started":
				s.toWorker(envelope("activity", session.id, map[string]string{"state": "speaking"}))
				s.setState(session, "speaking", "")
			case "playback_waiting":
				s.setState(session, "preparing", "")
			case "playback_ended":
				s.toWorker(envelope("activity", session.id, map[string]string{"state": "idle"}))
				s.mu.Lock()
				if session.currentTurn == current {
					session.currentTurn = ""
				}
				s.mu.Unlock()
				s.setState(session, "idle", "")
				archive, stop := context.WithTimeout(s.ctx, time.Second)
				_ = s.store.finish(archive, session.id, current, "complete")
				stop()
			}
			s.toPlayer(session, event)
		case "emotion_decision":
			var decision rendercontract.EmotionDecision
			if json.Unmarshal(event.Data, &decision) != nil {
				continue
			}
			s.mu.Lock()
			valid := admitEmotionDecision(session, decision)
			channel := session.channel
			s.mu.Unlock()
			if valid && channel != nil {
				if err = channel.Send(p.ctx, conversation.Event{Type: "emotion_decision", ConversationID: session.cid, TurnID: decision.TurnID, EmotionDecision: &decision}); err != nil {
					s.closeSession(session.id, "emotion state delivery failed")
				}
			}
		case "emotion_activated":
			var activation rendercontract.EmotionActivation
			if json.Unmarshal(event.Data, &activation) != nil || !activation.Emotion.Valid() || activation.PhraseIndex < 0 || activation.PhraseIndex >= 1048576 {
				continue
			}
			s.mu.Lock()
			current := session.currentTurn
			channel := session.channel
			valid := admitEmotionActivation(session, activation)
			s.mu.Unlock()
			if !valid || channel == nil {
				continue
			}
			if err = channel.Send(p.ctx, conversation.Event{Type: "emotion_activated", ConversationID: session.cid, TurnID: current, EmotionActivation: &activation}); err != nil {
				s.closeSession(session.id, "emotion state delivery failed")
			}
		case "metrics":
			archive, stop := context.WithTimeout(s.ctx, time.Second)
			_ = s.store.metrics(archive, session.id, event.Data)
			stop()
		case "error":
			s.toPlayer(session, envelope("error", session.id, map[string]string{"message": "The private renderer failed. Start a new session."}))
			s.closeSession(session.id, "rendering failed")
		}
	}
}

func (s *Service) playerSocket(w http.ResponseWriter, r *http.Request, id string) {
	if !identifier.MatchString(id) {
		writeError(w, 404, "Stream not found.")
		return
	}
	if s.admission != nil {
		ip := s.admission.clientIP(r)
		if retry := s.admission.allow("ip-connect", ip, s.admission.config.ConnectionsPerMinute, time.Now()); retry != 0 {
			limited(w, retry, "Too many connection attempts. Try again shortly.")
			return
		}
		release, ok := s.admission.acquireConnection("ip", ip, s.admission.config.ConnectionsPerIP)
		if !ok {
			limited(w, 5, "Too many streaming connections. Try again shortly.")
			return
		}
		defer release()
	}
	patterns := []string{"localhost:5173", "127.0.0.1:5173"}
	if httpapi.Cloud() {
		patterns = []string{"app.syntelle.online"}
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: patterns})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(128 << 10)
	auth, stop := context.WithTimeout(r.Context(), 5*time.Second)
	_, b, err := c.Read(auth)
	stop()
	if err != nil {
		return
	}
	var event Envelope
	var data struct {
		Ticket string `json:"ticket"`
	}
	if json.Unmarshal(b, &event) != nil || event.Type != "authenticate" || event.SessionID != id || json.Unmarshal(event.Data, &data) != nil {
		return
	}
	s.mu.Lock()
	session := s.sessions[id]
	valid := session != nil && consumeTicket(session, data.Ticket, time.Now())
	s.mu.Unlock()
	if !valid {
		return
	}
	if s.admission != nil {
		if retry := s.admission.allow("principal-connect", session.principal.ID, s.admission.config.ConnectionsPerMinute, time.Now()); retry != 0 {
			_ = c.Close(websocket.StatusPolicyViolation, "Too many connection attempts.")
			return
		}
		release, ok := s.admission.acquireConnection("principal", session.principal.ID, s.admission.config.ConnectionsPerPrincipal)
		if !ok {
			_ = c.Close(websocket.StatusPolicyViolation, "Too many streaming connections.")
			return
		}
		defer release()
	}
	if s.config.ValidatePrincipal != nil {
		if err = s.config.ValidatePrincipal(r.Context(), session.principal); err != nil {
			s.closeSession(id, "authorization expired")
			return
		}
	}
	p := s.trackPeer(s.ctx, c)
	if p == nil {
		return
	}
	defer s.wg.Done()
	defer p.cancel()
	s.mu.Lock()
	if s.sessions[id] != session {
		s.mu.Unlock()
		return
	}
	previous := session.player
	reconnect := session.playerAttached
	session.playerAttached = true
	session.player = p
	session.lastSeen = time.Now()
	signals := session.signals
	session.signals = nil
	response := s.response(session, "")
	s.mu.Unlock()
	if previous != nil {
		previous.cancel()
		_ = previous.conn.CloseNow()
	}
	defer func() {
		s.mu.Lock()
		if session.player == p {
			session.player = nil
			session.lastSeen = time.Now()
		}
		s.mu.Unlock()
	}()
	p.send(envelope("session", id, response))
	p.send(envelope("state", id, map[string]string{"state": response.State}))
	if reconnect {
		s.toWorker(envelope("connect", id, map[string]bool{"reconnect": true}))
	} else {
		for _, signal := range signals {
			p.send(signal)
		}
	}
	lastAuthorization := time.Now()
	checkAuthorization := func() bool {
		check, stop := context.WithTimeout(p.ctx, time.Second)
		err := s.config.ValidatePrincipal(check, session.principal)
		stop()
		if err != nil {
			s.closeSession(id, "authorization expired")
			return false
		}
		lastAuthorization = time.Now()
		return true
	}
	for {
		// The queued worker envelope retains Data after this iteration. A fresh
		// value prevents the next decode from reusing its RawMessage storage.
		var event Envelope
		kind, b, err := c.Read(p.ctx)
		if err != nil {
			return
		}
		if kind != websocket.MessageText || json.Unmarshal(b, &event) != nil {
			return
		}
		if event.SessionID != id {
			return
		}
		s.mu.Lock()
		active := s.sessions[id] == session && session.player == p
		if active {
			session.lastSeen = time.Now()
		}
		channel := session.channel
		starting := session.state == "starting"
		s.mu.Unlock()
		if !active {
			return
		}
		// Audio/signaling must not extend authorization by skipping heartbeats.
		authorizationDue := s.config.ValidatePrincipal != nil && time.Since(lastAuthorization) >= time.Second
		if authorizationDue && !checkAuthorization() {
			return
		}
		if s.admission != nil && event.Type != "microphone" {
			if retry := s.admission.request(s.admission.clientIP(r), session.principal.ID, time.Now()); retry != 0 {
				p.send(envelope("error", id, map[string]any{"message": "Too many streaming requests. Try again shortly.", "retry_after": retry, "command_id": event.CommandID}))
				continue
			}
		}
		// Rejected control floods must not cause an authorization database read
		// per message; only admitted commands/heartbeats get the extra check.
		if !authorizationDue && s.config.ValidatePrincipal != nil && (event.Type == "heartbeat" || event.Type == "command") && !checkAuthorization() {
			return
		}
		switch event.Type {
		case "heartbeat":
			p.send(envelope("heartbeat", id, nil))
		case "signal":
			if len(event.Data) > 64<<10 {
				return
			}
			s.toWorker(event)
		case "command":
			var command Command
			if json.Unmarshal(event.Data, &command) != nil {
				continue
			}
			command.ID = event.CommandID
			result, err := s.command(p.ctx, session, command)
			if err != nil {
				p.send(envelope("error", id, map[string]string{"message": err.Error(), "command_id": command.ID}))
				continue
			}
			p.send(envelope("command", id, result))
		case "microphone":
			// Admission precedes GPU startup; do not open paid recognition while
			// the admitted renderer is still starting.
			if starting {
				p.send(envelope("error", id, map[string]string{"message": "The renderer is still starting."}))
				continue
			}
			var microphone conversation.Event
			if json.Unmarshal(event.Data, &microphone) != nil || !identifier.MatchString(microphone.TurnID) || channel == nil {
				continue
			}
			if microphone.Type != "mic_start" && microphone.Type != "mic_stop" && microphone.Type != "audio" && microphone.Type != "activity" {
				continue
			}
			if len(microphone.Audio) > 6400 {
				return
			}
			// PCM frames are a continuous media stream, not new API requests.
			// Starting/stopping recognition still consumes the request allowance.
			if s.admission != nil && microphone.Type != "audio" && microphone.Type != "activity" {
				if retry := s.admission.request(s.admission.clientIP(r), session.principal.ID, time.Now()); retry != 0 {
					p.send(envelope("error", id, map[string]any{"message": "Too many streaming requests. Try again shortly.", "retry_after": retry}))
					continue
				}
			}
			microphone.ConversationID = session.cid
			if err = channel.Send(p.ctx, microphone); err != nil {
				return
			}
		}
	}
}

func (s *Service) conversationEvent(session *streamSession, event conversation.Event) {
	s.mu.Lock()
	if s.sessions[session.id] != session {
		s.mu.Unlock()
		return
	}
	if event.Type == "accepted" {
		for _, ignored := range session.ignoredTurns {
			if ignored == event.TurnID {
				s.mu.Unlock()
				return
			}
		}
		if session.pendingTurn != "" && session.pendingTurn != event.TurnID {
			s.mu.Unlock()
			return
		}
		session.currentTurn = event.TurnID
		session.pendingTurn = ""
		if strings.TrimSpace(event.Text) != "" {
			session.lastMessage = time.Now()
		}
	}
	current := session.currentTurn
	if event.Type == "transcript_final" && strings.TrimSpace(event.Text) != "" {
		session.lastMessage = time.Now()
	}
	if event.Type == "error" && event.TurnID == session.pendingTurn {
		current = session.pendingTurn
	}
	s.mu.Unlock()
	if event.Type == "accepted" && session.principal.AccountID != "" && strings.TrimSpace(event.Text) != "" {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		if err := s.store.saveTurn(ctx, session.cid, session.principal.AccountID); err != nil {
			slog.Warn("Conversation history could not be saved", "code", "history_save_failed")
		}
		cancel()
	}
	if event.Type == "ready" {
		return
	}
	independent := event.Type == "transcript_partial" || event.Type == "transcript_final" || event.Type == "listening" || event.Type == "speech_detected" || event.Type == "microphone_error" || event.Type == "persistence_warning" || event.Type == "cancelled"
	if !independent && event.TurnID != current {
		return
	}
	activity := ""
	switch event.Type {
	case "listening", "speech_detected":
		activity = "listening"
	case "accepted", "preparing":
		activity = "preparing"
	case "cancelled", "error":
		activity = "idle"
	}
	if activity != "" {
		s.toWorker(envelope("activity", session.id, map[string]string{"state": activity}))
	}
	if event.Type == "accepted" {
		s.setState(session, "preparing", "")
		s.toWorker(envelope("interrupt", session.id, map[string]string{"reason": "new turn"}))
		s.toWorker(envelope("conversation", session.id, event))
	}
	// Private renderer receives the original prepared speech. The public socket
	// receives only a small allowlisted status representation for audio events.
	if event.Type == "speech_prefix" || event.Type == "speech_part" || event.Type == "prepared_speech" || event.Type == "speech_complete" {
		s.toWorker(envelope("conversation", session.id, event))
		if event.Type != "speech_complete" {
			return
		}
	}
	if event.Type == "error" || event.Type == "cancelled" {
		if event.TurnID == current {
			s.toWorker(envelope("interrupt", session.id, map[string]string{"reason": "turn ended"}))
			s.mu.Lock()
			session.currentTurn = ""
			session.pendingTurn = ""
			s.mu.Unlock()
			s.setState(session, "idle", "")
			archive, stop := context.WithTimeout(s.ctx, time.Second)
			status := "failed"
			if event.Type == "cancelled" {
				status = "interrupted"
			}
			_ = s.store.finish(archive, session.id, current, status)
			stop()
		}
	}
	if public, ok := publicConversation(event, session.id); ok {
		s.toPlayer(session, envelope("conversation", session.id, public))
	}
}

// Explicit allowlisting prevents a future provider/diagnostic event from leaking
// native audio or private payloads through the player protocol.
func publicConversation(event conversation.Event, streamID string) (conversation.Event, bool) {
	switch event.Type {
	case "accepted", "reply_delta", "reply_complete", "preparing", "transcript_partial", "transcript_final", "listening", "speech_detected", "microphone_error", "persistence_warning", "cancelled", "error", "speech_complete":
	default:
		return conversation.Event{}, false
	}
	public := conversation.Event{Type: event.Type, ConversationID: streamID, TurnID: event.TurnID, Text: event.Text, Reason: event.Reason}
	if event.Type == "reply_complete" {
		public.Data = map[string]string{"spoken": event.Text}
	}
	if event.Type == "speech_complete" {
		b, _ := json.Marshal(event.Data)
		var part struct {
			Parts int `json:"parts"`
		}
		_ = json.Unmarshal(b, &part)
		public.Data = part
	}
	return public, true
}
