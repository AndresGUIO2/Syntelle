package streaming

import (
	"testing"
	"time"

	"syntelle/api/internal/conversation"
)

func TestInactivityUsesMessagesRatherThanTransport(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name    string
		message time.Time
		want    time.Time
	}{
		{"no message", time.Time{}, now.Add(2 * time.Minute)},
		{"first message", now.Add(10 * time.Second), now.Add(130 * time.Second)},
		{"later message", now.Add(100 * time.Second), now.Add(220 * time.Second)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := &streamSession{created: now, lastMessage: tc.message, lastSeen: now.Add(time.Hour)}
			if got := session.inactivityDeadline(); !got.Equal(tc.want) {
				t.Fatalf("deadline = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestInactivityCountsOnlyCompletedRecognizedMessages(t *testing.T) {
	session := &streamSession{id: "fixture_session", created: time.Now()}
	service := &Service{sessions: map[string]*streamSession{session.id: session}}
	for _, event := range []conversation.Event{
		{Type: "transcript_partial", Text: "unfinished"},
		{Type: "transcript_final", Text: "  "},
		{Type: "listening"},
		{Type: "speech_detected"},
		{Type: "reply_delta", TurnID: "obsolete", Text: "model output"},
	} {
		service.conversationEvent(session, event)
		if !session.lastMessage.IsZero() {
			t.Fatalf("%s kept the renderer alive", event.Type)
		}
	}
	before := time.Now()
	service.conversationEvent(session, conversation.Event{Type: "transcript_final", Text: "A recognized user message."})
	if session.lastMessage.Before(before) {
		t.Fatal("recognized user message did not reset inactivity")
	}
}

func TestInactivityStaleScanCannotPauseNewMessage(t *testing.T) {
	now := time.Now()
	session := &streamSession{id: "fixture_session", created: now.Add(-time.Minute), lastMessage: now}
	service := &Service{sessions: map[string]*streamSession{session.id: session}}
	// No store/worker is needed: refreshed activity must prevent any teardown.
	service.endSession(session.id, inactivityReason, now)
	if service.sessions[session.id] != session {
		t.Fatal("stale inactivity scan closed a freshly active session")
	}
}

func TestSessionTimeLimitCannotBeExtendedByMessages(t *testing.T) {
	now := time.Now()
	session := &streamSession{id: "fixture_session", deadline: now.Add(5 * time.Minute), lastMessage: now.Add(time.Hour)}
	service := &Service{sessions: map[string]*streamSession{session.id: session}}
	service.endSession(session.id, sessionLimitReason, now.Add(5*time.Minute-time.Nanosecond))
	if service.sessions[session.id] != session {
		t.Fatal("session ended before its fixed deadline")
	}
	if sessionTimeLimit != 5*time.Minute || visitorTimeLimit != 15*time.Minute {
		t.Fatal("unexpected demo policy")
	}
}
