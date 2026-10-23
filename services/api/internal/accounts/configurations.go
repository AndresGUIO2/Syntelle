package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

func scanConfiguration(row pgx.Row) (Configuration, error) {
	var c Configuration
	var data []byte
	err := row.Scan(&c.ID, &c.Name, &c.PackageHash, &data, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(data, &c.Settings)
	}
	return c, err
}

// Configuration is usable by an account or one of its stream-scoped keys.
func (s *Service) Configuration(ctx context.Context, p Principal, id string) (Configuration, error) {
	return scanConfiguration(s.Pool.QueryRow(ctx, `SELECT id,name,package_hash,settings,created_at,updated_at FROM syntelle_accounts.configurations WHERE id=$1 AND ((account_id=$2 AND $2<>'') OR (guest_session_id=$3 AND $2='')) AND (expires_at IS NULL OR expires_at>$4)`, id, p.AccountID, p.SessionID, s.cfg.Now()))
}

func (s *Service) configurations(w http.ResponseWriter, r *http.Request) {
	p, err := s.Resolve(w, r, true)
	if err != nil {
		accessError(w, err)
		return
	}
	if p.Kind == "key" {
		accessError(w, ErrForbidden)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/configurations")
	if id != "" {
		id = strings.TrimPrefix(id, "/")
	}
	switch {
	case id == "" && r.Method == http.MethodGet:
		rows, err := s.Pool.Query(r.Context(), `SELECT id,name,package_hash,settings,created_at,updated_at FROM syntelle_accounts.configurations WHERE ((account_id=$1 AND $1<>'') OR (guest_session_id=$2 AND $1='')) AND (expires_at IS NULL OR expires_at>$3) ORDER BY updated_at DESC`, p.AccountID, p.SessionID, s.cfg.Now())
		if err != nil {
			accessError(w, err)
			return
		}
		defer rows.Close()
		items := []Configuration{}
		for rows.Next() {
			c, err := scanConfiguration(rows)
			if err != nil {
				accessError(w, err)
				return
			}
			items = append(items, c)
		}
		if err = rows.Err(); err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"items": items})
	case id != "" && r.Method == http.MethodGet:
		c, err := s.Configuration(r.Context(), p, id)
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, c)
	case (id == "" && r.Method == http.MethodPost) || (id != "" && r.Method == http.MethodPatch):
		var input struct {
			Name     string   `json:"name"`
			Settings Settings `json:"settings"`
		}
		input.Settings = Defaults()
		if !decode(w, r, &input) {
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 100 {
			httpapi.WriteError(w, 400, "invalid_name", "Use a configuration name with 1 to 100 characters.")
			return
		}
		if err = input.Settings.Validate(); err != nil {
			httpapi.WriteError(w, 400, "invalid_settings", err.Error())
			return
		}
		if s.cfg.PackageHash == "" {
			httpapi.WriteError(w, 503, "package_unavailable", "The Syntelle package has not been registered.")
			return
		}
		b, err := json.Marshal(input.Settings)
		if err != nil {
			accessError(w, err)
			return
		}
		if id == "" {
			id = randomID()
			var accountID, guestID *string
			var expiry *time.Time
			if p.AccountID != "" {
				accountID = &p.AccountID
			} else {
				guestID = &p.SessionID
				e := s.cfg.Now().Add(24 * time.Hour)
				expiry = &e
			}
			_, err = s.Pool.Exec(r.Context(), `INSERT INTO syntelle_accounts.configurations(id,account_id,guest_session_id,name,package_hash,settings,created_at,updated_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7,$8)`, id, accountID, guestID, input.Name, s.cfg.PackageHash, b, s.cfg.Now(), expiry)
		} else {
			var result interface{ RowsAffected() int64 }
			result, err = s.Pool.Exec(r.Context(), `UPDATE syntelle_accounts.configurations SET name=$4,package_hash=$5,settings=$6,updated_at=$7 WHERE id=$1 AND ((account_id=$2 AND $2<>'') OR (guest_session_id=$3 AND $2='')) AND (expires_at IS NULL OR expires_at>$7)`, id, p.AccountID, p.SessionID, input.Name, s.cfg.PackageHash, b, s.cfg.Now())
			if err == nil && result.RowsAffected() == 0 {
				err = ErrNotFound
			}
		}
		if err != nil {
			accessError(w, err)
			return
		}
		c, err := s.Configuration(r.Context(), p, id)
		if err != nil {
			accessError(w, err)
			return
		}
		status := 200
		if r.Method == http.MethodPost {
			status = 201
		}
		httpapi.WriteJSON(w, status, c)
	case id != "" && r.Method == http.MethodDelete:
		result, err := s.Pool.Exec(r.Context(), `DELETE FROM syntelle_accounts.configurations WHERE id=$1 AND ((account_id=$2 AND $2<>'') OR (guest_session_id=$3 AND $2=''))`, id, p.AccountID, p.SessionID)
		if err != nil {
			accessError(w, err)
			return
		}
		if result.RowsAffected() == 0 {
			accessError(w, ErrNotFound)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
	default:
		httpapi.WriteError(w, 405, "method_not_allowed", "This configuration action is unavailable.")
	}
}
