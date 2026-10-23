package streaming

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"syntelle/api/internal/accounts"
)

//go:embed migrations/*.sql
var migrations embed.FS

type recordStore struct{ pool *pgxpool.Pool }

// paused is owner-scoped. The caller cannot choose an arbitrary conversation ID.
func (s *recordStore) paused(ctx context.Context, id, principal string) (string, string, bool, error) {
	var cid, language string
	var messages bool
	err := s.pool.QueryRow(ctx, `SELECT conversation_id,settings->>'language',EXISTS(SELECT 1 FROM syntelle_conversation.turns t WHERE t.conversation_id=s.conversation_id) FROM syntelle_streaming.sessions s WHERE id=$1 AND principal_id=$2 AND state='paused' AND closed_at IS NOT NULL AND reason=$3`, id, principal, inactivityReason).Scan(&cid, &language, &messages)
	return cid, language, messages, err
}

func openStore(ctx context.Context, pool *pgxpool.Pool) (*recordStore, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(187200051); CREATE SCHEMA IF NOT EXISTS syntelle_streaming; CREATE TABLE IF NOT EXISTS syntelle_streaming.migrations(version integer PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return nil, err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		version, parseErr := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		if parseErr != nil {
			return nil, parseErr
		}
		var applied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_streaming.migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return nil, err
		}
		if applied {
			continue
		}
		data, readErr := migrations.ReadFile("migrations/" + file.Name())
		if readErr != nil {
			return nil, readErr
		}
		if _, err = tx.Exec(ctx, string(data)); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO syntelle_streaming.migrations(version) VALUES($1)`, version); err != nil {
			return nil, err
		}
	}
	// An interrupted process cannot prove how much of an outstanding lease ran.
	// Charge its bounded reservation rather than reset a visitor's allowance.
	if _, err = tx.Exec(ctx, `UPDATE syntelle_streaming.sessions SET used_ms=used_ms+lease_ms,lease_ms=0,lease_started_at=NULL,state='failed',closed_at=now(),reason='server restarted' WHERE closed_at IS NULL; UPDATE syntelle_streaming.commands SET status='uncertain',finished_at=now() WHERE status='accepted'`); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &recordStore{pool}, nil
}

func (s *recordStore) state(ctx context.Context, id, state, reason string, closed bool) error {
	if !closed {
		_, err := s.pool.Exec(ctx, `UPDATE syntelle_streaming.sessions SET state=$2,reason=$3,updated_at=now() WHERE id=$1 AND closed_at IS NULL`, id, state, reason)
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var principal string
	if err = tx.QueryRow(ctx, `SELECT principal_id FROM syntelle_streaming.sessions WHERE id=$1`, id).Scan(&principal); err != nil {
		return err
	}
	if err = lockUsage(ctx, tx, principal); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE syntelle_streaming.sessions SET used_ms=used_ms+CASE WHEN lease_started_at IS NULL THEN 0 ELSE least(lease_ms,greatest(0,ceil(extract(epoch FROM (clock_timestamp()-lease_started_at))*1000)::bigint)) END,lease_ms=0,lease_started_at=NULL,state=$2,reason=$3,updated_at=now(),closed_at=now() WHERE id=$1`, id, state, reason); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE syntelle_streaming.commands SET status='interrupted',finished_at=now() WHERE session_id=$1 AND status='accepted'`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *recordStore) settings(ctx context.Context, id string, settings accounts.Settings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE syntelle_streaming.sessions SET settings=$2,updated_at=now() WHERE id=$1`, id, data)
	return err
}
func (s *recordStore) command(ctx context.Context, id string, c Command, turnID string) (bool, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return false, err
	}
	result, err := s.pool.Exec(ctx, `INSERT INTO syntelle_streaming.commands(session_id,id,turn_id,kind,request,status) VALUES($1,$2,$3,$4,$5,'accepted') ON CONFLICT(session_id,id) DO NOTHING`, id, c.ID, turnID, c.Type, data)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 1 {
		return true, nil
	}
	var equal bool
	if err = s.pool.QueryRow(ctx, `SELECT request=$3::jsonb FROM syntelle_streaming.commands WHERE session_id=$1 AND id=$2`, id, c.ID, data).Scan(&equal); err != nil {
		return false, err
	}
	if !equal {
		return false, errors.New("command identifier belongs to different content")
	}
	return false, nil
}
func (s *recordStore) finish(ctx context.Context, id, turnID, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE syntelle_streaming.commands SET status=$3,finished_at=now() WHERE session_id=$1 AND turn_id=$2 AND status='accepted'`, id, turnID, status)
	return err
}
func (s *recordStore) metrics(ctx context.Context, id string, data json.RawMessage) error {
	if len(data) > 16384 || !json.Valid(data) {
		return errors.New("invalid render metrics")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO syntelle_streaming.events(session_id,kind,data) VALUES($1,'metrics',$2)`, id, data)
	return err
}
