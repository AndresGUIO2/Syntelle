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

type phraseAttempt struct {
	ticket *DeliveryTicket
	audio  any
	ctx    context.Context
	cancel context.CancelFunc
	notice chan rendercontract.DeliveryNotice
	done   chan struct{}
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

func (s *Service) prepareDirectedPhrase(ctx context.Context, cid, id, voice, language string, p PerformancePhrase, index int) (*phraseAttempt, error) {
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
			if n.Status != "activated" {
				cancel()
			}
			a.notice <- n
		}
	}()
	key := id + "-" + ticket.Reservation.Token
	phraseContext := context.WithValue(work, phraseSpeechKey{}, key)
	prepared, err := provider.PrepareDirected(phraseContext, p.Text, language, voice, id, delivery)
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
