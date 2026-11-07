package accounts

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"syntelle/api/internal/httpapi"
)

func (s *Service) workspaceConfigurations(w http.ResponseWriter, r *http.Request, wid string, tail []string, p Principal, role string) {
	if !operational(role) {
		accessError(w, ErrForbidden)
		return
	}
	if len(tail) == 0 && r.Method == http.MethodGet {
		rows, err := s.Pool.Query(r.Context(), `SELECT id,name,package_hash,settings,created_at,updated_at FROM syntelle_accounts.workspace_configurations WHERE workspace_id=$1 ORDER BY updated_at DESC LIMIT 100`, wid)
		if err != nil {
			accessError(w, err)
			return
		}
		defer rows.Close()
		items := []Configuration{}
		for rows.Next() {
			c, scanErr := scanConfiguration(rows)
			if scanErr != nil {
				accessError(w, scanErr)
				return
			}
			items = append(items, c)
		}
		if err = rows.Err(); err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"items": items})
		return
	}
	if len(tail) == 1 && tail[0] == "copy" && r.Method == http.MethodPost {
		var input struct {
			ConfigurationID string `json:"configuration_id"`
		}
		if !decode(w, r, &input) {
			return
		}
		tx, err := s.Pool.Begin(r.Context())
		if err != nil {
			accessError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		var current string
		err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, wid, p.AccountID).Scan(&current)
		if err != nil || !operational(current) {
			accessError(w, ErrForbidden)
			return
		}
		id := randomID()
		result, err := tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_configurations(id,workspace_id,name,package_hash,settings) SELECT $1,$2,name,package_hash,settings FROM syntelle_accounts.configurations WHERE id=$3 AND account_id=$4 AND expires_at IS NULL AND settings->>'character'='syntelle'`, id, wid, input.ConfigurationID, p.AccountID)
		if err != nil {
			accessError(w, err)
			return
		}
		if result.RowsAffected() != 1 {
			accessError(w, ErrNotFound)
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'configuration.copied',$3)`, wid, p.AccountID, id)
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 201, map[string]string{"id": id})
		return
	}
	if (len(tail) == 0 && r.Method == http.MethodPost) || (len(tail) == 1 && tail[0] != "copy" && r.Method == http.MethodPatch) {
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
			httpapi.WriteError(w, 400, "invalid_name", "Use 1 to 100 characters.")
			return
		}
		if err := input.Settings.Validate(); err != nil {
			httpapi.WriteError(w, 400, "invalid_settings", err.Error())
			return
		}
		if input.Settings.Character != "syntelle" {
			httpapi.WriteError(w, 400, "personal_character", "A personal character cannot be shared with a workspace.")
			return
		}
		if s.cfg.PackageHash == "" {
			httpapi.WriteError(w, 503, "package_unavailable", "The character package is unavailable.")
			return
		}
		settings, err := json.Marshal(input.Settings)
		if err != nil {
			accessError(w, err)
			return
		}
		tx, err := s.Pool.Begin(r.Context())
		if err != nil {
			accessError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		var current string
		err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, wid, p.AccountID).Scan(&current)
		if err != nil || !operational(current) {
			accessError(w, ErrForbidden)
			return
		}
		id := randomID()
		if len(tail) == 0 {
			_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_configurations(id,workspace_id,name,package_hash,settings) VALUES($1,$2,$3,$4,$5)`, id, wid, input.Name, s.cfg.PackageHash, settings)
		} else {
			id = tail[0]
			changed, changeErr := tx.Exec(r.Context(), `UPDATE syntelle_accounts.workspace_configurations SET name=$3,package_hash=$4,settings=$5,updated_at=$6 WHERE id=$1 AND workspace_id=$2`, id, wid, input.Name, s.cfg.PackageHash, settings, s.cfg.Now())
			err = changeErr
			if err == nil && changed.RowsAffected() != 1 {
				err = ErrNotFound
			}
		}
		if err != nil {
			accessError(w, err)
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'configuration.saved',$3)`, wid, p.AccountID, id)
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]string{"id": id})
		return
	}
	if len(tail) == 1 && r.Method == http.MethodDelete {
		tx, err := s.Pool.Begin(r.Context())
		if err != nil {
			accessError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		var current string
		err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, wid, p.AccountID).Scan(&current)
		if err != nil || !operational(current) {
			accessError(w, ErrForbidden)
			return
		}
		changed, err := tx.Exec(r.Context(), `DELETE FROM syntelle_accounts.workspace_configurations WHERE id=$1 AND workspace_id=$2`, tail[0], wid)
		if err != nil {
			accessError(w, err)
			return
		}
		if changed.RowsAffected() != 1 {
			accessError(w, ErrNotFound)
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'configuration.deleted',$3)`, wid, p.AccountID, tail[0])
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	httpapi.WriteError(w, 404, "not_found", "Configuration action not found.")
}

func (s *Service) workspaceKeys(w http.ResponseWriter, r *http.Request, wid string, tail []string, p Principal, role string) {
	if !manager(role) {
		accessError(w, ErrForbidden)
		return
	}
	if len(tail) == 0 && r.Method == http.MethodGet {
		rows, err := s.Pool.Query(r.Context(), `SELECT id,name,scope,created_at,revoked_at,coalesce(integration_id,'') FROM syntelle_accounts.workspace_keys WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT 100`, wid)
		if err != nil {
			accessError(w, err)
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, name, scope, integrationID string
			var created time.Time
			var revoked *time.Time
			if err = rows.Scan(&id, &name, &scope, &created, &revoked, &integrationID); err != nil {
				accessError(w, err)
				return
			}
			status := "pending_payment_activation"
			if integrationID != "" {
				status = "integration_scoped"
			}
			if revoked != nil {
				status = "revoked"
			}
			items = append(items, map[string]any{"id": id, "name": name, "scope": scope, "created_at": created, "revoked_at": revoked, "integration_id": integrationID, "status": status})
		}
		if err = rows.Err(); err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"items": items})
		return
	}
	if len(tail) == 0 && r.Method == http.MethodPost {
		var input struct {
			Name          string `json:"name"`
			IntegrationID string `json:"integration_id,omitempty"`
		}
		if !decode(w, r, &input) {
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 80 {
			httpapi.WriteError(w, 400, "invalid_name", "Use 1 to 80 characters.")
			return
		}
		tx, err := s.Pool.Begin(r.Context())
		if err != nil {
			accessError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		var current string
		err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, wid, p.AccountID).Scan(&current)
		if err != nil || !manager(current) {
			accessError(w, ErrForbidden)
			return
		}
		id, secret := randomID(), randomID()
		token := "syntelle_wk_" + id + "." + secret
		if input.IntegrationID != "" {
			var valid bool
			err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.integrations WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL)`, input.IntegrationID, wid).Scan(&valid)
			if err != nil || !valid {
				accessError(w, ErrNotFound)
				return
			}
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_keys(id,workspace_id,name,secret_hash,scope,created_by,integration_id) VALUES($1,$2,$3,$4,'streams',$5,nullif($6,''))`, id, wid, input.Name, digest(token), p.AccountID, input.IntegrationID)
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'key.created',$3)`, wid, p.AccountID, id)
		}
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		status := "pending_payment_activation"
		if input.IntegrationID != "" {
			status = "integration_scoped"
		}
		httpapi.WriteJSON(w, 201, map[string]string{"id": id, "secret": token, "integration_id": input.IntegrationID, "status": status})
		return
	}
	if len(tail) == 1 && r.Method == http.MethodDelete {
		tx, err := s.Pool.Begin(r.Context())
		if err != nil {
			accessError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		var current string
		err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, wid, p.AccountID).Scan(&current)
		if err != nil || !manager(current) {
			accessError(w, ErrForbidden)
			return
		}
		changed, err := tx.Exec(r.Context(), `UPDATE syntelle_accounts.workspace_keys SET revoked_at=$3 WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, tail[0], wid, s.cfg.Now())
		if err != nil {
			accessError(w, err)
			return
		}
		if changed.RowsAffected() != 1 {
			accessError(w, ErrNotFound)
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'key.revoked',$3)`, wid, p.AccountID, tail[0])
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		if s.OnRevoked != nil {
			s.OnRevoked("workspace_key", tail[0])
		}
		httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	httpapi.WriteError(w, 404, "not_found", "Key action not found.")
}
