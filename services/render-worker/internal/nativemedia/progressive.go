package nativemedia

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
)

// Immutable prefix replacement: preparation never holds the audio mutex, and
// the installed sample cursor never moves backwards. Only the first quiet
// phrase uses this path; complete/laughter/gesture clips retain their path.
func (s *session) acceptSpeechPrefix(turn string, raw json.RawMessage, generation uint64) error {
	var part speechPart
	if json.Unmarshal(raw, &part) != nil || part.Index != 0 || part.OffsetMS != 0 {
		return errors.New("invalid opening prefix")
	}
	value, err := decodeClip(part.Speech, part.Text)
	if err != nil {
		return err
	}
	if len(value.samples) > sampleRate*30 || len(value.samples) < sampleRate*4/5 || value.preview {
		return errors.New("opening prefix exceeds bound")
	}
	var performance struct {
		Resolved []json.RawMessage `json:"resolved"`
		Intent   struct {
			Cues []json.RawMessage `json:"cues"`
		} `json:"intent"`
	}
	if json.Unmarshal(value.performance, &performance) != nil || len(performance.Resolved) > 0 || len(performance.Intent.Cues) > 0 {
		return errors.New("opening prefix requires quiet performance")
	}
	value.streaming = true
	value.streamFinal = part.Final
	value.phraseIndex = 0
	s.mu.Lock()
	old := s.stream
	valid := generation == s.generation && turn == s.turn && !s.ignored[turn]
	if old == nil {
		valid = valid && s.nextIndex == 0 && s.deliveryMatchesLocked(value.delivery, part.Text, 0)
	} else {
		valid = valid && reflect.DeepEqual(old.delivery, value.delivery) && old.text == value.text
		if !old.streamFinal && (len(value.samples) < len(old.samples) || !slices.Equal(value.samples[:len(old.samples)], old.samples) || len(value.visemes) < len(old.visemes) || !slices.Equal(value.visemes[:len(old.visemes)], old.visemes)) {
			s.mu.Unlock()
			return errors.New("opening changed an admitted native prefix")
		}
		if old.streamFinal || (len(value.samples) == len(old.samples) && !part.Final) {
			s.mu.Unlock()
			return nil
		}
		valid = valid && (s.playing == old || s.deliveryMatchesLocked(value.delivery, part.Text, 0))
	}
	s.mu.Unlock()
	if !valid {
		return nil
	}
	if preparer, ok := s.scene.(SpeechPreparer); ok {
		value.prepared, err = preparer.PrepareSpeech(s.ctx, SpeechState{Streaming: true, TurnID: turn, Samples: value.samples, SampleRate: sampleRate, Duration: value.duration, Visemes: value.visemes, Text: value.text, Performance: value.performance})
		if err != nil {
			return err
		}
		if value.prepared == nil {
			return errors.New("opening analysis missing")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation || turn != s.turn || s.stream != old {
		return nil
	}
	if old == nil {
		if !s.deliveryMatchesLocked(value.delivery, part.Text, 0) {
			return nil
		}
		s.queue = append(s.queue, value)
		s.nextIndex++
	} else if s.playing == old {
		s.playing = value
	} else {
		found := false
		for i, c := range s.queue {
			if c == old {
				s.queue[i] = value
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	previousBytes := 0
	if old != nil {
		previousBytes = len(old.samples) * 2
	}
	s.queuedBytes += len(value.samples)*2 - previousBytes
	s.stream = value
	s.revision++
	return nil
}
