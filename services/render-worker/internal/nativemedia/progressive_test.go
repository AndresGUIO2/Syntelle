package nativemedia

import (
	"encoding/json"
	"slices"
	"syntelle/render-contract"
	"testing"
	"time"
)

func prefixFixture(t *testing.T) (*session, func(int, bool) json.RawMessage) {
	t.Helper()
	s, h, messages := deliveryFixture(t)
	reserveFixture(t, s, h, 0)
	var notice rendercontract.DeliveryNotice
	if err := json.Unmarshal((*messages)[0].Data, &notice); err != nil {
		t.Fatal(err)
	}
	return s, func(samples int, final bool) json.RawMessage {
		p := preparedSpeech{Audio: testWAV(samples), DurationMS: float64(samples) * 1000 / sampleRate, Visemes: []Viseme{{ID: 1}, {ID: 2, OffsetMS: 300}}, Delivery: notice.Reservation, Performance: json.RawMessage(`{"resolved":[],"intent":{"cues":[]}}`)}
		return conversationData(t, map[string]any{"type": "speech_prefix", "turn_id": s.turn, "data": speechPart{Speech: p, Text: "Hello.", Index: 0, Final: final}})
	}
}

func TestProgressiveOpeningPlaysBeforeCompletionWithoutRepeating(t *testing.T) {
	s, prefix := prefixFixture(t)
	if err := s.conversation(prefix(sampleRate, false)); err != nil {
		t.Fatal(err)
	}
	var heard []int16
	pcm := make([]int16, audioSamples)
	for range 20 {
		block := s.nextAudioBlock(time.Now(), pcm)
		s.commitEmotions(block, true)
		heard = append(heard, pcm...)
	}
	if s.position != 9600 || s.stream.streamFinal || s.delivery != nil {
		t.Fatal("opening did not play before synthesis completion")
	}
	if err := s.conversation(prefix(sampleRate*2, true)); err != nil {
		t.Fatal(err)
	}
	for range 80 {
		block := s.nextAudioBlock(time.Now(), pcm)
		s.commitEmotions(block, true)
		heard = append(heard, pcm...)
	}
	want, err := decodeWAV(testWAV(sampleRate * 2))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(heard, want) || s.playing != nil || s.stream != nil || s.nextIndex != 1 || s.queuedBytes != 0 {
		t.Fatal("stream repeated, lost or changed native PCM")
	}
	if err := s.conversation(prefix(sampleRate*2, true)); err != nil {
		t.Fatal(err)
	}
	s.nextAudioBlock(time.Now(), pcm)
	if !slices.Equal(pcm, make([]int16, audioSamples)) {
		t.Fatal("duplicate completion replayed audio")
	}
}

func TestProgressiveOpeningStallFreezesAndCancelDiscards(t *testing.T) {
	s, prefix := prefixFixture(t)
	first := prefix(sampleRate, false)
	if err := s.conversation(first); err != nil {
		t.Fatal(err)
	}
	pcm := make([]int16, audioSamples)
	for range 40 {
		block := s.nextAudioBlock(time.Now(), pcm)
		s.commitEmotions(block, true)
	}
	position := s.position
	for range 5 {
		s.nextAudioBlock(time.Now(), pcm)
	}
	if position != sampleRate*4/5 || s.position != position || !slices.Equal(pcm, make([]int16, audioSamples)) {
		t.Fatal("provider stall consumed unavailable audio")
	}
	s.interrupt()
	if err := s.conversation(first); err != nil {
		t.Fatal(err)
	}
	if s.playing != nil || s.stream != nil || len(s.queue) != 0 || s.queuedBytes != 0 {
		t.Fatal("cancelled prefix survived")
	}
}

func TestProgressiveOpeningRejectsChangedPrefix(t *testing.T) {
	s, prefix := prefixFixture(t)
	if err := s.conversation(prefix(sampleRate, false)); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type string     `json:"type"`
		Turn string     `json:"turn_id"`
		Data speechPart `json:"data"`
	}
	if err := json.Unmarshal(prefix(sampleRate*2, true), &event); err != nil {
		t.Fatal(err)
	}
	event.Data.Speech.Audio[44] ^= 1
	if err := s.conversation(conversationData(t, event)); err == nil {
		t.Fatal("changed PCM prefix accepted")
	}
}

func TestProgressiveOpeningRequiresFinalRecordingBeforeTurnCompletion(t *testing.T) {
	s, prefix := prefixFixture(t)
	if err := s.conversation(prefix(sampleRate, false)); err != nil {
		t.Fatal(err)
	}
	if err := s.conversation(conversationData(t, map[string]any{"type": "speech_complete", "turn_id": s.turn, "data": map[string]int{"parts": 1}})); err == nil {
		t.Fatal("truncated stream accepted as complete")
	}
}
