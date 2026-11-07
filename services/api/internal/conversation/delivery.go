package conversation

import (
	"context"
	"errors"
	"sync"
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
	streamed        bool
	ticket          *DeliveryTicket
	audio           any
	err             error
	ctx             context.Context
	cancel          context.CancelFunc
	activated       chan struct{}
	activationErr   error
	prepared        chan struct{}
	done            chan struct{}
	closeOnce       sync.Once
	progressCleanup func()
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

func (a *phraseAttempt) close() {
	a.closeOnce.Do(func() {
		a.cancel()
		if a.progressCleanup != nil {
			a.progressCleanup()
		}
		<-a.prepared
		<-a.done
		a.ticket.Close()
	})
}

func (a *phraseAttempt) activation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.activated:
		return a.activationErr
	}
}

func (s *Service) prepareDirectedPhrase(ctx context.Context, cid, id, voice, language string, p PerformancePhrase, index int, progress ...func(any, *rendercontract.DeliveryReservation) error) (*phraseAttempt, error) {
	a, err := s.beginDirectedPhrase(ctx, cid, id, voice, language, p, index, progress...)
	if err != nil {
		return nil, err
	}
	if err = a.preparation(ctx); err != nil {
		a.close()
		return nil, err
	}
	return a, nil
}

func (a *phraseAttempt) preparation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.prepared:
		return a.err
	}
}

// Preparation and activation have separate broadcast completions: the next
// phrase may prepare after actual PCM activation while this provider still runs.
func (s *Service) beginDirectedPhrase(ctx context.Context, cid, id, voice, language string, p PerformancePhrase, index int, progress ...func(any, *rendercontract.DeliveryReservation) error) (*phraseAttempt, error) {
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
	a := &phraseAttempt{ticket: ticket, ctx: work, cancel: cancel, activated: make(chan struct{}), prepared: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(a.done)
		defer close(a.activated)
		select {
		case <-work.Done():
			a.activationErr = work.Err()
			return
		case n, ok := <-ticket.Updates:
			// Early PCM needs an activation receipt immediately for echo checks,
			// even while the native provider is still completing this recording.
			if n.Status == "activated" && n.Token == ticket.Reservation.Token && len(progress) > 0 && progress[0] != nil {
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
			if !ok || n.Status != "activated" || n.Token != ticket.Reservation.Token {
				a.activationErr = errors.New("approved delivery did not activate")
				if n.Status == "invalidated" && n.Reason == "superseded_by_settings" {
					a.activationErr = ErrDeliverySuperseded
				}
				cancel()
			}
		}
	}()
	go func() {
		defer close(a.prepared)
		key := id + "-" + ticket.Reservation.Token
		phraseContext := context.WithValue(work, phraseSpeechKey{}, key)
		var prepared any
		var err error
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
				<-a.activated
				if errors.Is(a.activationErr, ErrDeliverySuperseded) {
					err = ErrDeliverySuperseded
				}
			}
			a.err = err
			cancel()
			return
		}
		a.audio = prepared
	}()
	return a, nil
}
