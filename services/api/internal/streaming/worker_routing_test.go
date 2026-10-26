package streaming

import (
	"context"
	"testing"
	"time"
)

func TestWorkerRoutingKeepsSessionsOnTheirAssignedPeer(t *testing.T) {
	first := &peer{ctx: context.Background(), out: make(chan Envelope, 2)}
	second := &peer{ctx: context.Background(), out: make(chan Envelope, 2)}
	s := &Service{sessions: map[string]*streamSession{
		"session_one": {id: "session_one", workerID: "allocation_one"},
		"session_two": {id: "session_two", workerID: "allocation_two"},
	}, workers: map[string]*cloudWorker{
		"allocation_one": {peer: first},
		"allocation_two": {peer: second},
	}}
	if !s.toWorker(envelope("stop", "session_one", nil)) || !s.toWorker(envelope("stop", "session_two", nil)) {
		t.Fatal("bound worker did not accept its command")
	}
	if (<-first.out).SessionID != "session_one" || (<-second.out).SessionID != "session_two" {
		t.Fatal("session command crossed worker boundary")
	}
	delete(s.workers, "allocation_one")
	if s.toWorker(envelope("stop", "session_one", nil)) {
		t.Fatal("disconnected worker command was rerouted")
	}
}

func TestCloudWorkerAssignmentRespectsTwoSessionCapacity(t *testing.T) {
	now := time.Now()
	sessions := map[string]*streamSession{}
	s := &Service{sessions: sessions, workers: map[string]*cloudWorker{
		"allocation_a": {started: now},
		"allocation_b": {started: now},
	}}
	for i, id := range []string{"one", "two", "three", "four"} {
		session := &streamSession{id: id}
		sessions[id] = session
		if !s.assignCloudWorkerLocked(session, now) {
			t.Fatal("available worker was skipped")
		}
		want := "allocation_a"
		if i >= 2 {
			want = "allocation_b"
		}
		if session.workerID != want {
			t.Fatalf("session %s assigned to %s, want %s", id, session.workerID, want)
		}
	}
	extra := &streamSession{id: "five"}
	sessions[extra.id] = extra
	if s.assignCloudWorkerLocked(extra, now) {
		t.Fatal("assigned above the measured two-session target")
	}
	s.workers["allocation_c"] = &cloudWorker{started: now.Add(-25 * time.Minute)}
	if s.assignCloudWorkerLocked(extra, now) {
		t.Fatal("assigned session to expiring container")
	}
}
