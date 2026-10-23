package speech

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"syntelle/api/internal/conversation"
	"testing"
	"time"
)

type parallelSpeechFixture struct {
	started chan string
	release <-chan struct{}
	calls   atomic.Int32
	fail    bool
}

func (p *parallelSpeechFixture) Synthesize(ctx context.Context, input Input) (Result, error) {
	p.calls.Add(1)
	select {
	case p.started <- input.Text:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	select {
	case <-p.release:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	if p.fail && strings.HasPrefix(input.Text, "The opening") {
		return Result{}, errors.New("fixture provider failure")
	}
	return Result{Audio: sequenceFixtureWAV(100), DurationMS: 100, Visemes: []Viseme{{0, 0}, {2, 20}, {0, 100}}, Mime: "audio/wav"}, nil
}

func TestProgressiveSpeechUsesTwoWorkersAndStopsOnProviderFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordered", true: "failure"}[fail], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			release := make(chan struct{})
			provider := &parallelSpeechFixture{started: make(chan string, 2), release: release, fail: fail}
			service := NewService(provider, &sequenceLedgerFixture{}, "canadacentral")
			speech := NewConversationSpeech(service).(conversationSpeech)
			text := "The opening sentence is ready. " + strings.Repeat("Another useful complete sentence. ", 40)
			done := make(chan error, 1)
			go func() {
				expected := 0
				_, err := speech.PrepareParts(ctx, text, "en-US", "en-US-Ava:DragonHDLatestNeural", "parallel-fixture", func(p conversation.SpeechPart) error {
					if p.Index != expected {
						return errors.New("part out of order")
					}
					expected++
					return nil
				})
				done <- err
			}()
			// Both requests must start before either is permitted to complete.
			for range 2 {
				select {
				case <-provider.started:
				case <-ctx.Done():
					t.Fatal("preparation remained serial")
				}
			}
			close(release)
			// Do not block later fixture requests on evidence capture.
			go func() {
				for {
					select {
					case <-provider.started:
					case <-ctx.Done():
						return
					}
				}
			}()
			select {
			case err := <-done:
				if fail && (err == nil || provider.calls.Load() != 2) {
					t.Fatal("failure retried or queued more requests", err, provider.calls.Load())
				}
				if !fail && err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("workers did not finish")
			}
		})
	}
}
