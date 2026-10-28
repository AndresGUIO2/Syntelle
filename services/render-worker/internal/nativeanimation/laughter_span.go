package nativeanimation

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"syntelle/render-contract"
)

type preparedLaughter struct {
	span     rendercontract.LaughterSpan
	channels *preparedArticulation
	rate     int
}

func ValidateLaughterSpan(span *rendercontract.LaughterSpan, samples []int16, rate int) error {
	if span == nil {
		return nil
	}
	if rate <= 0 || span.Version != "laughter-span/1" || span.Offset < 0 || span.Count <= 0 || span.Offset > len(samples) || span.Count > len(samples)-span.Offset || float64(span.Count)/float64(rate) > 2.5 {
		return errors.New("invalid bounded laughter PCM span")
	}
	for _, hash := range []string{span.PCMHash, span.RecordingHash} {
		if data, err := hex.DecodeString(hash); err != nil || len(data) != 32 {
			return errors.New("invalid laughter source hash")
		}
	}
	pcm := make([]byte, span.Count*2)
	for i, v := range samples[span.Offset : span.Offset+span.Count] {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}
	if fmt.Sprintf("%x", sha256.Sum256(pcm)) != span.PCMHash {
		return errors.New("laughter PCM hash mismatch")
	}
	return nil
}
func AttachLaughterSpan(ctx context.Context, p *PreparedSpeech, samples []float32, rate int, span *rendercontract.LaughterSpan, c Config) error {
	if span == nil {
		return nil
	}
	if span.Offset < 0 || span.Count <= 0 || span.Offset > len(samples) || span.Count > len(samples)-span.Offset {
		return errors.New("laughter span outside prepared PCM")
	}
	portion := samples[span.Offset : span.Offset+span.Count]
	var energy []float64
	acousticCues(ctx, portion, rate, &energy)
	channels, err := prepareArticulation(ctx, portion, rate, nil, nil, energy, c, true)
	if err != nil {
		return err
	}
	p.laughter = &preparedLaughter{span: *span, channels: channels, rate: rate}
	return nil
}
func (s *Speech) LaughterAt(seconds float64) (ArticulationFrame, bool) {
	if s == nil || s.laughter == nil {
		return ArticulationFrame{}, false
	}
	p := s.laughter
	relative := seconds - float64(p.span.Offset)/float64(p.rate)
	if relative < 0 {
		return ArticulationFrame{}, false
	}
	return envelopeSample(p.channels.frames, relative), true
}
func (s *Speech) LaughterLastBurst() float64 {
	if s == nil || s.laughter == nil {
		return 0
	}
	return float64(s.laughter.span.Offset)/float64(s.laughter.rate) + s.laughter.channels.lastBurst
}
