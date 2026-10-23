package speech

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed speech.sql
var speechSchema string

const speechBudget = "authorized-dragon-continuation-2026-10-20"
const previousSpeechBudget = "authorized-dragon-motion-2026-10-19"
const speechRequestLimit = 20
const speechCharacterLimit = 5000

type TrialLedger struct{ pool *pgxpool.Pool }

func (l *TrialLedger) Close() { l.pool.Close() }

func speechAllowanceError(count, total, active, requested, maxRequests, maxCharacters int) error {
	if count >= maxRequests {
		return fmt.Errorf("All %d authorized speech requests have been used. Saved recordings can still be replayed; new text needs an additional speech allowance.", maxRequests)
	}
	if total+requested > maxCharacters {
		return fmt.Errorf("This text would exceed the authorized %d-character speech allowance. Saved recordings can still be replayed.", maxCharacters)
	}
	if active > 0 {
		return errors.New("Another speech recording is still being prepared. Wait for it to finish before submitting new text.")
	}
	return nil
}

func OpenTrialLedger(ctx context.Context, dsn string) (*TrialLedger, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, errors.New("Speech PostgreSQL configuration is invalid.")
	}
	if _, err = pool.Exec(ctx, speechSchema); err != nil {
		pool.Close()
		return nil, errors.New("Speech PostgreSQL initialization failed.")
	}
	return &TrialLedger{pool}, nil
}
func (l *TrialLedger) Reserve(ctx context.Context, input Input, binding string, characters int) (int64, *Result, error) {
	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return 0, nil, errors.New("Speech database unavailable.")
	}
	defer tx.Rollback(ctx)
	var halted bool
	var maxRequests, maxCharacters int
	err = tx.QueryRow(ctx, "SELECT halted,max_requests,max_characters FROM syntelle_speech.budgets WHERE id=$1 FOR UPDATE", speechBudget).Scan(&halted, &maxRequests, &maxCharacters)
	if err != nil {
		return 0, nil, errors.New("Speech budget unavailable.")
	}
	if maxRequests != speechRequestLimit || maxCharacters != speechCharacterLimit {
		return 0, nil, errors.New("Persisted speech limits differ from the authorized trial.")
	}
	var id int64
	var oldBinding, status string
	var response, audio []byte
	// Reuse exact matches from either authorized allowance, without counting
	// historical recordings against the newly granted continuation allowance.
	err = tx.QueryRow(ctx, "SELECT id,binding,status,response,audio FROM syntelle_speech.requests WHERE budget_id IN ($3,$4) AND (request_key=$1 OR binding=$2) ORDER BY (request_key=$1) DESC LIMIT 1", input.RequestKey, binding, speechBudget, previousSpeechBudget).Scan(&id, &oldBinding, &status, &response, &audio)
	if err == nil {
		if oldBinding != binding {
			return 0, nil, errors.New("Idempotency key belongs to different speech.")
		}
		if status != "received" {
			return 0, nil, errors.New("This synthesis already started or failed; no retry is allowed.")
		}
		var result Result
		if json.Unmarshal(response, &result) != nil {
			return 0, nil, errors.New("Cached speech record is invalid.")
		}
		result.Audio = audio
		return id, &result, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, errors.New("Speech cache unavailable.")
	}
	if halted {
		return 0, nil, errors.New("A provider failure stopped this speech trial.")
	}
	var count, total, active int
	err = tx.QueryRow(ctx, "SELECT count(*),coalesce(sum(characters),0),count(*) FILTER(WHERE status='started') FROM syntelle_speech.requests WHERE budget_id=$1", speechBudget).Scan(&count, &total, &active)
	if err != nil {
		return 0, nil, errors.New("Speech allowance could not be checked. No recording was requested.")
	}
	if err = speechAllowanceError(count, total, active, characters, maxRequests, maxCharacters); err != nil {
		return 0, nil, err
	}
	request, _ := json.Marshal(struct {
		Input                                    Input
		Region, SDKVersion, NativeSHA256, Format string
	}{input, input.region, speechSDKVersion, speechNativeSHA256, "riff-24khz-16bit-mono-pcm"})
	err = tx.QueryRow(ctx, "INSERT INTO syntelle_speech.requests(budget_id,request_key,binding,characters,request,status) VALUES($1,$2,$3,$4,$5,'started') RETURNING id", speechBudget, input.RequestKey, binding, characters, request).Scan(&id)
	if err != nil {
		return 0, nil, errors.New("Speech reservation could not be committed.")
	}
	return id, nil, tx.Commit(ctx)
}
func (l *TrialLedger) Finish(ctx context.Context, id int64, result *Result, message string, halt bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	status := "received"
	if message != "" {
		status = "failed"
	}
	record := *result
	record.Audio = nil
	response, _ := json.Marshal(record)
	_, err = tx.Exec(ctx, "UPDATE syntelle_speech.requests SET status=$2,response=$3,audio=$4,error=$5,finished_at=now() WHERE id=$1 AND status='started'", id, status, response, result.Audio, message)
	if err != nil {
		return err
	}
	if halt {
		if _, err = tx.Exec(ctx, "UPDATE syntelle_speech.budgets SET halted=true WHERE id=$1", speechBudget); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
