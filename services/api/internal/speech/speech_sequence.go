package speech

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"syntelle/api/internal/conversation"
)

// A preparation size, never a reply-length restriction. Every text byte is
// retained; speech/audio limits apply independently to each provider request.
const conversationPartUnits = 3000

// Start with a short natural phrase, then amortize requests over larger parts.
// This changes preparation granularity, never the complete spoken reply.
func splitProgressiveSpeech(text string) []string {
	var parts []string
	for text != "" {
		if len(parts) == 0 {
			opening := conversation.OpeningPhrase(text, true)
			cut := strings.Index(text, opening) + len(opening)
			if cut <= 0 {
				return nil
			}
			for cut < len(text) && strings.ContainsRune(" \n\t\r", rune(text[cut])) {
				cut++
			}
			parts = append(parts, text[:cut])
			text = text[cut:]
			continue
		}
		limit := 900
		if len(parts) < 3 {
			limit = 220
		}
		units, cut, word, sentence := 0, 0, 0, 0
		for index, r := range text {
			n := 1
			if r > 0xffff {
				n = 2
			}
			if units+n > limit {
				break
			}
			units += n
			cut = index + len(string(r))
			if r == ' ' || r == '\n' {
				word = cut
			}
			if strings.ContainsRune(".!?。！？", r) && units >= 24 {
				sentence = cut
				if len(parts) == 0 {
					break
				}
			}
		}
		if cut < len(text) {
			if sentence > 0 {
				cut = sentence
			} else if word > 0 {
				cut = word
			}
		}
		// Attach boundary whitespace to this part, avoiding whitespace-only tails.
		for cut < len(text) && (text[cut] == ' ' || text[cut] == '\n' || text[cut] == '\t' || text[cut] == '\r') {
			cut++
		}
		parts = append(parts, text[:cut])
		text = text[cut:]
	}
	return parts
}

type PreparedPart struct {
	RequestKey       string  `json:"request_key"`
	ProviderResultID string  `json:"provider_result_id"`
	AudioSHA256      string  `json:"audio_sha256"`
	OffsetMS         float64 `json:"offset_ms"`
	DurationMS       float64 `json:"duration_ms"`
}

func splitSpeech(text string) []string {
	var parts []string
	for text != "" {
		units, cut, word, sentence := 0, 0, 0, 0
		for index, r := range text {
			n := 1
			if r > 0xffff {
				n = 2
			}
			if units+n > conversationPartUnits {
				break
			}
			units += n
			cut = index + len(string(r))
			if r == ' ' || r == '\n' {
				word = cut
			}
			if strings.ContainsRune(".!?。！？", r) {
				sentence = cut
			}
		}
		if cut < len(text) {
			if sentence >= cut/2 {
				cut = sentence
			} else if word > 0 {
				cut = word
			}
		}
		parts = append(parts, text[:cut])
		text = text[cut:]
	}
	return parts
}

func speechPCM(wav []byte) ([]byte, []byte, error) {
	if len(wav) < 44 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, nil, errors.New("invalid speech WAV")
	}
	var format, pcm []byte
	for at := 12; at+8 <= len(wav); {
		size := int(binary.LittleEndian.Uint32(wav[at+4 : at+8]))
		start := at + 8
		if size > len(wav)-start {
			return nil, nil, errors.New("truncated speech WAV")
		}
		switch string(wav[at : at+4]) {
		case "fmt ":
			if size >= 16 {
				format = wav[start : start+16]
			}
		case "data":
			pcm = wav[start : start+size]
		}
		at = start + size + size%2
	}
	if len(format) != 16 || binary.LittleEndian.Uint16(format) != 1 || binary.LittleEndian.Uint16(format[2:]) != 1 || binary.LittleEndian.Uint16(format[14:]) != 16 || binary.LittleEndian.Uint32(format[4:]) != 24000 || len(pcm) == 0 || len(pcm)%2 != 0 {
		return nil, nil, errors.New("expected pinned mono PCM16 speech")
	}
	return format, pcm, nil
}

func joinSpeech(parts []Result, keys []string) (Result, error) {
	if len(parts) == 0 || len(parts) != len(keys) {
		return Result{}, errors.New("missing speech sequence parts")
	}
	result := Result{Mime: "audio/wav", Voice: parts[0].Voice, Language: parts[0].Language, Region: parts[0].Region, SDKVersion: parts[0].SDKVersion, Cached: true, ProviderStatus: "assembled-native-parts"}
	var format, pcm []byte
	for i, part := range parts {
		if part.Voice != result.Voice || part.Language != result.Language || part.Region != result.Region {
			return Result{}, errors.New("speech sequence identity differs")
		}
		f, data, err := speechPCM(part.Audio)
		if err != nil {
			return Result{}, err
		}
		if i == 0 {
			format = f
		} else if !bytes.Equal(format, f) {
			return Result{}, errors.New("speech sequence format differs")
		}
		duration := float64(len(data)) / 48000 * 1000
		if math.Abs(duration-part.DurationMS) > 100 {
			return Result{}, errors.New("speech part duration differs from PCM")
		}
		hash := sha256.Sum256(part.Audio)
		result.Parts = append(result.Parts, PreparedPart{keys[i], part.ProviderResultID, hex.EncodeToString(hash[:]), result.DurationMS, duration})
		for _, event := range part.Visemes {
			result.Visemes = append(result.Visemes, Viseme{event.ID, result.DurationMS + math.Min(event.OffsetMS, duration)})
		}
		result.DurationMS += duration
		result.Cached = result.Cached && part.Cached
		pcm = append(pcm, data...)
	}
	wav := make([]byte, 44+len(pcm))
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	copy(wav[20:], format)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(pcm)))
	copy(wav[44:], pcm)
	result.Audio = wav
	return result, validateSpeechResult(result)
}
