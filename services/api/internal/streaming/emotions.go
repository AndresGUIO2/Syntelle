package streaming

import "syntelle/render-contract"

// Private worker only; configuration epochs prevent A -> B -> A from reviving
// delayed evidence. Clock recovery is valid after the reply has ended.
func admitEmotionDecision(s *streamSession, d rendercontract.EmotionDecision) bool {
	if d.Version != rendercontract.EmotionPolicyVersion || d.Instance == "" || d.PolicyRevision != s.emotionPolicyRevision || d.Baseline.Normalized() != appliedSettings(s).EmotionBaseline.Normalized() || d.Regulation.Normalized() != appliedSettings(s).EmotionRegulation.Normalized() || !d.Effective.Valid() {
		return false
	}
	if d.PhraseIndex >= 0 && (d.PhraseIndex >= 1048576 || d.TurnID == "" || d.TurnID != s.currentTurn) {
		return false
	}
	if d.PhraseIndex < 0 && d.Reason != "clock_transition" && d.Reason != "configuration" && d.Reason != "reply_ended" {
		return false
	}
	if d.Instance != s.emotionInstance {
		if d.Sequence != 1 || d.Reason != "configuration" {
			return false
		}
		for _, id := range s.retiredEmotionInstances {
			if id == d.Instance {
				return false
			}
		}
		if s.emotionInstance != "" {
			s.retiredEmotionInstances = append(s.retiredEmotionInstances, s.emotionInstance)
		}
		s.emotionInstance = d.Instance
		s.lastEmotionDecisionSequence = 0
		s.lastEmotionGeneration = 0
	}
	if d.Sequence <= s.lastEmotionDecisionSequence || d.Generation < s.lastEmotionGeneration {
		return false
	}

	s.lastEmotionDecisionSequence = d.Sequence
	s.lastEmotionGeneration = d.Generation
	return true
}

// Caller holds Service.mu; a private event can never resurrect a superseded
// turn or make an older phrase overwrite a newer activated state.
func admitEmotionActivation(s *streamSession, value rendercontract.EmotionActivation) bool {
	if appliedSettings(s).EmotionRegulation.Normalized().Enabled {
		return false
	}
	if value.TurnID == "" || value.TurnID != s.currentTurn || value.Baseline.Normalized() != appliedSettings(s).EmotionBaseline.Normalized() || !value.Emotion.Valid() || value.PhraseIndex < 0 || value.PhraseIndex >= 1048576 {
		return false
	}
	if value.Generation < s.lastEmotionGeneration {
		return false
	}
	if value.TurnID == s.lastEmotionTurn && value.Generation == s.lastEmotionGeneration && value.PhraseIndex <= s.lastEmotionPhrase {
		return false
	}
	s.lastEmotionTurn = value.TurnID
	s.lastEmotionPhrase = value.PhraseIndex
	s.lastEmotionGeneration = value.Generation
	return true
}
