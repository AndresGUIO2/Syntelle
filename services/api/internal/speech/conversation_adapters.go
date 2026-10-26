package speech

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/conversation"
)

type localOwnerSpeechLedger struct{ store *conversation.Store }

func NewOwnerLedger(store *conversation.Store) Ledger {
	return &localOwnerSpeechLedger{store: store}
}

func (l *localOwnerSpeechLedger) Reserve(ctx context.Context, input Input, binding string, characters int) (int64, *Result, error) {
	// Only the two genuine, authorized ledgers can supply historical audio.
	var response, audio []byte
	var previousBinding, previousStatus string
	e := l.store.Pool.QueryRow(ctx, `SELECT response,audio,binding,status FROM syntelle_speech.requests WHERE budget_id IN ($1,$2) AND (request_key=$4 OR (binding=$3 AND status='received')) ORDER BY (request_key=$4) DESC,id LIMIT 1`, speechBudget, previousSpeechBudget, binding, input.RequestKey).Scan(&response, &audio, &previousBinding, &previousStatus)
	if e == nil {
		if previousBinding != binding {
			return 0, nil, errors.New("Idempotency key belongs to different historical speech.")
		}
		if previousStatus != "received" {
			return 0, nil, errors.New("This historical request already started or failed; no retry is allowed.")
		}
		var r Result
		if json.Unmarshal(response, &r) != nil {
			return 0, nil, errors.New("Historical speech cache is invalid.")
		}
		r.Audio = audio
		if e = validateSpeechResult(r); e != nil {
			return 0, nil, e
		}
		return 0, &r, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return 0, nil, errors.New("Historical speech cache is unavailable.")
	}
	cid, tid := "", ""
	if ref, ok := ctx.Value(speechTurnKey{}).(speechTurnRef); ok {
		cid, tid = ref.conversation, ref.turn
	}
	op, e := l.store.Reserve(ctx, "dragon", input.RequestKey, binding, cid, tid, map[string]any{"input": input, "region": input.region, "sdk_version": speechSDKVersion, "native_sha256": speechNativeSHA256, "format": "riff-24khz-16bit-mono-pcm", "characters": characters, "policy": "local-owner-uncapped"}, true)
	if errors.Is(e, conversation.ErrOperationInProgress) {
		return l.waitForSharedSpeech(ctx, op.ID, binding)
	}
	if e != nil {
		return 0, nil, e
	}
	if op.Status == "received" {
		var r Result
		if json.Unmarshal(op.Result, &r) != nil {
			return 0, nil, errors.New("Speech cache is invalid.")
		}
		r.Audio = op.Audio
		if e = validateSpeechResult(r); e != nil {
			return 0, nil, e
		}
		return op.ID, &r, nil
	}
	return op.ID, nil, nil
}

// A matching phrase already sent to Azure is never sent a second time. Wait
// for that exact durable operation, so an unrelated later request cannot turn
// its failure or cancellation into an automatic paid retry.
func (l *localOwnerSpeechLedger) waitForSharedSpeech(ctx context.Context, id int64, binding string) (int64, *Result, error) {
	wait, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		var status, recordedBinding string
		var response, audio []byte
		err := l.store.Pool.QueryRow(wait, `SELECT status,result,audio,binding FROM syntelle_conversation.operations WHERE id=$1 AND kind='dragon'`, id).Scan(&status, &response, &audio, &recordedBinding)
		if err != nil {
			return 0, nil, err
		}
		if recordedBinding != binding {
			return 0, nil, errors.New("Shared speech binding changed; no retry was made.")
		}
		switch status {
		case "received":
			var result Result
			if json.Unmarshal(response, &result) != nil {
				return 0, nil, errors.New("Shared speech cache is invalid.")
			}
			result.Audio = audio
			if err := validateSpeechResult(result); err != nil {
				return 0, nil, err
			}
			return id, &result, nil
		case "started":
			select {
			case <-wait.Done():
				return 0, nil, wait.Err()
			case <-ticker.C:
			}
		default:
			return 0, nil, errors.New("Shared speech attempt did not succeed; no retry was made.")
		}
	}
}
func (l *localOwnerSpeechLedger) Finish(ctx context.Context, id int64, r *Result, message string, _ bool) error {
	record := *r
	record.Audio = nil
	status := "received"
	if message != "" {
		status = "failed"
	}
	if strings.Contains(message, "cancelled") {
		status = "cancelled"
	}
	return l.store.Finish(ctx, id, status, record, map[string]any{"duration_ms": r.DurationMS}, map[string]string{"message": message}, r.Audio)
}

type speechTurnKey struct{}
type speechTurnRef struct{ conversation, turn string }
type conversationSpeech struct{ service *Service }

func (s conversationSpeech) AcquireVoice(ctx context.Context, conversationID string) (func(), error) {
	return s.service.voice.acquire(ctx, conversationID)
}

func NewConversationSpeech(service *Service) conversation.Speech {
	return conversationSpeech{service: service}
}

func (s conversationSpeech) Prepare(ctx context.Context, text, language, voice, id string) (any, error) {
	return s.prepare(ctx, text, language, voice, id, splitSpeech(text), nil)
}

func (s conversationSpeech) PrepareParts(ctx context.Context, text, language, voice, id string, publish func(conversation.SpeechPart) error) (any, error) {
	return s.prepare(ctx, text, language, voice, id, splitProgressiveSpeech(text), publish)
}

func (s conversationSpeech) prepare(ctx context.Context, text, language, voice, id string, chunks []string, publish func(conversation.SpeechPart) error) (any, error) {
	// runTurn supplies the session ID through a context value shared only within Go.
	if cid, ok := conversation.IDs(ctx); ok {
		ctx = context.WithValue(ctx, speechTurnKey{}, speechTurnRef{cid, id})
	}
	if phraseKey, ok := conversation.PhraseSpeechKey(ctx); ok {
		id = phraseKey
	}
	if len(chunks) == 0 {
		return nil, errors.New("speech text is empty")
	}
	parts := make([]Result, 0, len(chunks))
	keys := make([]string, 0, len(chunks))
	if publish != nil {
		var err error
		parts, keys, err = s.prepareConcurrentParts(ctx, chunks, language, voice, id, publish)
		if err != nil {
			return nil, err
		}
	} else {
		for index, chunk := range chunks {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			key := "chat-" + id
			if len(chunks) > 1 || len(key) > 128 {
				hash := sha256.Sum256([]byte(id))
				key = fmt.Sprintf("chat-%x-part-%03d", hash, index)
			}
			part, err := s.service.synthesizeBounded(ctx, Input{Text: chunk, Language: language, Voice: voice, RequestKey: key}, conversationPartUnits, 60*time.Second)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
			keys = append(keys, key)
		}
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := joinSpeech(parts, keys)
	if err != nil {
		return nil, err
	}
	if owner, ok := s.service.ledger.(*localOwnerSpeechLedger); ok {
		bindingBytes, err := json.Marshal(result.Parts)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(bindingBytes)
		cid, tid := "", ""
		if ref, ok := ctx.Value(speechTurnKey{}).(speechTurnRef); ok {
			cid = ref.conversation
			tid = ref.turn
		}
		op, err := owner.store.Reserve(ctx, "dragon-sequence", "chat-"+id, hex.EncodeToString(hash[:]), cid, tid, map[string]any{"parts": result.Parts, "text": text}, false)
		if err != nil {
			return nil, err
		}
		if op.Status != "received" {
			record := result
			record.Audio = nil
			if err = owner.store.Finish(ctx, op.ID, "received", record, nil, map[string]string{"origin": "derived-native-parts"}, result.Audio); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
