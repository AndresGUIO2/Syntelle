package streaming

import (
	"syntelle/render-contract"
	"testing"
	"time"
)

func TestWorkerMoodRecoveryDropsUnplayedReservationsAndPreservesAge(t *testing.T) {
	s := &Service{}
	now := time.Now()
	old := &rendercontract.MoodCheckpoint{Version: "mood-checkpoint/1", Revision: 2, State: []byte(`{"age":60000}`)}
	ch := make(chan rendercontract.DeliveryNotice, 1)
	session := &streamSession{emotionalPresence: true, moodCheckpoint: old, emotionPolicyRevision: 2, requestedPolicyRevision: 2, currentTurn: "old_turn", deliveryWaiters: map[string]chan rendercontract.DeliveryNotice{"pending": ch}, reservedDelivery: &rendercontract.DeliveryReservation{Token: "unplayed"}}
	if !s.retainWorkerMood(session, now) || session.moodCheckpoint != old || session.currentTurn != "" || session.reservedDelivery != nil || session.workerLostAt != now {
		t.Fatal("lost committed mood or replayed output", session)
	}
	if notice := <-ch; notice.Status != "invalidated" {
		t.Fatal(notice)
	}
	session.currentTurn = "new_turn"
	session.requestedPolicyRevision = 3
	if !s.retainWorkerMood(session, now) || session.moodCheckpoint != nil || session.emotionPolicyRevision != 3 {
		t.Fatal("stale manual revision restored")
	}
}

func TestPersistentMoodHasTimeToCompleteInactivityRecovery(t *testing.T) {
	now := time.Now()
	session := &streamSession{emotionalPresence: true, created: now, lastMessage: now}
	if session.inactivityDeadline().Sub(now) < 4*time.Minute {
		t.Fatal("transport closes before mood recovery")
	}
	session.emotionalPresence = false
	if session.inactivityDeadline().Sub(now) != 2*time.Minute {
		t.Fatal("legacy inactivity changed")
	}
}
