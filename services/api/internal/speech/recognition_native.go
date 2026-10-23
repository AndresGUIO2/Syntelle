//go:build speech && linux

package speech

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Microsoft/cognitive-services-speech-sdk-go/audio"
	"github.com/Microsoft/cognitive-services-speech-sdk-go/common"
	"github.com/Microsoft/cognitive-services-speech-sdk-go/speech"
	"sync/atomic"
	"syntelle/api/internal/conversation"
	"time"
)

type azureRecognition struct{ key, region string }

func NewAzureRecognition(key, region string) conversation.Recognition {
	return &azureRecognition{key, region}
}
func (p *azureRecognition) Run(ctx context.Context, language string, pcm <-chan []byte, emit func(conversation.RecognitionEvent)) error {
	config, e := speech.NewSpeechConfigFromSubscription(p.key, p.region)
	if e != nil {
		return errors.New("Recognition configuration failed.")
	}
	defer config.Close()
	if e = config.SetSpeechRecognitionLanguage(language); e != nil {
		return e
	}
	if e = config.SetProperty(common.SegmentationSilenceTimeoutMs, "700"); e != nil {
		return e
	}
	format, e := audio.GetWaveFormatPCM(16000, 16, 1)
	if e != nil {
		return e
	}
	defer format.Close()
	stream, e := audio.CreatePushAudioInputStreamFromFormat(format)
	if e != nil {
		return e
	}
	defer stream.Close()
	input, e := audio.NewAudioConfigFromStreamInput(stream)
	if e != nil {
		return e
	}
	defer input.Close()
	recognizer, e := speech.NewSpeechRecognizerFromConfig(config, input)
	if e != nil {
		return e
	}
	defer recognizer.Close()
	failed := make(chan struct{}, 1)
	var bytes atomic.Int64
	defer func() { emit(conversation.RecognitionEvent{Type: "usage", DurationMS: float64(bytes.Load()) / 32}) }()
	recognizer.Canceled(func(event speech.SpeechRecognitionCanceledEventArgs) {
		defer event.Close()
		select {
		case failed <- struct{}{}:
		default:
		}
	})
	recognizer.SpeechStartDetected(func(event speech.RecognitionEventArgs) {
		defer event.Close()
		emit(conversation.RecognitionEvent{Type: "start"})
	})
	recognizer.Recognizing(func(event speech.SpeechRecognitionEventArgs) {
		defer event.Close()
		emit(conversation.RecognitionEvent{Type: "partial", ID: event.Result.ResultID, Text: event.Result.Text})
	})
	recognizer.Recognized(func(event speech.SpeechRecognitionEventArgs) {
		defer event.Close()
		if event.Result.Reason == common.RecognizedSpeech && event.Result.Text != "" {
			raw := json.RawMessage(event.Result.Properties.GetProperty(common.SpeechServiceResponseJSONResult, "{}"))
			if !json.Valid(raw) {
				raw = json.RawMessage(`{}`)
			}
			emit(conversation.RecognitionEvent{Type: "final", ID: event.Result.ResultID, Text: event.Result.Text, OffsetMS: float64(event.Result.Offset) / float64(time.Millisecond), DurationMS: float64(event.Result.Duration) / float64(time.Millisecond), Raw: raw})
		} else {
			emit(conversation.RecognitionEvent{Type: "end"})
		}
	})
	// Start/stop are joined before freeing SDK handles. The HTTP shutdown has an
	// independent hard five-second process deadline if a native call stalls.
	pending := recognizer.StartContinuousRecognitionAsync()
	select {
	case e = <-pending:
	case <-ctx.Done():
		<-pending
		e = ctx.Err()
	}
	if e != nil {
		return e
	}
	defer func() { stream.CloseStream(); <-recognizer.StopContinuousRecognitionAsync() }()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-failed:
			return errors.New("Azure recognition cancelled.")
		case data, ok := <-pcm:
			if !ok {
				return nil
			}
			if e = stream.Write(data); e != nil {
				return errors.New("Recognition stream write failed.")
			}
			bytes.Add(int64(len(data)))
		}
	}
}
