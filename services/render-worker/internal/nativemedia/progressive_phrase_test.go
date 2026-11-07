package nativemedia

import (
	"encoding/json"
	"slices"
	"syntelle/render-contract"
	"testing"
	"time"
)

func latestReservation(t *testing.T, messages []Envelope, index int) *rendercontract.DeliveryReservation {
	t.Helper()
	for i := len(messages) - 1; i >= 0; i-- {
		var n rendercontract.DeliveryNotice
		if json.Unmarshal(messages[i].Data, &n) == nil && n.Status == "reserved" && n.Reservation != nil && n.Reservation.Request.PhraseIndex == index {
			return n.Reservation
		}
	}
	t.Fatal("missing reservation", index)
	return nil
}
func phrasePrefix(t *testing.T, s *session, r *rendercontract.DeliveryReservation, index, samples int, offset float64, final bool) json.RawMessage {
	t.Helper()
	p := preparedSpeech{Audio: testWAV(samples), DurationMS: float64(samples) * 1000 / sampleRate, Visemes: []Viseme{{ID: 1}, {ID: 2, OffsetMS: 300}}, Delivery: r, Performance: json.RawMessage(`{"resolved":[],"intent":{"cues":[]}}`)}
	return conversationData(t, map[string]any{"type": "speech_prefix", "turn_id": s.turn, "data": speechPart{Speech: p, Text: "Hello.", Index: index, OffsetMS: offset, Final: final}})
}
func TestProgressiveNextPhraseQueuesDuringPreviousPlayback(t *testing.T) {
	s, h, messages := deliveryFixture(t)
	reserveFixture(t, s, h, 0)
	r0 := latestReservation(t, *messages, 0)
	if err := s.conversation(phrasePrefix(t, s, r0, 0, sampleRate, 0, false)); err != nil {
		t.Fatal(err)
	}
	pcm := make([]int16, audioSamples)
	heard := []int16{}
	play := func(n int) {
		for range n {
			block := s.nextAudioBlock(time.Now(), pcm)
			s.commitEmotions(block, true)
			heard = append(heard, pcm...)
		}
	}
	play(20)
	reserveFixture(t, s, h, 1)
	r1 := latestReservation(t, *messages, 1)
	if err := s.conversation(phrasePrefix(t, s, r0, 0, sampleRate*2, 0, true)); err != nil {
		t.Fatal("predecessor completion blocked by next reservation", err)
	}
	if err := s.conversation(phrasePrefix(t, s, r1, 1, sampleRate*3/5, 2000, false)); err != nil {
		t.Fatal("second phrase streaming rejected", err)
	}
	play(100)
	if s.position != sampleRate*2/5 {
		t.Fatal("next phrase did not start with retained lookahead", s.position)
	}
	if err := s.conversation(phrasePrefix(t, s, r1, 1, sampleRate, 2000, true)); err != nil {
		t.Fatal(err)
	}
	play(30)
	a, _ := decodeWAV(testWAV(sampleRate * 2))
	b, _ := decodeWAV(testWAV(sampleRate))
	want := append(a, b...)
	if !slices.Equal(heard, want) || s.queuedBytes != 0 {
		t.Fatal("phrase seam repeated, padded, lost or retained PCM", len(heard), len(want), s.queuedBytes)
	}
}
func TestProgressiveSixHundredMillisecondsStartsWithoutFinal(t *testing.T) {
	s, prefix := prefixFixture(t)
	if err := s.conversation(prefix(sampleRate*3/5, false)); err != nil {
		t.Fatal("600ms prefix rejected", err)
	}
	block := s.nextAudioBlock(time.Now(), make([]int16, audioSamples))
	s.commitEmotions(block, true)
	if !s.started || s.position != audioSamples {
		t.Fatal("usable prefix waited for full synthesis")
	}
}

func TestProgressiveResolvedIdentityCannotBeRetimed(t *testing.T) {
	a := json.RawMessage(`{"intent":{"text":"A point"},"motion":{"reply_id":"r"},"motion_direction":"semantic-anchors/1","resolved":[{"id":"r:0:0","time":0.4}]}`)
	b := json.RawMessage(`{"intent":{"text":"A point"},"motion":{"reply_id":"r"},"motion_direction":"semantic-anchors/1","resolved":[{"id":"r:0:0","time":0.5}]}`)
	if samePrefixIntent(a, b) {
		t.Fatal("admitted cue moved on replacement")
	}
	b = json.RawMessage(`{"intent":{"text":"A point"},"motion":{"reply_id":"r"},"motion_direction":"semantic-anchors/1","resolved":[{"id":"r:0:0","time":0.4},{"id":"r:0:1","time":1.2}]}`)
	if !samePrefixIntent(a, b) {
		t.Fatal("fresh future cue prevented prefix growth")
	}
}
