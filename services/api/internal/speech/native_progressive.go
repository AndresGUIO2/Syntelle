package speech

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"syntelle/api/internal/conversation"
	"syntelle/render-contract"
	"time"
)

type progressiveProvider interface {
	SynthesizeProgressive(context.Context, Input, time.Duration, func(Result) error) (Result, error)
}

// The transport is raw native PCM; the durable/public recording remains a WAV.
// No samples are resampled, synthesized locally, or replayed on completion.
func pcmWAV(pcm []byte) []byte {
	b := make([]byte, 44+len(pcm))
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 24000)
	binary.LittleEndian.PutUint32(b[28:], 48000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(len(pcm)))
	copy(b[44:], pcm)
	return b
}

func (s conversationSpeech) PrepareDirectedProgressive(ctx context.Context, text, language, voice, id string, delivery rendercontract.VoiceDelivery, publish func(any) error) (any, error) {
	if len(splitSpeech(text)) != 1 {
		return nil, errors.New("progressive opening exceeds one native request")
	}
	if cid, ok := conversation.IDs(ctx); ok {
		ctx = context.WithValue(ctx, speechTurnKey{}, speechTurnRef{cid, id})
	}
	if key, ok := conversation.PhraseSpeechKey(ctx); ok {
		id = key
	}
	key := "chat-" + id
	if len(key) > 128 {
		hash := sha256.Sum256([]byte(id))
		key = fmt.Sprintf("chat-%x-part-%03d", hash, 0)
	}
	input, err := WithVoiceDelivery(Input{Text: text, Language: language, Voice: voice, RequestKey: key}, delivery)
	if err != nil {
		return nil, err
	}
	return s.service.synthesizeBounded(ctx, input, conversationPartUnits, 60*time.Second, func(r Result) error { return publish(r) })
}
