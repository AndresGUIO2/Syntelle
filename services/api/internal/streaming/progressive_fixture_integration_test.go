//go:build integration

package streaming

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"syntelle/render-contract"
	"time"
)

// Cached-only browser proof. The test process holds completion until the
// browser has actually decoded audio; no production endpoint/provider is added.
func (v *cachedVoiceFixture) PrepareDirectedProgressive(ctx context.Context, text, language, voice, id string, d rendercontract.VoiceDelivery, publish func(any) error) (any, error) {
	value, err := v.PrepareDirected(ctx, text, language, voice, id, d)
	gate := os.Getenv("SYNTELLE_STREAMING_FIXTURE_PREFIX_GATE")
	if err != nil || gate == "" {
		return value, err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var full map[string]any
	var native struct {
		Audio   []byte                  `json:"audio_base64"`
		Visemes []rendercontract.Viseme `json:"visemes"`
	}
	if json.Unmarshal(b, &full) != nil || json.Unmarshal(b, &native) != nil {
		return nil, errors.New("invalid cached prefix fixture")
	}
	var pcm []byte
	for at := 12; at+8 <= len(native.Audio); {
		size := int(binary.LittleEndian.Uint32(native.Audio[at+4:]))
		end := at + 8 + size
		if end > len(native.Audio) {
			return nil, errors.New("invalid cached WAV")
		}
		if string(native.Audio[at:at+4]) == "data" {
			pcm = native.Audio[at+8 : end]
		}
		at = end + size%2
	}
	n := 3 * 48000
	if len(pcm) < n {
		return nil, errors.New("cached fixture must contain three seconds")
	}
	wav := make([]byte, 44+n)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 24000)
	binary.LittleEndian.PutUint32(wav[28:], 48000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(n))
	copy(wav[44:], pcm[:n])
	prefix := make(map[string]any)
	for k, v := range full {
		prefix[k] = v
	}
	prefix["audio_base64"] = wav
	prefix["duration_ms"] = 3000
	visemes := []rendercontract.Viseme{}
	for _, v := range native.Visemes {
		if v.OffsetMS <= 3000 {
			visemes = append(visemes, v)
		}
	}
	prefix["visemes"] = visemes
	if err = publish(prefix); err != nil {
		return nil, err
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, errors.New("browser did not release cached completion")
		case <-ticker.C:
			if _, err = os.Stat(gate); err == nil {
				return value, nil
			}
		}
	}
}
