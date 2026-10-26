package speech

import (
	"context"
	"errors"
	"strconv"
)

const defaultSpeechConcurrency = 8

func configuredSpeechConcurrency(value string) (int, error) {
	if value == "" {
		return defaultSpeechConcurrency, nil
	}
	concurrency, err := strconv.Atoi(value)
	if err != nil || concurrency < 1 || concurrency > 32 {
		return 0, errors.New("SYNTELLE_SPEECH_CONCURRENCY must be between 1 and 32")
	}
	return concurrency, nil
}

// synthesisGate bounds simultaneous native calls while allowing queued callers
// to leave immediately when their conversation is cancelled.
type synthesisGate struct {
	slot chan struct{}
}

func newSynthesisGate(concurrency int) synthesisGate {
	return synthesisGate{slot: make(chan struct{}, concurrency)}
}

func (g *synthesisGate) acquire(ctx context.Context) (func(), error) {
	select {
	case g.slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-g.slot
			return nil, err
		}
		return func() { <-g.slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
