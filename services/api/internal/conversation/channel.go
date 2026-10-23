package conversation

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// Channel is a private in-process transport. Its owner must call Close; it has
// the same single-consumer session loop as the historical browser WebSocket.
type Channel struct {
	incoming chan Event
	done     chan struct{}
	cancel   context.CancelFunc
}

func (s *Service) OpenChannel(parent context.Context, emit func(Event)) (*Channel, error) {
	ctx, cancel := context.WithCancel(parent)
	c := &Channel{incoming: make(chan Event, 16), done: make(chan struct{}), cancel: cancel}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		cancel()
		return nil, errors.New("conversation service is shutting down")
	}
	if s.channels == nil {
		s.channels = make(map[*Channel]context.CancelFunc)
	}
	s.channels[c] = cancel
	s.workers.Add(1)
	s.mu.Unlock()
	go func() {
		defer close(c.done)
		defer cancel()
		defer func() { s.mu.Lock(); delete(s.channels, c); s.mu.Unlock(); s.workers.Done() }()
		s.runSession(ctx, c.incoming, func(producer context.Context, event Event) {
			if producer.Err() == nil && ctx.Err() == nil {
				emit(event)
			}
		})
	}()
	return c, nil
}

func (c *Channel) Send(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-c.done:
		return errors.New("conversation channel is closed")
	default:
	}
	timer := time.NewTimer(outputTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return errors.New("conversation channel is closed")
	case c.incoming <- event:
		return nil
	case <-timer.C:
		return errors.New("conversation channel is congested")
	}
}
func (c *Channel) Close()                { c.cancel() }
func (c *Channel) Done() <-chan struct{} { return c.done }

// runLiteralTurn bypasses the language model. Segmentation and native synthesis
// remain the shared speech provider's responsibility; all supplied text survives.
func (s *Service) runLiteralTurn(ctx context.Context, cid, id, text, language, voice string, send func(Event)) {
	ctx = context.WithValue(ctx, sessionContextKey{}, cid)
	status, reason := "failed", ""
	emit := func(kind, content string, data any) {
		send(Event{Type: kind, ConversationID: cid, TurnID: id, Text: content, Data: data})
	}
	defer func() {
		if status == "ready" {
			return
		}
		if ctx.Err() != nil {
			status, reason = "interrupted", "cancelled"
		}
		archive, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if err := s.Store.FinishTurn(archive, id, status, text, text, reason); err != nil {
			emit("persistence_warning", "Speech stopped, but its final state could not be saved.", nil)
		}
	}()
	emit("reply_complete", text, map[string]string{"spoken": text})
	emit("preparing", "", nil)
	if speech, ok := s.Speech.(progressiveSpeech); ok {
		parts := 0
		_, err := speech.PrepareParts(ctx, text, language, voice, id, func(part SpeechPart) error {
			evidence, err := speechDeliveryEvidence(part)
			if err != nil {
				return err
			}
			op, err := s.Store.Reserve(ctx, "speech-delivery", id+"-literal-part-"+strconv.Itoa(part.Index), "literal-speech/1", cid, id, map[string]any{"index": part.Index, "text": part.Text, "offset_ms": part.OffsetMS, "literal": true}, false)
			if err != nil {
				return err
			}
			if err = s.Store.Finish(ctx, op.ID, "received", evidence, nil, map[string]string{"source": "literal-speech/1"}, nil); err != nil {
				return err
			}
			parts++
			emit("speech_part", part.Text, part)
			return ctx.Err()
		})
		if err != nil {
			if ctx.Err() == nil {
				emit("error", "Speech preparation failed; no retry was made.", nil)
			}
			return
		}
		if err = s.Store.FinishTurn(ctx, id, "ready", text, text, ""); err != nil {
			emit("error", "Speech result could not be committed.", nil)
			return
		}
		status = "ready"
		emit("speech_complete", text, map[string]int{"parts": parts})
		return
	}
	prepared, err := s.Speech.Prepare(ctx, text, language, voice, id)
	if err != nil {
		if ctx.Err() == nil {
			emit("error", "Speech preparation failed; no retry was made.", nil)
		}
		return
	}
	if err = s.Store.FinishTurn(ctx, id, "ready", text, text, ""); err != nil {
		emit("error", "Speech result could not be committed.", nil)
		return
	}
	status = "ready"
	emit("prepared_speech", text, prepared)
}
