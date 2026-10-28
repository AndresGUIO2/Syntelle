package streaming

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syntelle/api/internal/conversation"
	"syntelle/render-contract"
	"time"
)

func (s *Service) ReserveDelivery(ctx context.Context, cid string, r rendercontract.DeliveryRequest) (*conversation.DeliveryTicket, error) {
	s.mu.Lock()
	var session *streamSession
	for _, candidate := range s.sessions {
		if candidate.cid == cid {
			session = candidate
			break
		}
	}
	s.mu.Unlock()
	if session == nil {
		return nil, errors.New("private performance session unavailable")
	}
	select {
	case session.commandMu <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.mu.Lock()
	if s.sessions[session.id] != session || session.currentTurn != r.TurnID {
		s.mu.Unlock()
		<-session.commandMu
		return nil, errors.New("obsolete performance turn")
	}
	if session.requestedPolicyRevision == 0 {
		session.requestedPolicyRevision = max(1, session.emotionPolicyRevision)
	}
	r.Revision = session.requestedPolicyRevision
	r.RequestID = fmt.Sprintf("%s-p%d-r%d", r.TurnID, r.PhraseIndex, r.Revision)
	if !r.Valid() {
		s.mu.Unlock()
		<-session.commandMu
		return nil, errors.New("invalid delivery request")
	}
	if session.deliveryWaiters == nil {
		session.deliveryWaiters = map[string]chan rendercontract.DeliveryNotice{}
	}
	if session.deliveryWaiters[r.RequestID] != nil {
		s.mu.Unlock()
		<-session.commandMu
		return nil, errors.New("delivery request is already pending")
	}
	updates := make(chan rendercontract.DeliveryNotice, 4)
	session.deliveryWaiters[r.RequestID] = updates
	session.coordinated = true
	s.mu.Unlock()
	var once sync.Once
	closeTicket := func() { once.Do(func() { s.mu.Lock(); delete(session.deliveryWaiters, r.RequestID); s.mu.Unlock() }) }
	if !s.toWorker(envelope("performance_preflight", session.id, r)) {
		<-session.commandMu
		closeTicket()
		return nil, errors.New("private preflight could not be sent")
	}
	<-session.commandMu
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		closeTicket()
		return nil, ctx.Err()
	case <-timer.C:
		closeTicket()
		return nil, errors.New("private preflight timed out before synthesis")
	case n := <-updates:
		if n.Status != "reserved" || !n.Reservation.Valid() || n.Reservation.Request != r && !sameDeliveryRequest(n.Reservation.Request, r) {
			closeTicket()
			s.mu.Lock()
			newer := session.requestedPolicyRevision > r.Revision
			s.mu.Unlock()
			if n.Status == "invalidated" || n.Reason == "stale_delivery" && newer {
				return nil, conversation.ErrDeliverySuperseded
			}
			return nil, errors.New("private performance reservation rejected")
		}
		return &conversation.DeliveryTicket{Reservation: *n.Reservation, Updates: updates, Close: closeTicket}, nil
	}
}

func sameDeliveryRequest(a, b rendercontract.DeliveryRequest) bool {
	if a.Emotion == nil || b.Emotion == nil {
		return a.Emotion == nil && b.Emotion == nil && a == b
	}
	x, y := *a.Emotion, *b.Emotion
	a.Emotion, b.Emotion = nil, nil
	return x == y && a == b
}

// Called only after worker/session ownership has been authenticated by workerSocket.
func (s *Service) deliveryNotice(session *streamSession, n rendercontract.DeliveryNotice) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[session.id] != session {
		return false
	}
	if n.Status == "reserved" {
		if !n.Reservation.Valid() || n.Reservation.Request.TurnID != session.currentTurn || n.Reservation.Request.Revision != session.requestedPolicyRevision || session.deliveryWaiters[n.RequestID] == nil {
			return false
		}
		copy := *n.Reservation
		session.reservedDelivery = &copy
	}
	if n.Status == "activated" {
		if !n.Reservation.Valid() || session.reservedDelivery == nil || n.Token != session.reservedDelivery.Token || n.Reservation.Instance != session.reservedDelivery.Instance || n.Reservation.Generation != session.reservedDelivery.Generation || n.Reservation.Request.TurnID != session.currentTurn || n.Applied == nil || n.Applied.Validate() != nil {
			return false
		}
		session.reservedDelivery = nil
		copy := *n.Applied
		session.appliedSettings = &copy
		session.emotionPolicyRevision = n.Reservation.Request.Revision
	} else if n.Status == "settings_applied" && n.Applied != nil {
		if n.Revision != session.requestedPolicyRevision || n.Applied.Validate() != nil || n.Applied.EmotionBaseline.Normalized() != session.settings.EmotionBaseline.Normalized() || n.Applied.EmotionRegulation.Normalized() != session.settings.EmotionRegulation.Normalized() {
			return false
		}
		copy := *n.Applied
		session.appliedSettings = &copy
		session.emotionPolicyRevision = session.requestedPolicyRevision
	}
	if n.Status == "invalidated" && session.reservedDelivery != nil && n.Token == session.reservedDelivery.Token {
		session.reservedDelivery = nil
	}
	if ch := session.deliveryWaiters[n.RequestID]; ch != nil {
		select {
		case ch <- n:
		default:
			session.cancel()
		}
	}
	return true
}

func appliedSettings(s *streamSession) rendercontract.Settings {
	if s.appliedSettings != nil {
		return *s.appliedSettings
	}
	return s.settings
}
