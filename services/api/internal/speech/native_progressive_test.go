package speech

import (
	"context"
	"errors"
	"testing"
	"time"
)

type progressiveFixture struct {
	ledger *fakeSpeechLedger
	fail   bool
	calls  int
}

func (p *progressiveFixture) Synthesize(context.Context, Input) (Result, error) {
	panic("unexpected nonprogressive provider call")
}
func (p *progressiveFixture) SynthesizeProgressive(ctx context.Context, _ Input, _ time.Duration, publish func(Result) error) (Result, error) {
	p.calls++
	if p.ledger.reserveCalls != 1 || p.ledger.finished {
		return Result{}, errors.New("request was not reserved before synthesis")
	}
	prefix := Result{Audio: pcmWAV(make([]byte, 48000)), DurationMS: 1000, Visemes: []Viseme{{1, 0}, {2, 500}}, Mime: "audio/wav"}
	if err := publish(prefix); err != nil {
		return prefix, err
	}
	if p.fail {
		return prefix, errors.New("fixture failed after audio onset")
	}
	if ctx.Err() != nil {
		return prefix, ctx.Err()
	}
	prefix.Audio = pcmWAV(make([]byte, 96000))
	prefix.DurationMS = 2000
	return prefix, nil
}
func TestProgressiveNativeSpeechCommitsOnlySuccessfulCompletion(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "partial failure"}[fail], func(t *testing.T) {
			ledger := &fakeSpeechLedger{}
			provider := &progressiveFixture{ledger: ledger, fail: fail}
			service := NewService(provider, ledger, "canadacentral")
			prefixes := 0
			result, err := service.synthesizeBounded(t.Context(), validSpeechInput(), 500, time.Second, func(prefix Result) error {
				prefixes++
				if ledger.finished || prefix.DurationMS != 1000 {
					t.Fatal("prefix waited for completion")
				}
				return nil
			})
			if prefixes != 1 || provider.calls != 1 || !ledger.finished || (err != nil) != fail {
				t.Fatal(prefixes, provider.calls, err)
			}
			if fail {
				if ledger.saved != nil || ledger.failure == "" {
					t.Fatal("partial failure became reusable speech")
				}
			} else if ledger.saved == nil || result.DurationMS != 2000 {
				t.Fatal("complete recording lost")
			}
		})
	}
}
func TestProgressiveNativeSpeechCancellationAndCachedReuse(t *testing.T) {
	ledger := &fakeSpeechLedger{}
	provider := &progressiveFixture{ledger: ledger}
	service := NewService(provider, ledger, "canadacentral")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, err := service.synthesizeBounded(ctx, validSpeechInput(), 500, time.Second, func(Result) error { cancel(); return ctx.Err() })
	if err == nil || ledger.saved != nil || provider.calls != 1 {
		t.Fatal("cancelled partial synthesis survived")
	}
	ledger = &fakeSpeechLedger{saved: &Result{Cached: true}}
	provider = &progressiveFixture{ledger: ledger}
	service = NewService(provider, ledger, "canadacentral")
	_, err = service.synthesizeBounded(t.Context(), validSpeechInput(), 500, time.Second, func(Result) error { t.Fatal("cache was streamed as a new provider request"); return nil })
	if err != nil || provider.calls != 0 {
		t.Fatal("cache retried provider")
	}
}
