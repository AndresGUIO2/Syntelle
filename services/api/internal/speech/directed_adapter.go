package speech

import (
	"context"
	"syntelle/render-contract"
)

func (s conversationSpeech) VoiceDirection(r rendercontract.DeliveryReservation) (rendercontract.VoiceDelivery, error) {
	return s.service.VoiceDirection(r)
}

func (s conversationSpeech) PrepareDirected(ctx context.Context, text, language, voice, id string, delivery rendercontract.VoiceDelivery) (any, error) {
	return s.prepare(ctx, text, language, voice, id, splitSpeech(text), nil, delivery)
}
