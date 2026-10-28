package nativemedia

import (
	"context"
	"errors"
	"fmt"
)

// One session-owned lane prepares immutable clips in FIFO order. The wake
// channel carries no borrowed envelope or PCM and is never closed; cancellation
// ends the lane, and session teardown joins it through s.wg.
type preparationTask struct {
	clip       *clip
	generation uint64
	turn       string
	ctx        context.Context // owned job lifecycle, canceled on completion/interrupt
	cancel     context.CancelFunc
	detached   bool
}

func (s *session) notifyPreparationLocked() {
	select {
	case s.preparationWake <- struct{}{}:
	default:
	}
}

func (s *session) preparationPendingLocked() bool {
	return len(s.queue) > 0 && s.queue[0].needsPreparation
}

func (s *session) cancelPreparationLocked() int {
	if s.preparing == nil {
		return 0
	}
	s.preparing.detached = true
	s.preparing.cancel()
	return len(s.preparing.clip.samples) * 2
}

func (s *session) nextPreparation() (*preparationTask, SpeechPreparer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preparer, ok := s.scene.(SpeechPreparer)
	if !ok || s.ctx.Err() != nil {
		return nil, nil
	}
	for _, value := range s.queue {
		if value.needsPreparation {
			ctx, cancel := context.WithCancel(s.ctx)
			task := &preparationTask{clip: value, generation: s.generation, turn: s.turn, ctx: ctx, cancel: cancel}
			s.preparing = task
			return task, preparer
		}
	}
	return nil, nil
}

func (s *session) preparationLoop() {
	defer s.wg.Done()
	for {
		task, preparer := s.nextPreparation()
		if task == nil {
			select {
			case <-s.ctx.Done():
				return
			case <-s.preparationWake:
				continue
			}
		}
		value := task.clip
		// Decoding transferred ownership to the clip. Only immutable data crosses
		// this call; no scene engine or animation-clock state is touched here.
		prepared, err := preparer.PrepareSpeech(task.ctx, SpeechState{LaughterSpan: value.laughterSpan, Preview: value.preview, TurnID: task.turn, Samples: value.samples, SampleRate: sampleRate, Duration: value.duration, Visemes: value.visemes, Text: value.text, Performance: value.performance})
		task.cancel()
		s.mu.Lock()
		s.preparing = nil
		if task.detached {
			s.queuedBytes -= len(value.samples) * 2
		}
		current := !task.detached && task.generation == s.generation && task.turn == s.turn && s.ctx.Err() == nil
		if current && err == nil {
			if prepared == nil {
				err = errors.New("native speech preparation returned no analysis")
			} else {
				value.prepared = prepared
				value.needsPreparation = false
				s.revision++
			}
		}
		s.mu.Unlock()
		if current && err != nil {
			s.fail(fmt.Errorf("prepare native speech: %w", err))
			return
		}
	}
}
