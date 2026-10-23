package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const sessionTimeLimit = 5 * time.Minute
const visitorTimeLimit = 15 * time.Minute
const sessionLimitReason = "session time limit"

var errDemoTime = errors.New("Your 15-minute demo allowance is exhausted or in use by another session.")
var errSessionTime = errors.New("This session has reached its 5-minute limit. Start a new session to continue.")

// Production is bounded unless the server explicitly selects development mode.
func TimeLimitsDisabledFromEnv() (bool, error) {
	raw := os.Getenv("SYNTELLE_STREAM_TIME_LIMITS_ENABLED")
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errors.New("SYNTELLE_STREAM_TIME_LIMITS_ENABLED must be true or false")
	}
	return !enabled, nil
}

// Development sessions retain lifecycle records without reserving, consuming or
// resetting production allowance. Previously metered evidence remains intact.
func (s *recordStore) allocateUnmetered(ctx context.Context, session *streamSession, packageHash string, resume bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockUsage(ctx, tx, session.principal.ID); err != nil {
		return err
	}
	settings, err := json.Marshal(session.settings)
	if err != nil {
		return err
	}
	if resume {
		result, err := tx.Exec(ctx, `UPDATE syntelle_streaming.sessions SET settings=$3,browser_session_id=$4,key_id=$5,principal_kind=$6,state='starting',reason='',closed_at=NULL,updated_at=now() WHERE id=$1 AND principal_id=$2 AND state='paused' AND closed_at IS NOT NULL AND reason=$7`, session.id, session.principal.ID, settings, session.principal.SessionID, session.principal.KeyID, session.principal.Kind, inactivityReason)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
	} else {
		if _, err = tx.Exec(ctx, `INSERT INTO syntelle_streaming.sessions(id,conversation_id,principal_id,principal_kind,browser_session_id,key_id,settings,package_hash,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'starting')`, session.id, session.cid, session.principal.ID, session.principal.Kind, session.principal.SessionID, session.principal.KeyID, settings, packageHash); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO syntelle_streaming.events(session_id,kind,data) VALUES($1,'development-time-limits-disabled','{}')`, session.id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// A principal-wide transaction lock serializes allocation and settlement across
// tabs, browser sessions and account keys. No client time or identity is trusted.
func lockUsage(ctx context.Context, tx pgx.Tx, principal string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,187200052))`, principal)
	return err
}

func (s *recordStore) allocate(ctx context.Context, session *streamSession, packageHash string, resume bool) (time.Time, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockUsage(ctx, tx, session.principal.ID); err != nil {
		return time.Time{}, err
	}
	var sessionUsed, allocated int64
	if resume {
		err = tx.QueryRow(ctx, `SELECT used_ms FROM syntelle_streaming.sessions WHERE id=$1 AND principal_id=$2 AND state='paused' AND closed_at IS NOT NULL AND reason=$3 FOR UPDATE`, session.id, session.principal.ID, inactivityReason).Scan(&sessionUsed)
		if err != nil {
			return time.Time{}, err
		}
	}
	if sessionUsed >= sessionTimeLimit.Milliseconds() {
		return time.Time{}, errSessionTime
	}
	if err = tx.QueryRow(ctx, `SELECT coalesce(sum(used_ms+lease_ms),0) FROM syntelle_streaming.sessions WHERE principal_id=$1`, session.principal.ID).Scan(&allocated); err != nil {
		return time.Time{}, err
	}
	grant := min(sessionTimeLimit.Milliseconds()-sessionUsed, visitorTimeLimit.Milliseconds()-allocated)
	if grant <= 0 {
		return time.Time{}, errDemoTime
	}
	settings, err := json.Marshal(session.settings)
	if err != nil {
		return time.Time{}, err
	}
	var started time.Time
	if resume {
		err = tx.QueryRow(ctx, `UPDATE syntelle_streaming.sessions SET settings=$3,browser_session_id=$4,key_id=$5,principal_kind=$6,state='starting',reason='',closed_at=NULL,updated_at=now(),lease_ms=$7,lease_started_at=clock_timestamp() WHERE id=$1 AND principal_id=$2 RETURNING lease_started_at`, session.id, session.principal.ID, settings, session.principal.SessionID, session.principal.KeyID, session.principal.Kind, grant).Scan(&started)
	} else {
		err = tx.QueryRow(ctx, `INSERT INTO syntelle_streaming.sessions(id,conversation_id,principal_id,principal_kind,browser_session_id,key_id,settings,package_hash,state,lease_ms,lease_started_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'starting',$9,clock_timestamp()) RETURNING lease_started_at`, session.id, session.cid, session.principal.ID, session.principal.Kind, session.principal.SessionID, session.principal.KeyID, settings, packageHash, grant).Scan(&started)
	}
	if err != nil {
		return time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}
	return started.Add(time.Duration(grant) * time.Millisecond), nil
}
