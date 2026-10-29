//go:build speech && linux

package speech

import (
	"context"
	"errors"
	"os"
	"sort"
	"sync"
	"syntelle/render-contract"
	"time"

	"github.com/Microsoft/cognitive-services-speech-sdk-go/common"
	"github.com/Microsoft/cognitive-services-speech-sdk-go/speech"
)

type azureSpeechProvider struct {
	key, region string
	synthesis   synthesisGate
}

func (*azureSpeechProvider) RequiresVoiceQualification() bool { return true }

func NewAzureProvider(key, region string) (Provider, error) {
	concurrency, err := configuredSpeechConcurrency(os.Getenv("SYNTELLE_SPEECH_CONCURRENCY"))
	if err != nil {
		return nil, err
	}
	return &azureSpeechProvider{key: key, region: region, synthesis: newSynthesisGate(concurrency)}, nil
}
func (p *azureSpeechProvider) Synthesize(ctx context.Context, input Input) (Result, error) {
	return p.SynthesizeQueued(ctx, input, 0)
}
func (p *azureSpeechProvider) SynthesizeQueued(ctx context.Context, input Input, timeout time.Duration) (Result, error) {
	return p.synthesizeNative(ctx, input, timeout, nil)
}
func (p *azureSpeechProvider) SynthesizeProgressive(ctx context.Context, input Input, timeout time.Duration, publish func(Result) error) (Result, error) {
	return p.synthesizeNative(ctx, input, timeout, publish)
}
func (p *azureSpeechProvider) synthesizeNative(ctx context.Context, input Input, timeout time.Duration, publish func(Result) error) (Result, error) {
	result := Result{Mime: "audio/wav", Voice: input.Voice, Language: input.Language, SDKVersion: speechSDKVersion}
	// HD synthesis can reject bursts even when the resource's published TPS
	// allowance is higher. Bound native calls per API replica and let queued
	// conversations cancel without consuming the provider deadline.
	release, err := p.synthesis.acquire(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	config, err := speech.NewSpeechConfigFromSubscription(p.key, p.region)
	if err != nil {
		return result, errors.New("Azure Speech configuration failed.")
	}
	defer config.Close()
	format := common.Riff24Khz16BitMonoPcm
	if publish != nil {
		format = common.Raw24Khz16BitMonoPcm
	}
	for _, err = range []error{config.SetSpeechSynthesisVoiceName(input.Voice), config.SetSpeechSynthesisLanguage(input.Language), config.SetSpeechSynthesisOutputFormat(format)} {
		if err != nil {
			return result, errors.New("Azure Speech settings failed.")
		}
	}
	synthesizer, err := speech.NewSpeechSynthesizerFromConfig(config, nil)
	if err != nil {
		return result, errors.New("Azure synthesizer initialization failed.")
	}
	defer synthesizer.Close()
	var lock sync.Mutex
	events := []Viseme{}
	words := []rendercontract.WordBoundary{}
	var rawPCM []byte
	var streamError error
	wake := make(chan struct{}, 1)
	notify := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	if publish != nil {
		synthesizer.Synthesizing(func(event speech.SpeechSynthesisEventArgs) {
			defer event.Close()
			lock.Lock()
			if len(rawPCM)+len(event.Result.AudioData) > 8<<20 {
				streamError = errors.New("native opening exceeded its audio bound")
			} else {
				rawPCM = append(rawPCM, event.Result.AudioData...)
			}
			lock.Unlock()
			notify()
		})
	}
	synthesizer.WordBoundary(func(event speech.SpeechSynthesisWordBoundaryEventArgs) {
		defer event.Close()
		lock.Lock()
		defer lock.Unlock()
		words = append(words, rendercontract.WordBoundary{Text: event.Text, TextOffset: int(event.TextOffset), Length: int(event.WordLength), OffsetMS: float64(event.AudioOffset) / 10000, DurationMS: float64(event.Duration) / float64(time.Millisecond)})
	})
	synthesizer.VisemeReceived(func(event speech.SpeechSynthesisVisemeEventArgs) {
		defer event.Close()
		lock.Lock()
		defer lock.Unlock()
		events = append(events, Viseme{event.VisemeID, float64(event.AudioOffset) / 10000})
		if publish != nil {
			notify()
		}
	})
	// Text-only synthesis intentionally mirrors OrgLens; no SVG/blendshape SSML request.
	var pending <-chan speech.SpeechSynthesisOutcome
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if input.providerText != "" {
		pending = synthesizer.SpeakSsmlAsync(input.providerText)
	} else {
		pending = synthesizer.SpeakTextAsync(input.Text)
	}
	var outcome speech.SpeechSynthesisOutcome
	lastBytes := 0
	waiting := true
	for waiting {
		select {
		case outcome = <-pending:
			waiting = false
		case <-ctx.Done():
			<-synthesizer.StopSpeakingAsync()
			outcome = <-pending
			waiting = false
		case <-wake:
			lock.Lock()
			prefix := result
			// Keep native viseme lookahead ahead of audible PCM. A provider that
			// delays visemes simply retains the complete-recording path.
			n := len(rawPCM) / 2 * 2
			if len(events) > 1 {
				n = min(n, int(events[len(events)-1].OffsetMS*48)/2*2)
			} else {
				n = 0
			}
			ready := n >= 38400 && n-lastBytes >= 19200 && streamError == nil
			if ready {
				prefix.Audio = pcmWAV(rawPCM[:n])
				prefix.DurationMS = float64(n) / 48
				for _, v := range events {
					if v.OffsetMS <= prefix.DurationMS {
						prefix.Visemes = append(prefix.Visemes, v)
					}
				}
				lastBytes = n
			}
			err = streamError
			lock.Unlock()
			if err == nil && ready && len(prefix.Visemes) >= 2 {
				err = publish(prefix)
			}
			if err != nil {
				<-synthesizer.StopSpeakingAsync()
				outcome = <-pending
				waiting = false
			}
		}
	}
	progressError := err
	if outcome.Result != nil {
		defer outcome.Result.Close()
		result.Audio = append([]byte(nil), outcome.Result.AudioData...)
		if publish != nil {
			result.Audio = pcmWAV(result.Audio)
		}
		result.DurationMS = float64(outcome.Result.AudioDuration) / float64(time.Millisecond)
		result.ProviderResultID = outcome.Result.ResultID
		result.ProviderStatus = outcome.Result.Reason.String()
		if outcome.Result.Reason != common.SynthesizingAudioCompleted {
			if details, detailErr := speech.NewCancellationDetailsFromSpeechSynthesisResult(outcome.Result); detailErr == nil {
				// SDK enum only: never retain provider details, text, or credentials.
				result.ProviderStatus += "/" + details.ErrorCode.String()
			}
		}
	}
	lock.Lock()
	result.Visemes = append([]Viseme(nil), events...)
	result.Words = normalizeWords(input, words, result.DurationMS)
	lock.Unlock()
	sort.SliceStable(result.Visemes, func(i, j int) bool { return result.Visemes[i].OffsetMS < result.Visemes[j].OffsetMS })
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if progressError != nil {
		return result, progressError
	}
	if outcome.Error != nil || outcome.Result == nil || outcome.Result.Reason != common.SynthesizingAudioCompleted {
		return result, errors.New("Azure synthesis did not complete.")
	}
	return result, nil
}
