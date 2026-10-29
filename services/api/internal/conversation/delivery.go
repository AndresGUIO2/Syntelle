package conversation

import (
	"context"
	"errors"
	"syntelle/render-contract"
	"time"
)

var ErrDeliverySuperseded = errors.New("delivery superseded by requested settings")

type DeliveryTicket struct {
	Reservation rendercontract.DeliveryReservation
	Updates     <-chan rendercontract.DeliveryNotice
	Close       func()
}

type DeliveryCoordinator interface {
	ReserveDelivery(context.Context, string, rendercontract.DeliveryRequest) (*DeliveryTicket, error)
}

type DirectedSpeech interface {
	VoiceDirection(rendercontract.DeliveryReservation) (rendercontract.VoiceDelivery, error)
	PrepareDirected(context.Context, string, string, string, string, rendercontract.VoiceDelivery) (any, error)
}

type progressiveDirectedSpeech interface {
	PrepareDirectedProgressive(context.Context, string, string, string, string, rendercontract.VoiceDelivery, func(any) error) (any, error)
}

type phraseAttempt struct {
	streamed bool
	ticket   *DeliveryTicket
	audio    any
	ctx      context.Context
	cancel   context.CancelFunc
	notice   chan rendercontract.DeliveryNotice
	done     chan struct{}
}

// Facial cues and contextual amusement cannot override the emotion that was
// approved for this exact phrase. The original proposal remains in reservation evidence.
func coherentIntent(intent PerformanceIntent, reservation *rendercontract.DeliveryReservation) PerformanceIntent {
	if !reservation.Valid() {
		return intent
	}
	effective := reservation.Effective
	intent.Emotion = &effective
	switch effective.State {
	case "fearful", "sad", "angry", "disgusted":
		intent.Tone = "serious"
	}
	return intent
}

func (a *phraseAttempt) close() { a.cancel(); <-a.done; a.ticket.Close() }

func (a *phraseAttempt) activation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case notice := <-a.notice:
		if notice.Status == "activated" && notice.Token == a.ticket.Reservation.Token {
			return nil
		}
		if notice.Status == "invalidated" && notice.Reason == "superseded_by_settings" {
			return ErrDeliverySuperseded
		}
		return errors.New("approved delivery did not activate")
	}
}

func (s *Service) prepareDirectedPhrase(ctx context.Context, cid, id, voice, language string, p PerformancePhrase, index int, progress ...func(any, *rendercontract.DeliveryReservation) error) (*phraseAttempt, error) {
	provider, ok := s.Speech.(DirectedSpeech)
	if s.Delivery == nil || !ok {
		return nil, errors.New("coordinated speech is unavailable")
	}
	r := rendercontract.DeliveryRequest{TurnID: id, PhraseIndex: index, TextHash: rendercontract.TextHash(p.Text), Voice: voice, Language: language, Purpose: p.Purpose, Emotion: p.Emotion}
	ticket, err := s.Delivery.ReserveDelivery(ctx, cid, r)
	if err != nil {
		return nil, err
	}
	delivery, err := provider.VoiceDirection(ticket.Reservation)
	if err != nil {
		ticket.Close()
		return nil, err
	}
	op, err := s.Store.Reserve(ctx, "performance-reservation", ticket.Reservation.Token, rendercontract.DeliveryVersion, cid, id, map[string]any{"reservation": ticket.Reservation, "voice_delivery": delivery}, false)
	if err == nil {
		err = s.Store.Finish(ctx, op.ID, "received", ticket.Reservation, nil, map[string]string{"source": "private-preflight", "played": "false"}, nil)
	}
	if err != nil {
		ticket.Close()
		return nil, err
	}
	work, cancel := context.WithCancel(ctx)
	a := &phraseAttempt{ticket: ticket, ctx: work, cancel: cancel, notice: make(chan rendercontract.DeliveryNotice, 1), done: make(chan struct{})}
	go func() {
		defer close(a.done)
		select {
		case <-work.Done():
			return
		case n := <-ticket.Updates:
			// Early PCM needs an activation receipt immediately for echo checks,
			// even while the native provider is still completing this recording.
			if n.Status == "activated" && len(progress) > 0 && progress[0] != nil {
				receiptCtx, stop := context.WithTimeout(context.WithoutCancel(work), 3*time.Second)
				receipt, e := s.Store.Reserve(receiptCtx, "delivery-activation", ticket.Reservation.Token, rendercontract.DeliveryVersion, cid, id, map[string]any{"index": index, "text": p.Text, "token": ticket.Reservation.Token, "provider_pending": true}, false)
				if e == nil {
					e = s.Store.Finish(receiptCtx, receipt.ID, "received", ticket.Reservation, nil, map[string]string{"source": "authenticated-post-send-activation"}, nil)
				}
				stop()
				if e != nil {
					n.Status = "failed"
					n.Reason = "activation_archive_failed"
				}
			}
			if n.Status != "activated" {
				cancel()
			}
			a.notice <- n
		}
	}()
	key := id + "-" + ticket.Reservation.Token
	phraseContext := context.WithValue(work, phraseSpeechKey{}, key)
	var prepared any
	if streaming, ok := provider.(progressiveDirectedSpeech); ok && len(progress) > 0 && progress[0] != nil {
		prepared, err = streaming.PrepareDirectedProgressive(phraseContext, p.Text, language, voice, id, delivery, func(value any) error {
			if e := work.Err(); e != nil {
				return e
			}
			if e := progress[0](value, &ticket.Reservation); e != nil {
				return e
			}
			a.streamed = true
			return nil
		})
	} else {
		prepared, err = provider.PrepareDirected(phraseContext, p.Text, language, voice, id, delivery)
	}
	if err != nil {
		if work.Err() != nil && ctx.Err() == nil {
			select {
			case n := <-a.notice:
				if n.Status == "invalidated" && n.Reason == "superseded_by_settings" {
					err = ErrDeliverySuperseded
				}
			case <-ctx.Done():
				err = ctx.Err()
			case <-time.After(time.Second):
			}
		}
		a.close()
		return nil, err
	}
	a.audio = prepared
	return a, nil
}
