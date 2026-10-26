package conversation

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct{ Pool *pgxpool.Pool }
type Turn struct {
	ID       string `json:"id"`
	Sequence int64  `json:"sequence"`
	Status   string `json:"status"`
	User     string `json:"user"`
	Reply    string `json:"reply"`
	Spoken   string `json:"spoken"`
}
type Operation struct {
	ID            int64
	Status        string
	Result, Audio []byte
}

var ErrOperationInProgress = errors.New("This provider operation is already running.")

func Open(ctx context.Context, dsn string) (*Store, error) {
	p, e := pgxpool.New(ctx, dsn)
	if e != nil {
		return nil, errors.New("Conversation database configuration is invalid.")
	}
	tx, e := p.Begin(ctx)
	if e == nil {
		defer tx.Rollback(ctx)
		_, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(187200029)")
		if e == nil {
			e = applyMigrations(ctx, tx)
		}
		if e == nil {
			e = tx.Commit(ctx)
		}
	}
	if e != nil {
		p.Close()
		return nil, errors.New("Conversation migration failed.")
	}
	return &Store{p}, nil
}

func applyMigrations(ctx context.Context, tx pgx.Tx) error {
	if _, e := tx.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS syntelle_conversation; CREATE TABLE IF NOT EXISTS syntelle_conversation.migrations(version integer PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())`); e != nil {
		return e
	}
	files, e := migrations.ReadDir("migrations")
	if e != nil {
		return e
	}
	for _, file := range files {
		version, e := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		if e != nil {
			return e
		}
		var applied bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_conversation.migrations WHERE version=$1)`, version).Scan(&applied); e != nil {
			return e
		}
		if applied {
			continue
		}
		data, e := migrations.ReadFile("migrations/" + file.Name())
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, string(data)); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO syntelle_conversation.migrations(version) VALUES($1)`, version); e != nil {
			return e
		}
	}
	return nil
}

func (s *Store) Recover(ctx context.Context) error {
	// A previous process may have died after sending a request. Never retry it.
	_, e := s.Pool.Exec(ctx, `UPDATE syntelle_conversation.operations SET status='uncertain',finished_at=now(),diagnostics='{"reason":"process restarted before completion"}' WHERE status='started'; UPDATE syntelle_conversation.turns SET status='interrupted',finished_at=now(),interruption='{"reason":"server restart"}' WHERE status='preparing'`)
	return e
}

func (s *Store) Session(ctx context.Context, id, character, language, instructions string) ([]Turn, error) {
	b, _ := json.Marshal(map[string]string{"text": instructions})
	_, e := s.Pool.Exec(ctx, `INSERT INTO syntelle_conversation.sessions(id,character,language,instructions) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO NOTHING`, id, character, language, b)
	if e != nil {
		return nil, e
	}
	var c, l string
	e = s.Pool.QueryRow(ctx, `SELECT character,language FROM syntelle_conversation.sessions WHERE id=$1`, id).Scan(&c, &l)
	if e != nil || c != character || l != language {
		return nil, errors.New("Conversation belongs to a different character or language. Start a new chat.")
	}
	_, e = s.Pool.Exec(ctx, `UPDATE syntelle_conversation.sessions SET instructions=$2,updated_at=now() WHERE id=$1`, id, b)
	if e != nil {
		return nil, e
	}
	// Initial UI restoration is bounded; all older turns remain in PostgreSQL.
	return s.Turns(ctx, id, 100)
}
func (s *Store) Turns(ctx context.Context, id string, limit int) ([]Turn, error) {
	rows, e := s.Pool.Query(ctx, `SELECT id,sequence,status,coalesce(messages->>'user',''),coalesce(model_output->>'text',''),coalesce(spoken->>'text','') FROM (SELECT * FROM syntelle_conversation.turns WHERE conversation_id=$1 ORDER BY sequence DESC LIMIT CASE WHEN $2::int=0 THEN NULL ELSE $2 END) t ORDER BY sequence`, id, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Turn{}
	for rows.Next() {
		var t Turn
		if e = rows.Scan(&t.ID, &t.Sequence, &t.Status, &t.User, &t.Reply, &t.Spoken); e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Instructions(ctx context.Context, id, text string) error {
	b, _ := json.Marshal(map[string]string{"text": text})
	_, e := s.Pool.Exec(ctx, `UPDATE syntelle_conversation.sessions SET instructions=$2,updated_at=now() WHERE id=$1`, id, b)
	return e
}
func (s *Store) BeginTurn(ctx context.Context, cid, id, text string) (bool, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return false, e
	}
	defer tx.Rollback(ctx)
	var n int64
	e = tx.QueryRow(ctx, `SELECT next_sequence FROM syntelle_conversation.sessions WHERE id=$1 FOR UPDATE`, cid).Scan(&n)
	if e != nil {
		return false, e
	}
	var previousCID, previousText string
	e = tx.QueryRow(ctx, `SELECT conversation_id,messages->>'user' FROM syntelle_conversation.turns WHERE id=$1`, id).Scan(&previousCID, &previousText)
	if e == nil {
		if previousCID != cid || previousText != text {
			return false, errors.New("Turn identifier already belongs to different content.")
		}
		return false, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return false, e
	}
	b, _ := json.Marshal(map[string]string{"user": text})
	_, e = tx.Exec(ctx, `INSERT INTO syntelle_conversation.turns(id,conversation_id,sequence,status,messages) VALUES($1,$2,$3,'preparing',$4)`, id, cid, n, b)
	if e != nil {
		return false, e
	}
	_, e = tx.Exec(ctx, `UPDATE syntelle_conversation.sessions SET next_sequence=next_sequence+1,updated_at=now() WHERE id=$1`, cid)
	if e != nil {
		return false, e
	}
	return true, tx.Commit(ctx)
}
func (s *Store) FinishTurn(ctx context.Context, id, status, output, spoken, reason string) error {
	a, _ := json.Marshal(map[string]string{"text": output})
	b, _ := json.Marshal(map[string]string{"text": spoken})
	c, _ := json.Marshal(map[string]string{"reason": reason})
	_, e := s.Pool.Exec(ctx, `UPDATE syntelle_conversation.turns SET status=CASE WHEN status='interrupted' THEN status ELSE $2 END,model_output=$3,spoken=$4,interruption=CASE WHEN status='interrupted' THEN interruption ELSE $5 END,finished_at=now() WHERE id=$1`, id, status, a, b, c)
	return e
}
func (s *Store) Interrupt(ctx context.Context, cid, id, reason string) error {
	b, _ := json.Marshal(map[string]string{"reason": reason})
	_, e := s.Pool.Exec(ctx, `UPDATE syntelle_conversation.turns SET status='interrupted',interruption=CASE WHEN status='interrupted' THEN interruption ELSE $3 END WHERE id=$1 AND conversation_id=$2`, id, cid, b)
	return e
}

func (s *Store) PlaybackText(ctx context.Context, cid, id string) (string, error) {
	var text string
	err := s.Pool.QueryRow(ctx, `SELECT coalesce((SELECT string_agg(request->>'text',' ' ORDER BY (request->>'index')::int) FROM syntelle_conversation.operations WHERE turn_id=t.id AND kind='speech-delivery' AND status='received'),spoken->>'text','') FROM syntelle_conversation.turns t WHERE conversation_id=$1 AND id=$2`, cid, id).Scan(&text)
	return text, err
}

func (s *Store) RecognitionResult(ctx context.Context, id int64, event RecognitionEvent) error {
	ctx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	result, _ := json.Marshal(event)
	_, e := s.Pool.Exec(ctx, `INSERT INTO syntelle_conversation.recognition_results(operation_id,result_id,result) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, event.ID, result)
	return e
}

