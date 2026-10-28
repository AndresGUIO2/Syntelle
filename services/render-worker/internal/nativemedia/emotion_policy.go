package nativemedia

import "syntelle/render-contract"

// Commit only the state staged for PCM that was actually sent. Configuration
// changes and cancellation invalidate an in-flight copy without touching audio.
func (s *session) commitEmotions(block audioBlock, sent bool) []rendercontract.EmotionDecision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !sent || !block.policy || block.generation != s.generation || block.epoch != s.emotionEpoch {
		return nil
	}
	if block.delivery != nil && (s.delivery != block.delivery || s.emotionClock > block.delivery.expires) {
		return nil
	}
	if block.applied != nil {
		s.settings = preservePerformanceSettings(s.settings, *block.applied)
		s.policyRevision = block.policyRevision
		if s.requestedRevision <= s.policyRevision {
			s.requestedSettings = nil
		}
	}
	s.emotionController = block.controller
	s.emotionClock = block.clock
	if block.delivery != nil {
		r := block.delivery.reservation
		s.delivery = nil
		copy := s.settings
		s.host.emit(event("performance_notice", s.id, r.Request.TurnID, rendercontract.DeliveryNotice{RequestID: r.Request.RequestID, Token: r.Token, Status: "activated", Reservation: &r, Applied: &copy}))
	} else if block.applied != nil {
		copy := s.settings
		s.host.emit(event("performance_notice", s.id, "", rendercontract.DeliveryNotice{Revision: s.policyRevision, Status: "settings_applied", Applied: &copy}))
	}
	return block.decisions
}
