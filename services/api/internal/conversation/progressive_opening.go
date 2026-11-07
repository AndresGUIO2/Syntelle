package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"syntelle/render-contract"
)

// State is owned by one provider publisher until its preparation completes.
// A later phrase can synthesize immediately, but publishes only after the
// predecessor's final length establishes its immutable reply-relative offset.
type progressivePhraseState struct {
	ready    <-chan struct{}
	offset   func() float64
	budget   *replyGestures
	mu       *sync.Mutex
	acoustic *acousticSchedule
	resolved []ResolvedCue
}

func retainResolved(previous, current []ResolvedCue) []ResolvedCue {
	result := append([]ResolvedCue(nil), previous...)
	seen := make(map[string]bool, len(previous))
	for _, c := range previous {
		if c.ID != "" {
			seen[c.ID] = true
		}
	}
	for _, c := range current {
		if c.ID != "" && seen[c.ID] {
			continue
		}
		result = append(result, c)
		if c.ID != "" {
			seen[c.ID] = true
		}
	}
	return result
}

func (s *Service) openingPublisher(ctx context.Context, cid, id string, p PerformancePhrase, index int, emit func(string, string, any), states ...*progressivePhraseState) func(any, *rendercontract.DeliveryReservation) error {
	// Older motion profiles build their gesture list once, at phrase entry.
	// Give those packages the complete recording for the same cases as before.
	if s.Performance == nil || s.Performance.MotionDirection != SemanticMotionDirection {
		if index != 0 || len(p.Cues) != 0 || len([]rune(p.Text)) > 240 {
			return nil
		}
	}
	if provider, ok := s.Speech.(interface{ CanStreamPhrase(string) bool }); ok && !provider.CanStreamPhrase(p.Text) {
		return nil
	}
	for _, c := range p.Cues {
		if c.Kind == "laugh" {
			return nil
		}
	}
	var state *progressivePhraseState
	if len(states) > 0 {
		state = states[0]
	}
	return func(value any, reservation *rendercontract.DeliveryReservation) error {
		offset := 0.
		if state != nil && state.ready != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-state.ready:
			}
			offset = state.offset()
		}
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var payload map[string]any
		if json.Unmarshal(b, &payload) != nil {
			return errors.New("invalid progressive opening")
		}
		var native struct {
			Audio    []byte                        `json:"audio_base64"`
			Duration float64                       `json:"duration_ms"`
			Words    []rendercontract.WordBoundary `json:"word_boundaries"`
		}
		if json.Unmarshal(b, &native) != nil || native.Duration < 600 || len(native.Audio) < 44 {
			return errors.New("invalid progressive audio")
		}
		intent := coherentIntent(PerformanceIntent{Version: p.Version, Purpose: p.Purpose, Emotion: p.Emotion, Text: p.Text, Tone: p.Tone, Cues: p.Cues}, reservation)
		direction := ValidatePerformance(intent, p.Text)
		if s.Performance != nil && s.Performance.MotionPolicy == rendercontract.ConversationalMotionVersion {
			direction.Motion = &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: id, Offset: offset / 1000}
		}
		if state != nil {
			state.mu.Lock()
			if state.acoustic.semanticOnly {
				direction = ResolvePartialPerformance(direction, native.Audio, native.Duration/1000, state.acoustic, offset/1000, native.Words...)
				AssignMotionCueIDs(&direction, index)
				direction = s.filterCapabilities(direction)
				direction = state.budget.admit(direction, offset/1000)
				direction.Resolved = retainResolved(state.resolved, direction.Resolved)
				state.resolved = direction.Resolved
			}
			state.mu.Unlock()
		}
		payload["performance"] = direction
		payload["delivery"] = reservation
		payload["emotion_phrase_index"] = index
		part := SpeechPart{Speech: payload, Text: p.Text, Index: index, OffsetMS: offset}
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
