package nativemedia

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"reflect"
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
)

type pendingDelivery struct {
	reservation rendercontract.DeliveryReservation
	policy      nativeanimation.ReservedEmotion
	settings    rendercontract.Settings
	expires     float64
}

// Caller owns sendMu and mu: requested revisions cannot race first PCM send.
func (s *session) invalidateDeliveryLocked(reason string) {
	if s.delivery == nil {
		return
	}
	r := s.delivery.reservation
	kept := s.queue[:0]
	for _, c := range s.queue {
		if c.delivery != nil && c.delivery.Token == r.Token {
			if s.preparing != nil && s.preparing.clip == c {
				s.preparing.detached = true
				s.preparing.cancel()
			} else {
				s.queuedBytes -= len(c.samples) * 2
			}
		} else {
			kept = append(kept, c)
		}
	}
	s.queue = kept
	s.nextIndex = min(s.nextIndex, r.Request.PhraseIndex)
	s.delivery = nil
	if s.host != nil {
		s.host.emit(event("performance_notice", s.id, r.Request.TurnID, rendercontract.DeliveryNotice{RequestID: r.Request.RequestID, Token: r.Token, Status: "invalidated", Reason: reason}))
	}
}

func (s *session) preflight(raw json.RawMessage) error {
	var request rendercontract.DeliveryRequest
	if json.Unmarshal(raw, &request) != nil || !request.Valid() {
		return errors.New("invalid performance preflight")
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	notice := rendercontract.DeliveryNotice{RequestID: request.RequestID, Status: "rejected", Reason: "stale_delivery"}
	defer func() { s.host.emit(event("performance_notice", s.id, request.TurnID, notice)) }()
	settings, revision := s.settings, s.policyRevision
	if s.requestedSettings != nil {
		settings, revision = *s.requestedSettings, s.requestedRevision
	}
	if request.TurnID != s.turn || s.ignored[request.TurnID] || request.Revision != revision || request.Voice != settings.Voice || request.Language != settings.Language {
		return nil
	}
	if s.delivery != nil {
		if s.emotionClock > s.delivery.expires {
			s.invalidateDeliveryLocked("expired_delivery")
			notice.Reason = "expired_delivery"
			return nil
		}
		if reflect.DeepEqual(s.delivery.reservation.Request, request) {
			notice.Status, notice.Reason, notice.Reservation = "reserved", "", &s.delivery.reservation
			return nil
		}
		notice.Reason = "pending_phrase_exists"
		return nil
	}
	if request.PhraseIndex != s.nextIndex {
		notice.Reason = "out_of_order_phrase"
		return nil
	}
	capability, ok := s.scene.(interface{ EmotionsAvailable() bool })
	if !ok || !capability.EmotionsAvailable() {
		notice.Reason = "emotion_capability_unavailable"
		return nil
	}
	policy, decision := s.emotionController.ReserveEmotion(s.emotionClock, settings, request, s.generation, s.requestedSettings != nil)
	r := rendercontract.DeliveryReservation{Expressive: settings.AutomaticExpressions && !settings.ReducedMotion, Version: rendercontract.DeliveryVersion, Token: rand.Text(), Request: request, Instance: s.emotionInstance, Generation: s.generation, Effective: decision.Effective, Reason: decision.Reason}
	s.delivery = &pendingDelivery{reservation: r, policy: policy, settings: settings, expires: s.emotionClock + 90000}
	s.coordinated = true
	notice.Status, notice.Reason, notice.Reservation = "reserved", "", &r
	return nil
}

func (s *session) deliveryMatchesLocked(r *rendercontract.DeliveryReservation, text string, index int) bool {
	return r.Valid() && s.delivery != nil && reflect.DeepEqual(*r, s.delivery.reservation) && r.Instance == s.emotionInstance && r.Generation == s.generation && r.Request.TurnID == s.turn && r.Request.PhraseIndex == index && r.Request.TextHash == rendercontract.TextHash(text) && s.emotionClock <= s.delivery.expires
}

func performanceSettingsEqual(a, b rendercontract.Settings) bool {
	return a.EmotionBaseline.Normalized() == b.EmotionBaseline.Normalized() && a.EmotionRegulation.Normalized() == b.EmotionRegulation.Normalized() && a.AutomaticExpressions == b.AutomaticExpressions && a.ReducedMotion == b.ReducedMotion
}

func preservePerformanceSettings(appearance, applied rendercontract.Settings) rendercontract.Settings {
	appearance.EmotionBaseline, appearance.EmotionRegulation = applied.EmotionBaseline, applied.EmotionRegulation
	appearance.AutomaticExpressions, appearance.ReducedMotion = applied.AutomaticExpressions, applied.ReducedMotion
	return appearance
}
