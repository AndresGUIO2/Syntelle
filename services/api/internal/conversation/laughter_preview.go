package conversation

import (
	"encoding/binary"
	"encoding/json"
	"errors"
)

// LaughterPreview reads only the reviewed recording bank. It never invokes the
// model, speech provider, conversation history or conversational laugh allowance.
func (s *Service) LaughterPreview(voice, language string) (any, error) {
	if s.Performance == nil {
		return nil, errors.New("No reviewed laughter recording is available.")
	}
	raw, ok := s.Performance.Laughter[voice+"|"+language]
	if !ok {
		return nil, errors.New("No saved laughter for this voice and language. Select Ava to preview laughter.")
	}
	var cached cachedSpeech
	if json.Unmarshal(raw, &cached) != nil || cached.Voice != voice || cached.Language != language {
		return nil, errors.New("Saved laughter identity is invalid.")
	}
	format, pcm, err := pcm16(cached.Audio)
	if err != nil {
		return nil, err
	}
	duration := float64(len(pcm)) / float64(binary.LittleEndian.Uint32(format[4:])*2)
	if duration <= 0 || duration > 2.5 {
		return nil, errors.New("Saved laughter duration exceeds its prepared limit.")
	}
	var payload map[string]any
	if err = json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	amount := 1.
	intent := PerformanceIntent{Version: "performance-intent/2", Text: "Cached laughter preview", Tone: "demonstration", Cues: []PerformanceCue{{Kind: "laugh", Anchor: "laughter", Amount: &amount}}}
	payload["performance"] = PerformanceValidation{Intent: intent, Resolved: []ResolvedCue{{Kind: "laugh", Time: 0, Duration: duration, Amount: &amount}}}
	payload["duration_ms"] = duration * 1000
	payload["preview_only"] = true
	return payload, nil
}
