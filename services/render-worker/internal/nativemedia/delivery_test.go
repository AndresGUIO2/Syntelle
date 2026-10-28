package nativemedia

import (
	"context"
	"encoding/json"
	"syntelle/render-contract"
	"testing"
	"time"
)

func deliveryFixture(t *testing.T) (*session, *Host, *[]Envelope) {
	t.Helper()
	ctx := context.Background()
	messages := []Envelope{}
	h := &Host{ctx: ctx, sessions: map[string]*session{}, config: Config{Emit: func(e Envelope) { messages = append(messages, e) }}}
	s := queuedSession()
	s.host, s.ctx, s.id, s.turn, s.emotionInstance, s.policyRevision = h, ctx, "stream_fixture", "turn_fixture", "instance_fixture", 1
	s.scene = emotionScene{}
	h.sessions[s.id] = s
	return s, h, &messages
}

func reserveFixture(t *testing.T, s *session, h *Host, index int) rendercontract.DeliveryReservation {
	t.Helper()
	r := rendercontract.DeliveryRequest{RequestID: "request_fixture", TurnID: s.turn, PhraseIndex: index, Revision: 1, TextHash: rendercontract.TextHash("Hello."), Voice: rendercontract.DefaultVoice, Language: "en-US", Purpose: "reassure", Emotion: &rendercontract.EmotionTarget{State: "affectionate", Intensity: .3, Energy: .4}}
	if err := h.Dispatch(context.Background(), Envelope{Type: "performance_preflight", SessionID: s.id, Data: conversationData(t, r)}); err != nil {
		t.Fatal(err)
	}
	return rendercontract.DeliveryReservation{}
}

func TestDeliveryReservationDoesNotBecomePlayedState(t *testing.T) {
	s, h, messages := deliveryFixture(t)
	reserveFixture(t, s, h, 0)
	if len(*messages) != 1 {
		t.Fatal("missing reservation response")
	}
	var n rendercontract.DeliveryNotice
	if json.Unmarshal((*messages)[0].Data, &n) != nil || n.Status != "reserved" || !n.Reservation.Valid() {
		t.Fatal(string((*messages)[0].Data))
	}
	if s.emotionController.Sequence != 0 || s.emotionClock != 0 {
		t.Fatal("reservation changed played state")
	}
	p := preparedFixture()
	p.DurationMS = 40
	p.Audio = testWAV(audioSamples * 2)
	p.Visemes = []Viseme{{ID: 1}, {ID: 0, OffsetMS: 40}}
	praw := map[string]any{"audio_base64": p.Audio, "duration_ms": p.DurationMS, "visemes": p.Visemes, "delivery": n.Reservation}
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_part", "turn_id": s.turn, "data": map[string]any{"index": 0, "text": "Hello.", "speech": praw}})); err != nil {
		t.Fatal(err)
	}
	block := s.nextAudioBlock(time.Now(), make([]int16, audioSamples))
	if s.emotionController.Sequence != 0 {
		t.Fatal("unsent audio committed emotion")
	}
	if got := s.commitEmotions(block, false); len(got) != 0 {
		t.Fatal("failed send activated direction")
	}
}

func TestDeliveryActivatesOnlyAfterSendAndStagesManualEmotion(t *testing.T) {
	s, h, messages := deliveryFixture(t)
	reserveFixture(t, s, h, 0)
	var n rendercontract.DeliveryNotice
	if err := json.Unmarshal((*messages)[0].Data, &n); err != nil {
		t.Fatal(err)
	}
	p := preparedFixture()
	p.Audio = testWAV(audioSamples * 4)
	p.DurationMS = 80
	p.Visemes = []Viseme{{ID: 1}, {ID: 0, OffsetMS: 80}}
	p.Delivery = n.Reservation
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_part", "turn_id": s.turn, "data": speechPart{Speech: p, Index: 0, Text: "Hello."}})); err != nil {
		t.Fatal(err)
	}
	block := s.nextAudioBlock(time.Now(), make([]int16, audioSamples))
	s.commitEmotions(block, true)
	if s.delivery != nil || s.emotionController.Motion.Effective != n.Reservation.Effective {
		t.Fatal("approved target did not activate exactly")
	}
	var ack rendercontract.DeliveryNotice
	if err := json.Unmarshal((*messages)[len(*messages)-1].Data, &ack); err != nil || ack.Status != "activated" {
		t.Fatal("missing post-send activation", ack, err)
	}
	before := s.settings
	next := before
	next.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: 1}
	if err := s.configure(conversationData(t, map[string]any{"settings": next, "emotion_policy_revision": 2})); err != nil {
		t.Fatal(err)
	}
	if s.settings.EmotionBaseline != before.EmotionBaseline || s.requestedSettings == nil || s.requestedRevision != 2 {
		t.Fatal("manual edit changed the audible phrase")
	}
	r := rendercontract.DeliveryRequest{RequestID: "next_request", TurnID: s.turn, PhraseIndex: 1, Revision: 2, TextHash: rendercontract.TextHash("Next."), Voice: rendercontract.DefaultVoice, Language: "en-US", Purpose: "playful"}
	if err := s.preflight(conversationData(t, r)); err != nil {
		t.Fatal(err)
	}
	if s.delivery == nil || s.delivery.reservation.Effective.State != "playful" {
		t.Fatal("next phrase did not approve requested mood")
	}
}

