package conversation

import "context"

type phraseSpeechKey struct{}

// PhraseSpeechKey separates per-phrase idempotency from the real parent turn ID.
// Recording adapters must retain the turn ID for their foreign-key binding.
func PhraseSpeechKey(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(phraseSpeechKey{}).(string)
	return value, ok
}
