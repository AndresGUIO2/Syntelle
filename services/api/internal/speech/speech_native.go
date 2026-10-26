//go:build speech && linux

package speech

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/Microsoft/cognitive-services-speech-sdk-go/common"
	"github.com/Microsoft/cognitive-services-speech-sdk-go/speech"
)

type azureSpeechProvider struct {
	key, region string
	synthesis   sync.Mutex
}

func NewAzureProvider(key, region string) (Provider, error) {
	return &azureSpeechProvider{key: key, region: region}, nil
}
func (p *azureSpeechProvider) Synthesize(ctx context.Context, input Input) (Result, error) {
	result := Result{Mime: "audio/wav", Voice: input.Voice, Language: input.Language, SDKVersion: speechSDKVersion}
	// HD synthesis can reject bursts even when the resource's published TPS
	// allowance is higher. One API replica admits one paid call at a time.
	p.synthesis.Lock()
	defer p.synthesis.Unlock()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	config, err := speech.NewSpeechConfigFromSubscription(p.key, p.region)
	if err != nil {
		return result, errors.New("Azure Speech configuration failed.")
	}
	defer config.Close()
	for _, err = range []error{config.SetSpeechSynthesisVoiceName(input.Voice), config.SetSpeechSynthesisLanguage(input.Language), config.SetSpeechSynthesisOutputFormat(common.Riff24Khz16BitMonoPcm)} {
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
	synthesizer.VisemeReceived(func(event speech.SpeechSynthesisVisemeEventArgs) {
		defer event.Close()
		lock.Lock()
		defer lock.Unlock()
		events = append(events, Viseme{event.VisemeID, float64(event.AudioOffset) / 10000})
	})
	// Text-only synthesis intentionally mirrors OrgLens; no SVG/blendshape SSML request.
	pending := synthesizer.SpeakTextAsync(input.Text)
	var outcome speech.SpeechSynthesisOutcome
	select {
	case outcome = <-pending:
	case <-ctx.Done():
		// Join both native operations before releasing the synthesizer handle.
		<-synthesizer.StopSpeakingAsync()
		outcome = <-pending
	}
	if outcome.Result != nil {
		defer outcome.Result.Close()
		result.Audio = append([]byte(nil), outcome.Result.AudioData...)
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
	lock.Unlock()
	sort.SliceStable(result.Visemes, func(i, j int) bool { return result.Visemes[i].OffsetMS < result.Visemes[j].OffsetMS })
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if outcome.Error != nil || outcome.Result == nil || outcome.Result.Reason != common.SynthesizingAudioCompleted {
		return result, errors.New("Azure synthesis did not complete.")
	}
	return result, nil
}
