package conversation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type SpeechPart struct {
	Speech   any     `json:"speech"`
	Text     string  `json:"text"`
	Index    int     `json:"index"`
	Final    bool    `json:"final"`
	OffsetMS float64 `json:"offset_ms"`
}

type progressiveSpeech interface {
	PrepareParts(context.Context, string, string, string, string, func(SpeechPart) error) (any, error)
}
type openingSpeech interface {
	PrepareOpening(context.Context, string, string, string, string) error
}
type openingPartSpeech interface {
	PrepareOpeningPart(context.Context, string, string, string, string) (SpeechPart, error)
}
type openingJob struct {
	text      string
	done      chan struct{}
	cancel    context.CancelFunc
	err       error
	published bool // Read only after done closes, directly or through pipeline.parts.
}

func startOpeningSpeech(parent context.Context, speech openingSpeech, text, language, voice, id string, publish ...func(SpeechPart) error) *openingJob {
	ctx, cancel := context.WithCancel(parent)
	job := &openingJob{text: text, done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(job.done)
		if provider, ok := speech.(openingPartSpeech); ok && len(publish) > 0 {
			var part SpeechPart
			part, job.err = provider.PrepareOpeningPart(ctx, text, language, voice, id)
			if job.err == nil {
				part.Text, part.Index, part.Final, part.OffsetMS = text, 0, false, 0
				job.err = publish[0](part)
				job.published = job.err == nil
			}
		} else {
			job.err = speech.PrepareOpening(ctx, text, language, voice, id)
		}
	}()
	return job
}
func (j *openingJob) stop() { j.cancel(); <-j.done }

// One worker owns synthesis and closes parts. A separately committed opening can
// already be audible; the remaining phrases still require completed model text.
type speechPipeline struct {
	text   string
	parts  <-chan SpeechPart
	done   chan struct{}
	cancel context.CancelFunc
	result any
	err    error
}

func startSpeechPipeline(ctx context.Context, speech progressiveSpeech, text, language, voice, id string, opening *openingJob) *speechPipeline {
	ctx, cancel := context.WithCancel(ctx)
	parts := make(chan SpeechPart, 1)
	p := &speechPipeline{text: text, parts: parts, done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(p.done)
		defer close(parts)
		if opening != nil {
			select {
			case <-ctx.Done():
				p.err = ctx.Err()
				return
			case <-opening.done:
			}
			if opening.err != nil {
				p.err = opening.err
				return
			}
			if OpeningPhrase(text, true) != opening.text {
				p.err = errors.New("opening phrase changed during generation")
				return
			}
		}
		p.result, p.err = speech.PrepareParts(ctx, text, language, voice, id, func(part SpeechPart) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case parts <- part:
				return nil
			}
		})
	}()
	return p
}

func (p *speechPipeline) stop() { p.cancel(); <-p.done }

// Native WAV bytes are already stored in the speech ledger. Keep their binding,
// provider identity and exact direction here, without writing base64 a second time.
func speechDeliveryEvidence(part SpeechPart) (any, error) {
	data, err := json.Marshal(part)
	if err != nil {
		return nil, err
	}
	var evidence map[string]any
	if err = json.Unmarshal(data, &evidence); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(part.Speech)
	var payload struct {
		Audio []byte `json:"audio_base64"`
	}
	if err = json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if speech, ok := evidence["speech"].(map[string]any); ok {
		delete(speech, "audio_base64")
		speech["audio_sha256"] = fmt.Sprintf("%x", sha256.Sum256(payload.Audio))
	}
	return evidence, nil
}

// Keep anchors in their actual phrase. Never replay a whole-reply gesture on
// every part; final expressions remain attached only to the last part.
func partPerformance(full *PerformanceValidation, part SpeechPart, lastHead *float64) (any, error) {
	if full == nil {
		return part.Speech, nil
	}
	data, err := json.Marshal(part.Speech)
	if err != nil {
		return nil, err
	}
	var audio struct {
		Audio    []byte  `json:"audio_base64"`
		Duration float64 `json:"duration_ms"`
	}
	if json.Unmarshal(data, &audio) != nil {
		return nil, errors.New("invalid prepared speech part")
	}
	intent := full.Intent
	intent.Text = part.Text
	intent.Cues = nil
	for _, cue := range full.Intent.Cues {
		if strings.Count(part.Text, cue.Anchor) == 1 && (part.Final || cue.Kind != "smile" && cue.Kind != "laugh") {
			intent.Cues = append(intent.Cues, cue)
		}
	}
	v := ResolvePerformance(ValidatePerformance(intent, part.Text), audio.Audio, audio.Duration/1000)
	kept := v.Resolved[:0]
	for _, cue := range v.Resolved {
		if cue.Kind == "nod" || cue.Kind == "shake" || cue.Kind == "emphasis" {
			at := part.OffsetMS/1000 + cue.Time
			if at-*lastHead < 2.2 {
				continue
			}
			*lastHead = at
		}
		kept = append(kept, cue)
	}
	v.Resolved = kept
	var payload map[string]any
	if json.Unmarshal(data, &payload) != nil {
		return nil, errors.New("invalid speech payload")
	}
	payload["performance"] = v
	return payload, nil
}
