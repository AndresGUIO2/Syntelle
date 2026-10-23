package accounts

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

type Key struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scope      string     `json:"scope"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

func (s *Service) resolveKey(ctx context.Context, authorization string) (Principal, error) {
	value, ok := strings.CutPrefix(authorization, "Bearer syntelle_sk_")
	parts := strings.Split(value, ".")
	if !ok || len(parts) != 2 || len(parts[0]) != 43 || len(parts[1]) != 43 {
		return Principal{}, ErrUnauthorized
	}
	var expected, account, scope string
	err := s.Pool.QueryRow(ctx, `SELECT k.secret_hash,k.account_id,k.scope FROM syntelle_accounts.api_keys k JOIN syntelle_accounts.accounts a ON a.id=k.account_id WHERE k.id=$1 AND k.revoked_at IS NULL AND a.disabled_at IS NULL`, parts[0]).Scan(&expected, &account, &scope)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, err
	}
	if !same(expected, digest(value)) || scope != "streams" {
		return Principal{}, ErrUnauthorized
	}
	if _, err = s.Pool.Exec(ctx, `UPDATE syntelle_accounts.api_keys SET last_used_at=$2::timestamptz WHERE id=$1 AND (last_used_at IS NULL OR last_used_at<$2::timestamptz-interval '1 minute')`, parts[0], s.cfg.Now()); err != nil {
		return Principal{}, err
	}
	return Principal{ID: "account:" + account, Kind: "key", AccountID: account, KeyID: parts[0]}, nil
}

func (s *Service) browserAccount(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	p, err := s.Resolve(w, r, false)
	if err != nil {
		accessError(w, err)
		return Principal{}, false
	}
	if p.Kind != "account" || p.AccountID == "" {
		accessError(w, ErrUnauthorized)
		return Principal{}, false
	}
	return p, true
}

func (s *Service) keys(w http.ResponseWriter, r *http.Request) {
	p, ok := s.browserAccount(w, r)
	if !ok {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/keys")
	if id != "" {
		id = strings.TrimPrefix(id, "/")
	}
	switch {
	case id == "" && r.Method == http.MethodGet:
		rows, err := s.Pool.Query(r.Context(), `SELECT id,name,scope,created_at,last_used_at,revoked_at FROM syntelle_accounts.api_keys WHERE account_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC`, p.AccountID)
		if err != nil {
			accessError(w, err)
			return
		}
		defer rows.Close()
		items := []Key{}
		for rows.Next() {
			var k Key
			if err = rows.Scan(&k.ID, &k.Name, &k.Scope, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
				accessError(w, err)
				return
			}
			items = append(items, k)
		}
		if err = rows.Err(); err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"items": items})
	case id == "" && r.Method == http.MethodPost:
		var input struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &input) {
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 80 {
			httpapi.WriteError(w, 400, "invalid_name", "Use a key name with 1 to 80 characters.")
			return
		}
		key := Key{ID: randomID(), Name: input.Name, Scope: "streams", CreatedAt: s.cfg.Now()}
		secret := key.ID + "." + randomID()
		_, err := s.Pool.Exec(r.Context(), `INSERT INTO syntelle_accounts.api_keys(id,account_id,name,secret_hash,scope,created_at) VALUES($1,$2,$3,$4,$5,$6)`, key.ID, p.AccountID, key.Name, digest(secret), key.Scope, key.CreatedAt)
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 201, map[string]any{"id": key.ID, "name": key.Name, "scope": key.Scope, "created_at": key.CreatedAt, "secret": "syntelle_sk_" + secret})
	case id != "" && r.Method == http.MethodDelete:
		result, err := s.Pool.Exec(r.Context(), `UPDATE syntelle_accounts.api_keys SET revoked_at=coalesce(revoked_at,$3) WHERE id=$1 AND account_id=$2`, id, p.AccountID, s.cfg.Now())
		if err != nil {
			accessError(w, err)
			return
		}
		if result.RowsAffected() == 0 {
			accessError(w, ErrNotFound)
			return
		}
		if s.OnRevoked != nil {
			s.OnRevoked("key", id)
		}
		httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
	default:
		httpapi.WriteError(w, 405, "method_not_allowed", "This key action is unavailable.")
	}
}
