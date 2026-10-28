package conversation

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestCachedLaughterKeepsPCMAndEnforcesReplyPolicy(t *testing.T) {
	wav := performanceFixtureWAV()
	laughWAV := append([]byte(nil), wav[:44+48000]...)
	binary.LittleEndian.PutUint32(laughWAV[4:], uint32(len(laughWAV)-8))
	binary.LittleEndian.PutUint32(laughWAV[40:], 48000)
	sample := cachedSpeech{Audio: laughWAV, Duration: 1000, Voice: "fixture-voice", Language: "en-US", Visemes: []cachedViseme{{ID: 0, Offset: 0}, {ID: 1, Offset: 200}}}
	raw, _ := json.Marshal(sample)
	service := &Service{Performance: &PerformanceProfile{Laughter: map[string]json.RawMessage{"fixture-voice|en-US": raw}}}
	amount := .8
	direction := PerformanceValidation{Resolved: []ResolvedCue{{Kind: "laugh", Time: 9.08, Duration: 1, Amount: &amount}}}
	originalHash := string(wav)
	service.beginPerformance("session", "reply-one")
	payload := map[string]any{"audio_base64": wav, "duration_ms": float64(9000), "visemes": sample.Visemes}
	got, err := service.appendLaughter("session", "reply-one", "fixture-voice", "en-US", payload, direction)
	if err != nil || len(got.Resolved) != 1 || got.Resolved[0].Time != 9.08 || got.Resolved[0].Duration != 1 {
		t.Fatal(got, err)
	}
	_, pcm, err := pcm16(payload["audio_base64"].([]byte))
	if err != nil {
		t.Fatal(err)
	}
	_, original, _ := pcm16(wav)
	if string(pcm[:len(original)]) != string(original) || string(pcm[len(pcm)-48000:]) != string(laughWAV[44:]) || string(wav) != originalHash {
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

func TestFixedLaughterBankUsesCompleteBoundAction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount float64
	}{{"omit zero", 0}, {"quiet intention", .2}, {"half intention", .5}, {"full intention", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			wav := append([]byte(nil), performanceFixtureWAV()[:44+48000]...)
			binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
			binary.LittleEndian.PutUint32(wav[40:], 48000)
			raw, _ := json.Marshal(cachedSpeech{Audio: wav, Duration: 1000, Voice: "voice", Language: "en-US", Visemes: []cachedViseme{{0, 0}, {0, 800}}})
			s := &Service{Performance: &PerformanceProfile{Laughter: map[string]json.RawMessage{"voice|en-US": raw}}}
			s.beginPerformance("s", "r")
			p := map[string]any{"audio_base64": wav, "duration_ms": float64(1000)}
			v, err := s.appendLaughter("s", "r", "voice", "en-US", p, PerformanceValidation{Resolved: []ResolvedCue{{Kind: "laugh", Amount: &tc.amount}}})
			if err != nil {
				t.Fatal(err)
			}
			if tc.amount == 0 {
				if len(v.Resolved) != 0 || len(p["audio_base64"].([]byte)) != len(wav) {
					t.Fatal("zero laughter appended/spent a full recording")
				}
				return
			}
			if *v.Resolved[0].Amount != 1 {
				t.Fatal("complete recording received partial face", *v.Resolved[0].Amount)
			}
			if p["laughter_span"] == nil {
				t.Fatal("approved laughter has no immutable PCM span")
			}
		})
	}
}

func TestZeroLaughterDoesNotSpendReplyAllowance(t *testing.T) {
	zero, positive := 0., .2
	b := newReplyGestures()
	got := b.admit(PerformanceValidation{Resolved: []ResolvedCue{{Kind: "laugh", Amount: &zero}}}, 0)
	if len(got.Resolved) != 0 || b.laughed {
		t.Fatal("zero strength spent reply laughter allowance")
	}
	got = b.admit(PerformanceValidation{Resolved: []ResolvedCue{{Kind: "laugh", Amount: &positive}}}, 1)
	if len(got.Resolved) != 1 {
		t.Fatal("positive request was blocked by previous zero request")
	}
	intent := PerformanceIntent{Version: "performance-intent/4", Text: "A playful thought", Tone: "playful", Purpose: "playful", Cues: []PerformanceCue{{Kind: "laugh", Anchor: "playful", Amount: &zero}}}
	if got := ValidatePerformance(intent, intent.Text); len(got.Intent.Cues) != 0 {
		t.Fatal("zero strength survived direction validation")
	}
}
