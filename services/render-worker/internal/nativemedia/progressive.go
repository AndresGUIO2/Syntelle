package nativemedia

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"
	"time"
)

// At most the playing and single reserved next phrase can grow concurrently.
// Prefixes/analysis are immutable; preparation never holds the audio mutex.
func (s *session) acceptSpeechPrefix(turn string, raw json.RawMessage, generation uint64) error {
	var part speechPart
	if json.Unmarshal(raw, &part) != nil || part.Index < 0 || math.IsNaN(part.OffsetMS) || math.IsInf(part.OffsetMS, 0) || part.OffsetMS < 0 || (part.Index == 0 && part.OffsetMS != 0) {
		return errors.New("invalid phrase prefix")
	}
	value, err := decodeClip(part.Speech, part.Text)
	if err != nil {
		return err
	}
	if len(value.samples) < sampleRate*3/5 || len(value.samples) > sampleRate*180 || value.preview {
		return errors.New("phrase prefix exceeds bound")
	}
	var performance struct{ Intent json.RawMessage }
	if json.Unmarshal(value.performance, &performance) != nil || len(performance.Intent) == 0 {
		return errors.New("prefix direction missing")
	}
	value.streaming, value.streamFinal, value.phraseIndex, value.replyOffsetMS = true, part.Final, part.Index, part.OffsetMS
	s.mu.Lock()
	old := s.streams[part.Index]
	valid := generation == s.generation && turn == s.turn && !s.ignored[turn]
	if old == nil {
		valid = valid && part.Index == s.nextIndex && s.deliveryMatchesLocked(value.delivery, part.Text, part.Index)
		if valid {
			expected, known := s.prefixOffsetLocked(part.Index)
			if !known || math.Abs(expected-part.OffsetMS) > 2 {
				s.mu.Unlock()
				return errors.New("prefix reply offset does not match preceding PCM")
			}
			if len(s.streams) >= 2 || len(s.queue) >= maxQueuedParts || s.queuedBytes+len(value.samples)*2 > maxQueuedPCMBytes {
				s.mu.Unlock()
				return errors.New("progressive phrase capacity exceeded")
			}
		}
	} else {
		valid = valid && reflect.DeepEqual(old.delivery, value.delivery) && old.text == value.text && old.replyOffsetMS == part.OffsetMS
		if valid && (len(value.samples) < len(old.samples) || !slices.Equal(value.samples[:len(old.samples)], old.samples) || len(value.visemes) < len(old.visemes) || !slices.Equal(value.visemes[:len(old.visemes)], old.visemes)) {
			s.mu.Unlock()
			return errors.New("changed admitted native prefix")
		}
		if valid && !samePrefixIntent(old.performance, value.performance) {
			s.mu.Unlock()
			return errors.New("changed approved prefix direction")
		}
		if valid && old.streamFinal {
			if len(value.samples) != len(old.samples) || len(value.visemes) != len(old.visemes) {
				s.mu.Unlock()
				return errors.New("extended finalized prefix")
			}
			s.mu.Unlock()
			return nil
		}
		if valid && len(value.samples) == len(old.samples) && !part.Final && slices.Equal(value.performance, old.performance) {
			s.mu.Unlock()
			return nil
		}
		valid = valid && (s.playing == old || s.deliveryMatchesLocked(value.delivery, part.Text, part.Index))
		if valid && s.queuedBytes+2*(len(value.samples)-len(old.samples)) > maxQueuedPCMBytes {
			s.mu.Unlock()
			return errors.New("progressive PCM capacity exceeded")
		}
	}
	s.mu.Unlock()
	if !valid {
		return nil
	}
	if preparer, ok := s.scene.(SpeechPreparer); ok {
		value.prepared, err = preparer.PrepareSpeech(s.ctx, SpeechState{Streaming: true, PhraseIndex: part.Index, TurnID: turn, Samples: value.samples, SampleRate: sampleRate, Duration: value.duration, Visemes: value.visemes, Text: value.text, Performance: value.performance})
		if err != nil {
			return err
		}
		if value.prepared == nil {
			return errors.New("phrase analysis missing")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation || turn != s.turn || s.streams[part.Index] != old {
		return nil
	}
	if old == nil {
		if !s.deliveryMatchesLocked(value.delivery, part.Text, part.Index) {
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
	if s.streams == nil {
		s.streams = map[int]*clip{}
	}
	s.streams[part.Index] = value
	s.refreshStreamAliasLocked()
	s.revision++
	return nil
}

func samePrefixIntent(a, b json.RawMessage) bool {
	var x, y struct {
		Intent          any
		Motion          any
		MotionDirection string `json:"motion_direction"`
		Resolved        []map[string]any
	}
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil || !reflect.DeepEqual(x.Intent, y.Intent) || !reflect.DeepEqual(x.Motion, y.Motion) || x.MotionDirection != y.MotionDirection {
		return false
	}
	known := map[string]map[string]any{}
	for _, c := range x.Resolved {
		if id, _ := c["id"].(string); id != "" {
			known[id] = c
		}
	}
	for _, c := range y.Resolved {
		if id, _ := c["id"].(string); id != "" {
			if previous, ok := known[id]; ok && !reflect.DeepEqual(previous, c) {
				return false
			}
			known[id] = c
		}
	}
	return true
}

func (s *session) prefixOffsetLocked(index int) (float64, bool) {
	if index == 0 {
		return 0, true
	}
	for _, c := range append([]*clip{s.playing}, s.queue...) {
		if c != nil && c.phraseIndex == index-1 {
			if c.streaming && !c.streamFinal {
				return 0, false
			}
			return c.replyOffsetMS + float64(c.duration)/float64(time.Millisecond), true
		}
	}
	if s.completedPhraseIndex == index {
		return s.completedSourceMS, true
	}
	return 0, false
}
func (s *session) refreshStreamAliasLocked() {
	s.stream = nil
	if s.playing != nil && s.playing.streaming {
		s.stream = s.playing
		return
	}
	for _, c := range s.queue {
		if c.streaming {
			s.stream = c
			return
		}
	}
}