func (s *Store) Reserve(ctx context.Context, kind, key, binding, cid, tid string, request any, cache bool) (Operation, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return Operation{}, e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, kind+binding)
	if e != nil {
		return Operation{}, e
	}
	var op Operation
	var old string
	e = tx.QueryRow(ctx, `SELECT id,status,result,audio,binding FROM syntelle_conversation.operations WHERE kind=$1 AND (request_key=$2 OR ($4 AND binding=$3 AND status IN ('started','received'))) ORDER BY (request_key=$2) DESC LIMIT 1`, kind, key, binding, cache).Scan(&op.ID, &op.Status, &op.Result, &op.Audio, &old)
	if e == nil {
		if old != binding {
			return op, errors.New("Idempotency key belongs to different input.")
		}
		if op.Status == "started" {
			return op, ErrOperationInProgress
		}
		if op.Status != "received" {
			return op, errors.New("This provider operation already started or failed; it will not be retried.")
		}
		return op, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return op, e
	}
	b, e := json.Marshal(request)
	if e != nil {
		return op, e
	}
	e = tx.QueryRow(ctx, `INSERT INTO syntelle_conversation.operations(kind,request_key,binding,conversation_id,turn_id,status,request) VALUES($1,$2,$3,nullif($4,''),nullif($5,''),'started',$6) RETURNING id`, kind, key, binding, cid, tid, b).Scan(&op.ID)
	if e != nil {
		return op, e
	}
	op.Status = "started"
	return op, tx.Commit(ctx)
}
func (s *Store) Finish(ctx context.Context, id int64, status string, result, usage, diagnostics any, audio []byte) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	a, _ := json.Marshal(result)
	b, _ := json.Marshal(usage)
	c, _ := json.Marshal(diagnostics)
	_, e := s.Pool.Exec(ctx, `UPDATE syntelle_conversation.operations SET status=$2,result=$3,usage=$4,diagnostics=$5,audio=$6,finished_at=now() WHERE id=$1 AND status='started'`, id, status, a, b, c, audio)
	return e
}
