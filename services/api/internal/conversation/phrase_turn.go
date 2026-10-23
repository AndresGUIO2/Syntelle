package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"syntelle/render-contract"
	"time"
)

type phraseDirector interface {
	PhraseDirection() bool
	PhraseRequest(string, []Message) any
	CompleteWithPhrases(context.Context, string, []Message, func(PerformancePhrase) error) (ModelResult, error)
}
type phraseJob struct {
	phrase PerformancePhrase
	index  int
	ready  chan struct{}
	audio  any
	err    error
}
type phraseModelResult struct {
	value ModelResult
	err   error
}

// One model producer, two synthesis workers and one ordered delivery consumer.
// The owner cancels and joins all producers on every exit.
func (s *Service) runPhraseTurn(ctx context.Context, model phraseDirector, cid, id, language, voice, instructions string, send func(Event)) {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	output, status := "", "failed"
	emit := func(kind, text string, data any) {
		if ctx.Err() == nil {
			send(Event{Type: kind, ConversationID: cid, TurnID: id, Text: text, Data: data})
		}
	}
	defer func() {
		wasCanceled := parent.Err() != nil
		cancel()
		workers.Wait()
		if status == "ready" {
			return
		}
		if wasCanceled {
			status = "interrupted"
		}
		archive, stop := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer stop()
		if err := s.Store.FinishTurn(archive, id, status, output, output, ""); err != nil {
			emit("persistence_warning", "Reply state could not be saved.", nil)
		}
	}()
	turns, err := s.Store.Turns(ctx, cid, 21)
	if err != nil {
		emit("error", "Conversation context could not be loaded.", nil)
		return
	}
	messages := ContextMessages(turns)
	instructions += "\nSelected language: " + language
	kind, adapter := "director", "trusted-phrase-controller/1"
	if provider, ok := s.Model.(interface{ Identity() (string, string) }); ok {
		kind, adapter = provider.Identity()
	}
	op, err := s.Store.Reserve(ctx, kind, id, "", cid, id, map[string]any{"body": model.PhraseRequest(instructions, messages), "settings": s.Model.Settings()}, false)
	if err != nil {
		emit("error", "Model request could not be committed.", nil)
		return
	}
	jobs, ordered := make(chan *phraseJob, 2), make(chan *phraseJob, 2)
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					phraseContext := context.WithValue(ctx, phraseSpeechKey{}, fmt.Sprintf("%s-phrase-%d", id, job.index))
					job.audio, job.err = s.Speech.Prepare(phraseContext, job.phrase.Text, language, voice, id)
					close(job.ready)
				}
			}
		}()
	}
	modelDone := make(chan phraseModelResult, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		defer close(jobs)
		defer close(ordered)
		index := 0
		result, modelErr := model.CompleteWithPhrases(ctx, instructions, messages, func(phrase PerformancePhrase) error {
			prefix := ""
			if index > 0 {
				prefix = " "
			}
			emit("reply_delta", prefix+phrase.Text, nil)
			job := &phraseJob{phrase: phrase, index: index, ready: make(chan struct{})}
			index++
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- job:
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ordered <- job:
				return nil
			}
		})
		opStatus := "received"
		if modelErr != nil {
			opStatus = "failed"
		}
		archive, stop := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer stop()
		if archiveErr := s.Store.Finish(archive, op.ID, opStatus, result, result.Usage, map[string]any{"adapter": adapter, "direction": "performance-intent/2"}, nil); archiveErr != nil && modelErr == nil {
			modelErr = archiveErr
		}
		modelDone <- phraseModelResult{result, modelErr}
	}()
	clock := 0.
	s.beginPerformance(cid, id)
	budget := newReplyGestures()
	acoustic := newAcousticSchedule(id)
	count := 0
	for {
		var job *phraseJob
		var ok bool
		select {
		case <-ctx.Done():
			return
		case job, ok = <-ordered:
		}
		if !ok {
			break
		}
		prefix := ""
		if output != "" {
			prefix = " "
		}
		output += prefix + job.phrase.Text
		select {
		case <-ctx.Done():
			return
		case <-job.ready:
		}
		if job.err != nil {
			emit("error", "Speech preparation failed; no retry was made.", nil)
			return
		}
		raw, marshalErr := json.Marshal(job.audio)
		if marshalErr != nil {
			emit("error", "Speech payload could not be encoded.", nil)
			return
		}
		var audio struct {
			Audio    []byte  `json:"audio_base64"`
			Duration float64 `json:"duration_ms"`
		}
		if json.Unmarshal(raw, &audio) != nil || audio.Duration <= 0 {
			emit("error", "Invalid prepared phrase audio.", nil)
			return
		}
		intent := PerformanceIntent{Version: "performance-intent/2", Text: job.phrase.Text, Tone: job.phrase.Tone, Cues: job.phrase.Cues}
		direction := resolvePerformance(ValidatePerformance(intent, job.phrase.Text), audio.Audio, audio.Duration/1000, acoustic, clock/1000)
		direction = s.filterCapabilities(direction)
		direction = budget.admit(direction, clock/1000)
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			emit("error", "Invalid speech payload.", nil)
			return
		}
		payload["performance"] = direction
		direction, err = s.appendLaughter(cid, id, voice, language, payload, direction)
		if err != nil {
			emit("error", "Cached laughter could not be prepared.", nil)
			return
		}
		payload["performance"] = direction
		if duration, ok := payload["duration_ms"].(float64); ok {
			audio.Duration = duration
		}
		part := SpeechPart{Speech: payload, Text: job.phrase.Text, Index: count, OffsetMS: clock}
		evidence, evidenceErr := speechDeliveryEvidence(part)
		if evidenceErr != nil {
			emit("error", "Speech evidence could not be encoded.", nil)
			return
		}
		delivery, reserveErr := s.Store.Reserve(ctx, "speech-delivery", fmt.Sprintf("%s-phrase-%d", id, count), "progressive-speech/4", cid, id, map[string]any{"index": count, "direction": intent, "model_operation_id": op.ID}, false)
		if reserveErr != nil {
			emit("error", "Speech delivery could not be reserved.", nil)
			return
		}
		if err = s.Store.Finish(ctx, delivery.ID, "received", evidence, nil, map[string]any{"clock": "audio", "model_operation_id": op.ID}, nil); err != nil {
			emit("error", "Speech evidence could not be committed.", nil)
			return
		}
		if err = s.Store.FinishTurn(ctx, id, "preparing", output, output, ""); err != nil {
			emit("error", "Reply prefix could not be committed.", nil)
			return
		}
		emit("speech_part", part.Text, part)
		clock += audio.Duration
		count++
	}
	completed := <-modelDone
	if completed.err != nil {
		emit("error", "Model reply stopped; no retry was made.", nil)
		return
	}
	if completed.value.Text != output {
		emit("error", "Completed reply differs from its delivered phrases.", nil)
		return
	}
	if err = s.Store.FinishTurn(ctx, id, "ready", output, output, ""); err != nil {
		emit("error", "Reply could not be committed.", nil)
		return
	}
	status = "ready"
	emit("reply_complete", output, map[string]string{"spoken": output})
	emit("speech_complete", output, map[string]int{"parts": count})
}

type replyGestures struct {
	heads     int
	last, end float64
	laughed   bool
}

func newReplyGestures() *replyGestures { return &replyGestures{last: -3, end: -3} }
func (b *replyGestures) admit(v PerformanceValidation, offset float64) PerformanceValidation {
	kept := make([]ResolvedCue, 0, len(v.Resolved))
	for _, cue := range v.Resolved {
		at := offset + cue.Time
		if headCue(cue.Kind) {
			repeats := rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)
			if repeats == 0 || b.heads+repeats > 8 || at-b.last < 2.2 || at < b.end {
				continue
			}
			b.heads += repeats
			b.last = at
			b.end = at + cue.Duration
		}
		if cue.Kind == "laugh" {
			if b.laughed {
				continue
			}
			b.laughed = true
		}
		kept = append(kept, cue)
	}
	v.Resolved = kept
	return v
}
