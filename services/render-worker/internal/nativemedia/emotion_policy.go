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
	s.emotionController = block.controller
	s.emotionClock = block.clock
	return block.decisions
}
