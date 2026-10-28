package streaming

import (
	"context"
	"syntelle/render-contract"
	"testing"
)

func TestLegacyWorkerDoesNotReceiveNewPrivateCommands(t *testing.T) {
	p := &peer{ctx: context.Background(), out: make(chan Envelope, 2)}
	s := &Service{worker: p}
	if !s.toWorker(envelope("activity", "session_fixture", map[string]string{"state": "preparing"})) || len(p.out) != 0 {
		t.Fatal("legacy worker received unknown activity command")
	}
	if s.toWorker(envelope("performance_preflight", "session_fixture", nil)) {
		t.Fatal("unsupported preflight was sent before synthesis")
	}
}

func TestDeliveryAcknowledgementRejectsStaleIdentityAndDuplicates(t *testing.T) {
	session := &streamSession{id: "session_fixture", currentTurn: "turn_fixture", requestedPolicyRevision: 1, deliveryWaiters: map[string]chan rendercontract.DeliveryNotice{"request_fixture": make(chan rendercontract.DeliveryNotice, 4)}}
	service := &Service{sessions: map[string]*streamSession{session.id: session}}
	r := rendercontract.DeliveryReservation{Version: rendercontract.DeliveryVersion, Token: "token_fixture", Instance: "worker_fixture", Generation: 1, Effective: rendercontract.EmotionTarget{State: "neutral", Energy: .5}, Request: rendercontract.DeliveryRequest{RequestID: "request_fixture", TurnID: session.currentTurn, Revision: 1, Voice: rendercontract.DefaultVoice, Language: "en-US", Purpose: "explain", TextHash: rendercontract.TextHash("Hello.")}}
	if !service.deliveryNotice(session, rendercontract.DeliveryNotice{Status: "reserved", RequestID: r.Request.RequestID, Reservation: &r}) {
		t.Fatal("reservation was not admitted")
	}
	settings := rendercontract.Defaults()
	stale := r
	stale.Instance = "replaced_worker"
	if service.deliveryNotice(session, rendercontract.DeliveryNotice{Status: "activated", Token: r.Token, Reservation: &stale, Applied: &settings}) {
		t.Fatal("stale worker activated a reservation")
	}
	notice := rendercontract.DeliveryNotice{Status: "activated", RequestID: r.Request.RequestID, Token: r.Token, Reservation: &r, Applied: &settings}
	if !service.deliveryNotice(session, notice) {
		t.Fatal("matching activation rejected")
	}
	if service.deliveryNotice(session, notice) {
		t.Fatal("duplicate activation committed twice")
	}
}
