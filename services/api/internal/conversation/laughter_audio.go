package conversation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"syntelle/render-contract"
)

type cachedViseme struct {
	ID     int     `json:"id"`
	Offset float64 `json:"offset_ms"`
}
type cachedSpeech struct {
	Audio    []byte         `json:"audio_base64"`
	Duration float64        `json:"duration_ms"`
	Visemes  []cachedViseme `json:"visemes"`
	Voice    string         `json:"voice"`
	Language string         `json:"language"`
}

func pcm16(wav []byte) ([]byte, []byte, error) {
	if len(wav) < 44 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, nil, errors.New("invalid cached WAV")
	}
	var format, pcm []byte
	for pos := 12; pos+8 <= len(wav); {
		size := int(binary.LittleEndian.Uint32(wav[pos+4 : pos+8]))
		start := pos + 8
		if size > len(wav)-start {
			return nil, nil, errors.New("truncated cached WAV")
		}
		switch string(wav[pos : pos+4]) {
		case "fmt ":
			format = wav[start : start+size]
		case "data":
			pcm = wav[start : start+size]
		}
		pos = start + size + size%2
	}
	if len(format) < 16 || binary.LittleEndian.Uint16(format) != 1 || binary.LittleEndian.Uint16(format[2:]) != 1 || binary.LittleEndian.Uint16(format[14:]) != 16 || len(pcm)%2 != 0 || len(pcm) == 0 {
		return nil, nil, errors.New("unsupported cached PCM")
	}
	if rate := binary.LittleEndian.Uint32(format[4:8]); rate < 8000 || rate > 48000 {
		return nil, nil, errors.New("unsupported cached sample rate")
	}
	return format[:16], pcm, nil
}

// appendLaughter creates a derived WAV. Both original provider recordings and
// their native event streams remain unchanged in their original evidence.
func (s *Service) appendLaughter(cid, reply, voice, language string, payload map[string]any, v PerformanceValidation, commit ...bool) (PerformanceValidation, error) {
	index := -1
	for i, c := range v.Resolved {
		if c.Kind == "laugh" {
			index = i
			break
		}
	}
	if index < 0 {
		return v, nil
	}
	omit := func() PerformanceValidation {
		v.Resolved = append(v.Resolved[:index], v.Resolved[index+1:]...)
		v.Omitted = append(v.Omitted, "laughter_unavailable_or_cooldown")
		return v
	}
	if v.Resolved[index].Amount != nil && *v.Resolved[index].Amount <= 0 {
		return omit(), nil
	}
	if s.Performance == nil {
		return omit(), nil
	}
	raw, ok := s.Performance.Laughter[voice+"|"+language]
	if !ok {
		return omit(), nil
	}
	var laugh, spoken cachedSpeech
	if json.Unmarshal(raw, &laugh) != nil || laugh.Voice != voice || laugh.Language != language {
		return omit(), nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal(data, &spoken); err != nil {
		return v, err
	}
	format, a, err := pcm16(spoken.Audio)
	if err != nil {
		return v, err
	}
	other, b, err := pcm16(laugh.Audio)
	if err != nil {
		return omit(), nil
	}
	if len(b) == 0 || float64(len(b))/float64(binary.LittleEndian.Uint32(other[4:])*2) > 2.5 || string(format) != string(other) {
		return omit(), nil
	}
	if !s.laughterAllowed(cid, reply, len(commit) == 0 || commit[0]) {
		return omit(), nil
	}
	rate := int(binary.LittleEndian.Uint32(format[4:]))
	silence := rate * 80 / 1000 * 2
	pcm := make([]byte, len(a)+silence+len(b))
	copy(pcm, a)
	copy(pcm[len(a)+silence:], b)
	wav := make([]byte, 44+len(pcm))
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(36+len(pcm)))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	copy(wav[20:], format)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(pcm)))
	copy(wav[44:], pcm)
	offset := float64(len(a))/float64(rate*2)*1000 + 80
	for _, event := range laugh.Visemes {
		event.Offset += offset
		spoken.Visemes = append(spoken.Visemes, event)
	}
	payload["laughter_span"] = &rendercontract.LaughterSpan{Version: "laughter-span/1", Offset: (len(a) + silence) / 2, Count: len(b) / 2, PCMHash: fmt.Sprintf("%x", sha256.Sum256(b)), RecordingHash: fmt.Sprintf("%x", sha256.Sum256(laugh.Audio))}
	payload["audio_base64"] = wav
	payload["visemes"] = spoken.Visemes
	payload["duration_ms"] = float64(len(pcm)) / float64(rate*2) * 1000
	payload["derived_audio"] = map[string]any{"version": "cached-laughter-sequence/1", "speech_sha256": fmt.Sprintf("%x", sha256.Sum256(spoken.Audio)), "laughter_sha256": fmt.Sprintf("%x", sha256.Sum256(laugh.Audio)), "new_provider_requests": 0}
	complete := 1.
	v.Resolved[index].Amount = &complete
	v.Resolved[index].Time = offset / 1000
	v.Resolved[index].Duration = float64(len(b)) / float64(rate*2)
	return v, nil
}
