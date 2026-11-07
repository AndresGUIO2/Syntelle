package nativemedia

import "syntelle/render-contract"

// Emit only committed live state, after successful audio send/commit and its
// emotion decisions. At most one periodic checkpoint per presentation second.
func (s *session) publishMood(force bool) {
	s.mu.Lock()
	if !s.emotionController.Persistent || (!force && s.emotionClock-s.lastMoodClock < 1000) {
		s.mu.Unlock()
		return
	}
	s.lastMoodClock = s.emotionClock
	s.moodSequence++
	hash := s.packageHash
	if hash == "" {
		hash = s.host.config.PackageHash
	}
	value := rendercontract.MoodCheckpoint{Version: "mood-checkpoint/1", SessionID: s.id, PackageHash: hash, Instance: s.emotionInstance, Revision: s.policyRevision, Sequence: s.moodSequence, State: s.emotionController.MoodMemory(s.emotionClock)}
	s.mu.Unlock()
	s.host.emit(event("mood_checkpoint", s.id, "", value))
}
