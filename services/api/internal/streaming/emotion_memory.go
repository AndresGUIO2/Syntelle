package streaming

import (
	"context"
	"syntelle/api/internal/conversation"
	"syntelle/api/internal/httpapi"
	"syntelle/render-contract"
	"time"
)

// Caller owns Service.mu. Only qualified local sessions retain mood during a
// worker replacement; cloud leases and legacy disconnection behavior stay intact.
func (s *Service) retainWorkerMood(session *streamSession, now time.Time) bool {
	if s.closing || httpapi.Cloud() || !session.emotionalPresence || session.moodCheckpoint == nil {
		return false
	}
	session.workerLostAt = now
	session.workerLostTurn = ""
	if session.state == "speaking" || session.state == "preparing" {
		session.workerLostTurn = session.currentTurn
		if session.workerLostTurn == "" {
			session.workerLostTurn = session.pendingTurn
		}
	}
	ignoreTurn(session, session.currentTurn)
	ignoreTurn(session, session.pendingTurn)
	session.currentTurn = ""
	session.pendingTurn = ""
	session.reservedDelivery = nil
	for id, ch := range session.deliveryWaiters {
		select {
		case ch <- rendercontract.DeliveryNotice{RequestID: id, Status: "invalidated", Reason: "worker_replaced"}:
		default:
		}
	}
	session.state = "starting"
	if session.requestedPolicyRevision > session.emotionPolicyRevision {
		session.emotionPolicyRevision = session.requestedPolicyRevision
		copy := session.settings
		session.appliedSettings = &copy
		session.moodCheckpoint = nil
	}
	return true
}

func (s *Service) cancelReplacedOutput(session *streamSession) {
	if session.channel == nil || session.workerLostTurn == "" {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()
	_ = session.channel.Send(ctx, conversation.Event{Type: "cancel", ConversationID: session.cid, TurnID: session.workerLostTurn, Reason: "worker replacement"})
}
