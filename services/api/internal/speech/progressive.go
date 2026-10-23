package speech

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"syntelle/api/internal/conversation"
)

// Two bounded workers overlap the short opening phrases. A phrase is not
// scheduled until an earlier result is delivered, limiting prepared lookahead.
// The caller cancels and joins both workers on any failure or interruption.
func (s conversationSpeech) prepareConcurrentParts(parent context.Context, chunks []string, language, voice, id string, publish func(conversation.SpeechPart) error) ([]Result, []string, error) {
	ctx, cancel := context.WithCancel(parent)
	type prepared struct {
		index  int
		key    string
		result Result
		err    error
	}
	jobs := make(chan int, 2)
	results := make(chan prepared, 2)
	var workers sync.WaitGroup
	n := min(2, len(chunks))
	workers.Add(n)
	for range n {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case index := <-jobs:
					if ctx.Err() != nil {
						return
					}
					hash := sha256.Sum256([]byte(id))
					key := fmt.Sprintf("chat-%x-fast3-%03d", hash, index)
					part, err := s.service.synthesizeBounded(ctx, Input{Text: strings.TrimSpace(chunks[index]), Language: language, Voice: voice, RequestKey: key}, conversationPartUnits, 60*time.Second)
					select {
					case results <- prepared{index, key, part, err}:
					case <-ctx.Done():
						return
					}
					if err != nil {
						return
					}
				}
			}
		}()
	}
	defer func() { cancel(); workers.Wait() }()
	for i := 0; i < n; i++ {
		jobs <- i
	}
	scheduled, expected, offset := n, 0, 0.0
	pending := map[int]prepared{}
	parts := make([]Result, 0, len(chunks))
	keys := make([]string, 0, len(chunks))
	for expected < len(chunks) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case p := <-results:
			if p.err != nil {
				return nil, nil, p.err
			}
			pending[p.index] = p
			for {
				next, ok := pending[expected]
				if !ok {
					break
				}
				delete(pending, expected)
				if err := ctx.Err(); err != nil {
					return nil, nil, err
				}
				if err := publish(conversation.SpeechPart{Speech: next.result, Text: chunks[expected], Index: expected, Final: expected == len(chunks)-1, OffsetMS: offset}); err != nil {
					return nil, nil, err
				}
				parts = append(parts, next.result)
				keys = append(keys, next.key)
				offset += next.result.DurationMS
				expected++
				if scheduled < len(chunks) {
					select {
					case jobs <- scheduled:
						scheduled++
					case <-ctx.Done():
						return nil, nil, ctx.Err()
					}
				}
			}
		}
	}
	return parts, keys, nil
}

// The first complete phrase can be prepared while the model is still producing
// text. The final pipeline waits for this durable result and reuses its exact key.
func (s conversationSpeech) PrepareOpening(ctx context.Context, text, language, voice, id string) error {
	_, err := s.PrepareOpeningPart(ctx, text, language, voice, id)
	return err
}

func (s conversationSpeech) PrepareOpeningPart(ctx context.Context, text, language, voice, id string) (conversation.SpeechPart, error) {
	if cid, ok := conversation.IDs(ctx); ok {
		ctx = context.WithValue(ctx, speechTurnKey{}, speechTurnRef{cid, id})
	}
	hash := sha256.Sum256([]byte(id))
	key := fmt.Sprintf("chat-%x-fast3-%03d", hash, 0)
	result, err := s.service.synthesizeBounded(ctx, Input{Text: strings.TrimSpace(text), Language: language, Voice: voice, RequestKey: key}, conversationPartUnits, 60*time.Second)
	return conversation.SpeechPart{Speech: result, Text: text, Index: 0}, err
}
