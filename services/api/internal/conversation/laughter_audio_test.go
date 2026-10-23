package conversation

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestCachedLaughterKeepsPCMAndEnforcesReplyPolicy(t *testing.T) {
	wav := performanceFixtureWAV()
	sample := cachedSpeech{Audio: wav, Duration: 9000, Voice: "fixture-voice", Language: "en-US", Visemes: []cachedViseme{{ID: 0, Offset: 0}, {ID: 1, Offset: 200}}}
	raw, _ := json.Marshal(sample)
	service := &Service{Performance: &PerformanceProfile{Laughter: map[string]json.RawMessage{"fixture-voice|en-US": raw}}}
	amount := .8
	direction := PerformanceValidation{Resolved: []ResolvedCue{{Kind: "laugh", Time: 9.08, Duration: 1, Amount: &amount}}}
	originalHash := string(wav)
	service.beginPerformance("session", "reply-one")
	payload := map[string]any{"audio_base64": wav, "duration_ms": float64(9000), "visemes": sample.Visemes}
	got, err := service.appendLaughter("session", "reply-one", "fixture-voice", "en-US", payload, direction)
	if err != nil || len(got.Resolved) != 1 || got.Resolved[0].Time != 9.08 || got.Resolved[0].Duration != 9 {
		t.Fatal(got, err)
	}
	_, pcm, err := pcm16(payload["audio_base64"].([]byte))
	if err != nil {
		t.Fatal(err)
	}
	_, original, _ := pcm16(wav)
	if string(pcm[:len(original)]) != string(original) || string(pcm[len(pcm)-len(original):]) != string(original) || string(wav) != originalHash {
		t.Fatal("provider PCM was modified")
	}
	service.beginPerformance("session", "reply-two")
	denied, err := service.appendLaughter("session", "reply-two", "fixture-voice", "en-US", map[string]any{"audio_base64": wav}, direction)
	if err != nil || len(denied.Resolved) != 0 {
		t.Fatal("consecutive/cooldown laugh admitted", denied, err)
	}
}

func TestIncompatibleOptionalLaughterPreservesSpokenPCM(t *testing.T) {
	wav := performanceFixtureWAV()
	other := append([]byte(nil), wav...)
	binary.LittleEndian.PutUint32(other[24:28], 16000)
	raw, _ := json.Marshal(cachedSpeech{Audio: other, Voice: "voice", Language: "en-US"})
	s := &Service{Performance: &PerformanceProfile{Laughter: map[string]json.RawMessage{"voice|en-US": raw}}}
	s.beginPerformance("session", "reply")
	payload := map[string]any{"audio_base64": wav, "duration_ms": float64(9000)}
	v, err := s.appendLaughter("session", "reply", "voice", "en-US", payload, PerformanceValidation{Resolved: []ResolvedCue{{Kind: "laugh"}}})
	if err != nil || len(v.Resolved) != 0 || string(payload["audio_base64"].([]byte)) != string(wav) {
		t.Fatal("optional laughter damaged speech", err)
	}
}

func TestUnsupportedExpressionCannotReachPlayback(t *testing.T) {
	service := &Service{}
	got := service.filterCapabilities(PerformanceValidation{Resolved: []ResolvedCue{{Kind: "laugh"}, {Kind: "smile"}, {Kind: "brows"}}})
	if len(got.Resolved) != 1 || got.Resolved[0].Kind != "brows" {
		t.Fatal(got)
	}
}
