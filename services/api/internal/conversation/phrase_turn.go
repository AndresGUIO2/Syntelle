package conversation

import (
	"context"
	"encoding/json"
	"errors"
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
					if job.phrase.Version != rendercontract.CoordinatedPerformanceVersion {
						job.audio, job.err = s.Speech.Prepare(phraseContext, job.phrase.Text, language, voice, id)
					}
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
		if archiveErr := s.Store.Finish(archive, op.ID, opStatus, result, result.Usage, map[string]any{"adapter": adapter, "direction": "performance-intent/3"}, nil); archiveErr != nil && modelErr == nil {
			modelErr = archiveErr
		}
		modelDone <- phraseModelResult{result, modelErr}
	}()
	clock := 0.
	s.beginPerformance(cid, id)
	budget := newReplyGestures()
	continuous := s.Performance != nil && s.Performance.MotionPolicy == rendercontract.ConversationalMotionVersion
	acoustic := newAcousticSchedule(id, continuous)
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
		for {
			audioValue := job.audio
			var attempt *phraseAttempt
			if job.phrase.Version == rendercontract.CoordinatedPerformanceVersion {
				var prepareErr error
				attempt, prepareErr = s.prepareDirectedPhrase(ctx, cid, id, voice, language, job.phrase, count)
				if errors.Is(prepareErr, ErrDeliverySuperseded) {
					continue
				}
				if prepareErr != nil {
					status = "speech_failed"
					emit("error", "Directed speech preparation failed; no retry was made.", nil)
					return
				}
				audioValue = attempt.audio
				// Turn cancellation joins any outstanding reservation watcher on all exits.
				defer func(a *phraseAttempt) { a.cancel(); <-a.done; a.ticket.Close() }(attempt)
			}
			beforeBudget, beforeAcoustic := *budget, *acoustic
			if job.err != nil {
				status = "speech_failed"
				emit("error", "Speech preparation failed; no retry was made.", nil)
				return
			}
			raw, marshalErr := json.Marshal(audioValue)
			if marshalErr != nil {
				emit("error", "Speech payload could not be encoded.", nil)
				return
			}
			var audio struct {
				Audio    []byte                        `json:"audio_base64"`
				Duration float64                       `json:"duration_ms"`
				Words    []rendercontract.WordBoundary `json:"word_boundaries"`
			}
			if json.Unmarshal(raw, &audio) != nil || audio.Duration <= 0 {
				emit("error", "Invalid prepared phrase audio.", nil)
				return
			}
			intent := PerformanceIntent{Version: phraseVersion(job.phrase.Version), Purpose: job.phrase.Purpose, Emotion: job.phrase.Emotion, Text: job.phrase.Text, Tone: job.phrase.Tone, Cues: job.phrase.Cues}
			validated := ValidatePerformance(intent, job.phrase.Text)
			if continuous && rendercontract.NaturalPerformance(intent.Version) {
				validated.Motion = &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: id, Offset: clock / 1000}
			}
			direction := resolvePerformance(validated, audio.Audio, audio.Duration/1000, acoustic, clock/1000, audio.Words...)
			direction = s.filterCapabilities(direction)
			direction = budget.admit(direction, clock/1000)
			var payload map[string]any
			if json.Unmarshal(raw, &payload) != nil {
				emit("error", "Invalid speech payload.", nil)
				return
			}
			payload["performance"] = direction
			if attempt != nil {
				payload["delivery"] = attempt.ticket.Reservation
			}
			payload["emotion_phrase_index"] = count
			direction, err = s.appendLaughter(cid, id, voice, language, payload, direction, attempt == nil)
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
			deliveryKey := fmt.Sprintf("%s-phrase-%d", id, count)
			if attempt != nil {
				deliveryKey = attempt.ticket.Reservation.Token
			}
			delivery, reserveErr := s.Store.Reserve(ctx, "speech-delivery", deliveryKey, "progressive-speech/4", cid, id, map[string]any{"index": count, "direction": intent, "model_operation_id": op.ID}, false)
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
			if attempt != nil {
				approvedReservation := attempt.ticket.Reservation
				activationErr := attempt.activation(ctx)
				attempt.close()
				attempt = nil
				if errors.Is(activationErr, ErrDeliverySuperseded) {
					*budget = beforeBudget
					*acoustic = beforeAcoustic
					invalidated, recordErr := s.Store.Reserve(ctx, "delivery-superseded", deliveryKey+"-superseded", rendercontract.DeliveryVersion, cid, id, map[string]any{"delivery_operation_id": delivery.ID, "reason": "superseded_by_settings"}, false)
					if recordErr == nil {
						recordErr = s.Store.Finish(ctx, invalidated.ID, "received", map[string]string{"played": "false"}, nil, nil, nil)
					}
					if recordErr != nil {
						emit("error", "Superseded speech evidence could not be saved.", nil)
						return
					}
					continue
				}
				if activationErr == nil {
					receiptCtx, stopReceipt := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
					receipt, receiptErr := s.Store.Reserve(receiptCtx, "delivery-activation", approvedReservation.Token, rendercontract.DeliveryVersion, cid, id, map[string]any{"index": count, "text": job.phrase.Text, "token": approvedReservation.Token, "delivery_operation_id": delivery.ID}, false)
					if receiptErr == nil {
						receiptErr = s.Store.Finish(receiptCtx, receipt.ID, "received", approvedReservation, nil, map[string]string{"source": "authenticated-post-send-activation"}, nil)
					}
					stopReceipt()
					if receiptErr != nil {
						emit("persistence_warning", "Played delivery evidence could not be committed.", nil)
						return
					}
				}
				if activationErr == nil {
					for _, cue := range direction.Resolved {
						if cue.Kind == "laugh" {
							if !s.reserveLaughter(cid, id) {
								emit("persistence_warning", "Laughter playback accounting could not be committed.", nil)
							}
							break
						}
					}
				}
				if activationErr != nil {
					emit("error", "Approved speech could not start; no retry was made.", nil)
					return
				}
			}
			clock += audio.Duration
			count++
			break
		}
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
	window    rendercontract.GestureWindow
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
			continuous := v.Motion.Valid() && rendercontract.NaturalPerformance(v.Intent.Version)
			if continuous && !b.window.Admit(at, cue.Duration, repeats) {
				continue
			}
			if !continuous && (repeats == 0 || b.heads+repeats > 8 || at-b.last < 2.2 || at < b.end) {
				continue
			}
			b.heads += repeats
			b.last = at
			b.end = at + cue.Duration
		}
		if cue.Kind == "laugh" {
			if cue.Amount != nil && *cue.Amount <= 0 {
				v.Omitted = append(v.Omitted, "zero_laughter")
				continue
			}
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