func TestDeliveryInvalidationRejectsOldAudioAndDoesNotSpendState(t *testing.T) {
	s, h, messages := deliveryFixture(t)
	reserveFixture(t, s, h, 0)
	var n rendercontract.DeliveryNotice
	_ = json.Unmarshal((*messages)[0].Data, &n)
	before := s.emotionController
	next := s.settings
	next.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: .5}
	if err := s.configure(conversationData(t, map[string]any{"settings": next, "emotion_policy_revision": 2})); err != nil {
		t.Fatal(err)
	}
	p := preparedFixture()
	p.Delivery = n.Reservation
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_part", "turn_id": s.turn, "data": speechPart{Speech: p, Text: "Hello."}})); err != nil {
		t.Fatal(err)
	}
	if len(s.queue) != 0 || s.emotionController != before {
		t.Fatal("stale preparation activated or spent state")
	}
}

func TestDeliveryDuplicatePauseExpiryAndAccessibility(t *testing.T) {
	for _, mode := range []string{"duplicate", "paused", "expired", "accessibility"} {
		t.Run(mode, func(t *testing.T) {
			s, h, messages := deliveryFixture(t)
			reserveFixture(t, s, h, 0)
			var notice rendercontract.DeliveryNotice
			if err := json.Unmarshal((*messages)[0].Data, &notice); err != nil {
				t.Fatal(err)
			}
			token := s.delivery.reservation.Token
			switch mode {
			case "duplicate":
				if err := s.preflight(conversationData(t, notice.Reservation.Request)); err != nil {
					t.Fatal(err)
				}
				if s.delivery.reservation.Token != token {
					t.Fatal("duplicate reservation spent a second token")
				}
			case "paused":
				s.paused = true
				for i := 0; i < 10; i++ {
					b := s.nextAudioBlock(time.Now(), make([]int16, audioSamples))
					s.commitEmotions(b, true)
				}
				if s.emotionClock != 0 || s.delivery.reservation.Token != token {
					t.Fatal("pause advanced the lease")
				}
			case "expired":
				s.emotionClock = s.delivery.expires + 1
				if err := s.preflight(conversationData(t, notice.Reservation.Request)); err != nil {
					t.Fatal(err)
				}
				if s.delivery != nil {
					t.Fatal("expired reservation renewed itself")
				}
			case "accessibility":
				next := s.settings
				next.ReducedMotion = true
				if err := s.configure(conversationData(t, map[string]any{"settings": next, "emotion_policy_revision": 2})); err != nil {
					t.Fatal(err)
				}
				if !s.settings.ReducedMotion || s.delivery != nil || s.requestedSettings != nil {
					t.Fatal("accessibility did not override pending delivery")
				}
			}
		})
	}
}

func TestDeliveryRejectsFuturePhraseWithoutReserving(t *testing.T) {
	s, h, messages := deliveryFixture(t)
	reserveFixture(t, s, h, 1)
	var n rendercontract.DeliveryNotice
	if err := json.Unmarshal((*messages)[0].Data, &n); err != nil {
		t.Fatal(err)
	}
	if s.delivery != nil || n.Reason != "out_of_order_phrase" {
		t.Fatal(n)
	}
}

func TestDeliverySettingsRoundTripCannotReviveOldPreparation(t *testing.T) {
	s, h, messages := deliveryFixture(t)
	reserveFixture(t, s, h, 0)
	var initial rendercontract.DeliveryNotice
	_ = json.Unmarshal((*messages)[0].Data, &initial)
	a := s.settings
	b := a
	b.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: 1}
	for i, next := range []rendercontract.Settings{b, a} {
		if err := s.configure(conversationData(t, map[string]any{"settings": next, "emotion_policy_revision": i + 2})); err != nil {
			t.Fatal(err)
		}
	}
	if s.requestedRevision != 3 || s.delivery != nil {
		t.Fatal("old reservation survived A-B-A")
	}
	r := initial.Reservation.Request
	r.Revision = 3
	r.RequestID = "revision_three"
	if err := s.preflight(conversationData(t, r)); err != nil {
		t.Fatal(err)
	}
	if s.delivery == nil || s.delivery.reservation.Token == initial.Token || s.delivery.reservation.Request.Revision != 3 {
		t.Fatal("new revision reused old preparation")
	}
	if s.deliveryMatchesLocked(initial.Reservation, "Hello.", 0) {
		t.Fatal("old audio admitted after settings round trip")
	}
}

func TestDeliveryConfigurationWaitsForFirstAudioTransaction(t *testing.T) {
	s, h, _ := deliveryFixture(t)
	reserveFixture(t, s, h, 0)
	next := s.settings
	next.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: .5}
	raw := conversationData(t, map[string]any{"settings": next, "emotion_policy_revision": 2})
	s.sendMu.Lock()
	started, done := make(chan struct{}), make(chan error, 1)
	go func() { close(started); done <- s.configure(raw) }()
	<-started
	select {
	case <-done:
		s.sendMu.Unlock()
		t.Fatal("settings crossed the audio send transaction")
	case <-time.After(10 * time.Millisecond):
	}
	s.sendMu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if s.requestedRevision != 2 || s.delivery != nil {
		t.Fatal("configuration was lost after send transaction")
	}
}
