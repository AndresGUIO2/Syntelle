package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"syntelle/render-contract"
)

// The first quiet phrase is eligible. Anchored gestures/laughter keep the
// complete native recording path, where their full timing is available.
func (s *Service) openingPublisher(ctx context.Context, cid, id string, p PerformancePhrase, index int, emit func(string, string, any)) func(any, *rendercontract.DeliveryReservation) error {
	if index != 0 || len(p.Cues) != 0 || len([]rune(p.Text)) > 240 {
		return nil
	}
	return func(value any, reservation *rendercontract.DeliveryReservation) error {
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var payload map[string]any
		if json.Unmarshal(b, &payload) != nil {
			return errors.New("invalid progressive opening")
		}
		var native struct {
			Audio    []byte  `json:"audio_base64"`
			Duration float64 `json:"duration_ms"`
		}
		if json.Unmarshal(b, &native) != nil || native.Duration < 800 || len(native.Audio) < 44 {
			return errors.New("invalid progressive audio")
		}
		intent := coherentIntent(PerformanceIntent{Version: p.Version, Purpose: p.Purpose, Emotion: p.Emotion, Text: p.Text, Tone: p.Tone, Cues: p.Cues}, reservation)
		direction := ValidatePerformance(intent, p.Text)
		payload["performance"] = direction
		payload["delivery"] = reservation
		payload["emotion_phrase_index"] = index
		part := SpeechPart{Speech: payload, Text: p.Text, Index: index}
		evidence, err := speechDeliveryEvidence(part)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("%s-prefix-%d", reservation.Token, len(native.Audio))
		op, err := s.Store.Reserve(ctx, "speech-prefix", key, "native-prefix/1", cid, id, map[string]any{"token": reservation.Token, "index": index, "provider_pending": true}, false)
		if err != nil {
			return err
		}
		if err = s.Store.Finish(ctx, op.ID, "received", evidence, nil, map[string]any{"provider_pending": true}, native.Audio); err != nil {
			return err
		}
		emit("speech_prefix", p.Text, part)
		return nil
	}
}
