package streaming

import "time"

const inactivityReason = "user inactivity"

// Caller holds Service.mu. Transport traffic never changes this deadline.
func (session *streamSession) inactivityDeadline() time.Time {
	if session.lastMessage.IsZero() {
		return session.created.Add(2 * time.Minute)
	}
	return session.lastMessage.Add(2 * time.Minute)
}

// The final check under the lock prevents an old scan from pausing a new message.
func (s *Service) expireAt(now time.Time) {
	s.mu.Lock()
	var inactive, disconnected, expired []string
	for _, session := range s.sessions {
		if !session.deadline.IsZero() && !now.Before(session.deadline) {
			expired = append(expired, session.id)
		} else if !now.Before(session.inactivityDeadline()) {
			inactive = append(inactive, session.id)
		} else if now.Sub(session.lastSeen) > 30*time.Second {
			disconnected = append(disconnected, session.id)
		}
	}
	s.mu.Unlock()
	for _, id := range expired {
		s.endSession(id, sessionLimitReason, now)
	}
	for _, id := range inactive {
		s.endSession(id, inactivityReason, now)
	}
	for _, id := range disconnected {
		s.closeSession(id, "connection heartbeat expired")
	}
}
