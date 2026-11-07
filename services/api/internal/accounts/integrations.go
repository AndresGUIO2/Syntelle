package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

type Publication struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	Name              string     `json:"name"`
	CharacterID       string     `json:"character_id"`
	PackageHash       string     `json:"package_hash"`
	Settings          Settings   `json:"settings"`
	PortraitAvailable bool       `json:"portrait_available"`
	CreatedAt         time.Time  `json:"created_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}
type WidgetAppearance struct {
	Placement     string `json:"placement"`
	Position      string `json:"position"`
	Theme         string `json:"theme"`
	Language      string `json:"language"`
	LauncherLabel string `json:"launcher_label"`
}
type IntegrationConfig struct {
	DailyLimitSeconds int              `json:"daily_limit_seconds"`
	PublicationID     string           `json:"publication_id"`
	AllowedOrigins    []string         `json:"allowed_origins"`
	Capabilities      []string         `json:"capabilities"`
	Settings          Settings         `json:"settings"`
	Appearance        WidgetAppearance `json:"appearance"`
}
type Integration struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	WidgetID    string `json:"widget_id"`
	Name        string `json:"name"`
	IntegrationConfig
	Revision          int64     `json:"revision"`
	PublishedRevision int64     `json:"published_revision"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// PublishedIntegration contains private configuration and must never be serialized to a visitor.
type PublishedIntegration struct {
	Integration
	Publication Publication
}

func NormalizeWidgetOrigin(value string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || strings.ContainsAny(u.Host, "*\\\r\n") {
		return "", errors.New("Use an exact website origin without a path or wildcard.")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local && !httpapi.Cloud()) {
		return "", errors.New("Website origins must use HTTPS.")
	}
	host := strings.ToLower(u.Host)
	if u.Scheme == "https" && u.Port() == "443" {
		host = strings.TrimSuffix(host, ":443")
	}
	return u.Scheme + "://" + host, nil
}
func (c *IntegrationConfig) normalize() error {
	if c.DailyLimitSeconds == 0 {
		c.DailyLimitSeconds = 3600
	}
	if c.DailyLimitSeconds < 60 || c.DailyLimitSeconds > 86400 {
		return errors.New("Use a daily integration allowance between 60 and 86400 seconds.")
	}
	if len(c.AllowedOrigins) > 20 {
		return errors.New("Use at most 20 website origins.")
	}
	seen := map[string]bool{}
	origins := []string{}
	for _, raw := range c.AllowedOrigins {
		value, err := NormalizeWidgetOrigin(raw)
		if err != nil {
			return err
		}
		if !seen[value] {
			origins = append(origins, value)
			seen[value] = true
		}
	}
	c.AllowedOrigins = origins
	if c.Capabilities == nil {
		c.Capabilities = []string{"message", "speak", "interrupt", "suspend"}
	}
	if len(c.Capabilities) == 0 {
		return errors.New("Choose at least one integration capability.")
	}
	seen = map[string]bool{}
	for _, capability := range c.Capabilities {
		if seen[capability] {
			return errors.New("Duplicate capability.")
		}
		seen[capability] = true
		switch capability {
		case "message", "speak", "interrupt", "suspend":
		default:
			return errors.New("Unsupported integration capability.")
		}
	}
	if c.Appearance.Placement == "" {
		c.Appearance.Placement = "floating"
	}
	if c.Appearance.Placement != "inline" && c.Appearance.Placement != "floating" {
		return errors.New("Unsupported widget placement.")
	}
	if c.Appearance.Position == "" {
		c.Appearance.Position = "bottom-right"
	}
	if c.Appearance.Position != "bottom-right" && c.Appearance.Position != "bottom-left" {
		return errors.New("Unsupported widget position.")
	}
	if c.Appearance.Theme == "" {
		c.Appearance.Theme = "system"
	}
	if c.Appearance.Theme != "system" && c.Appearance.Theme != "dark" && c.Appearance.Theme != "light" {
		return errors.New("Unsupported widget theme.")
	}
	if c.Appearance.Language == "" {
		c.Appearance.Language = c.Settings.Language
	}
	if c.Appearance.Language != "en-US" && c.Appearance.Language != "es-CO" {
		return errors.New("Unsupported widget language.")
	}
	c.Settings.Language = c.Appearance.Language
	if c.Appearance.LauncherLabel == "" {
		c.Appearance.LauncherLabel = "Start conversation"
	}
	if len([]rune(c.Appearance.LauncherLabel)) > 80 {
		return errors.New("Launcher label is too long.")
	}
	return c.Settings.Validate()
}
func (s *Service) lockWorkspace(ctx context.Context, tx pgx.Tx, wid, actor string, manage bool) error {
	var role string
	err := tx.QueryRow(ctx, `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, wid, actor).Scan(&role)
	if err != nil || (manage && !manager(role)) || (!manage && !operational(role)) {
		return ErrForbidden
	}
	return nil
}
func readPublication(row pgx.Row) (Publication, error) {
	var p Publication
	var raw []byte
	err := row.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.CharacterID, &p.PackageHash, &raw, &p.PortraitAvailable, &p.CreatedAt, &p.RevokedAt)
	if err == nil {
		err = json.Unmarshal(raw, &p.Settings)
	}
	return p, err
}

const publicationColumns = `id,workspace_id,name,character_id,package_hash,settings,portrait_bytes IS NOT NULL,created_at,revoked_at`

func (s *Service) workspacePublications(w http.ResponseWriter, r *http.Request, wid string, tail []string, p Principal, role string) {
	if !operational(role) {
		accessError(w, ErrForbidden)
		return
	}
	if len(tail) == 0 && r.Method == http.MethodGet {
		rows, err := s.Pool.Query(r.Context(), `SELECT `+publicationColumns+` FROM syntelle_accounts.publications WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT 100`, wid)
		if err != nil {
			accessError(w, err)
			return
		}
		defer rows.Close()
		items := []Publication{}
		for rows.Next() {
			item, err := readPublication(rows)
			if err != nil {
				accessError(w, err)
				return
			}
			items = append(items, item)
		}
		if err = rows.Err(); err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"items": items})
		return
	}
	if len(tail) == 2 && tail[1] == "portrait" && r.Method == http.MethodGet {
		s.publicationPortrait(w, r, wid, tail[0])
		return
	}
	if len(tail) == 0 && r.Method == http.MethodPost {
		var input struct {
			CreationID string    `json:"creation_id"`
			Name       string    `json:"name"`
			Settings   *Settings `json:"settings,omitempty"`
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
		if err = s.lockWorkspace(r.Context(), tx, wid, p.AccountID, false); err != nil {
			accessError(w, err)
			return
		}
		settings := Defaults()
		if input.Settings != nil {
			settings = *input.Settings
		}
		settings.Character = input.CreationID
		var name, hash, mime string
		var portrait []byte
		if input.CreationID == "syntelle" {
			name = "Syntelle"
			hash = s.cfg.PackageHash
		} else {
			err = tx.QueryRow(r.Context(), `SELECT name,package_hash,source_bytes,mime_type FROM syntelle_accounts.character_creations WHERE id=$1 AND account_id=$2 AND status='ready' AND deleted_at IS NULL FOR UPDATE`, input.CreationID, p.AccountID).Scan(&name, &hash, &portrait, &mime)
			if err != nil {
				accessError(w, ErrNotFound)
				return
			}
		}
		if strings.TrimSpace(input.Name) != "" {
			name = strings.TrimSpace(input.Name)
		}
		if len([]rune(name)) > 100 || hash == "" {
			httpapi.WriteError(w, 400, "invalid_publication", "A ready character and a name of at most 100 characters are required.")
			return
		}
		if err = settings.Validate(); err != nil {
			httpapi.WriteError(w, 400, "invalid_settings", err.Error())
			return
		}
		raw, _ := json.Marshal(settings)
		id := randomID()
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.publications(id,workspace_id,name,character_id,package_hash,settings,portrait_bytes,portrait_mime,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, wid, name, input.CreationID, hash, raw, portrait, mime, p.AccountID)
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'publication.created',$3)`, wid, p.AccountID, id)
		}
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		result, err := readPublication(s.Pool.QueryRow(r.Context(), `SELECT `+publicationColumns+` FROM syntelle_accounts.publications WHERE id=$1`, id))
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 201, result)
		return
	}
	if len(tail) == 1 && r.Method == http.MethodDelete && manager(role) {
		tx, err := s.Pool.Begin(r.Context())
		if err != nil {
			accessError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		if err = s.lockWorkspace(r.Context(), tx, wid, p.AccountID, true); err != nil {
			accessError(w, err)
			return
		}
		changed, err := tx.Exec(r.Context(), `UPDATE syntelle_accounts.publications SET revoked_at=now() WHERE workspace_id=$1 AND id=$2 AND revoked_at IS NULL`, wid, tail[0])
		if err == nil && changed.RowsAffected() != 1 {
			err = ErrNotFound
		}
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'publication.revoked',$3)`, wid, p.AccountID, tail[0])
		}
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		if s.OnRevoked != nil {
			s.OnRevoked("publication", tail[0])
		}
		httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	httpapi.WriteError(w, 405, "method_not_allowed", "Unsupported publication action.")
}
func (s *Service) publicationPortrait(w http.ResponseWriter, r *http.Request, wid, id string) {
	var data []byte
	var mime string
	err := s.Pool.QueryRow(r.Context(), `SELECT portrait_bytes,portrait_mime FROM syntelle_accounts.publications WHERE workspace_id=$1 AND id=$2 AND revoked_at IS NULL AND portrait_bytes IS NOT NULL`, wid, id).Scan(&data, &mime)
	if err != nil {
		accessError(w, ErrNotFound)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	_, _ = w.Write(data)
}

func scanIntegration(row pgx.Row, published bool) (Integration, error) {
	var i Integration
	var raw []byte
	err := row.Scan(&i.ID, &i.WorkspaceID, &i.WidgetID, &i.Name, &raw, &i.Revision, &i.PublishedRevision, &i.Enabled, &i.CreatedAt, &i.UpdatedAt)
	if err == nil {
		err = json.Unmarshal(raw, &i.IntegrationConfig)
	}
	return i, err
}
func integrationColumns(published bool) string {
	field := "draft"
	name := "name"
	if published {
		field = "published"
		name = "coalesce(published_name,name)"
	}
	return `id,workspace_id,widget_id,` + name + `,` + field + `,revision,published_revision,enabled,created_at,updated_at`
}
func (s *Service) workspaceIntegrations(w http.ResponseWriter, r *http.Request, wid string, tail []string, p Principal, role string) {
	if !operational(role) {
		accessError(w, ErrForbidden)
		return
	}
	if len(tail) == 0 && r.Method == http.MethodGet {
		rows, err := s.Pool.Query(r.Context(), `SELECT `+integrationColumns(false)+` FROM syntelle_accounts.integrations WHERE workspace_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC LIMIT 100`, wid)
		if err != nil {
			accessError(w, err)
			return
		}
		defer rows.Close()
		items := []Integration{}
		for rows.Next() {
			i, err := scanIntegration(rows, false)
			if err != nil {
				accessError(w, err)
				return
			}
			items = append(items, i)
		}
		if err = rows.Err(); err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"items": items})
		return
	}
	if !manager(role) {
		accessError(w, ErrForbidden)
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = s.lockWorkspace(r.Context(), tx, wid, p.AccountID, true); err != nil {
		accessError(w, err)
		return
	}
	id := randomID()
	action := "integration.created"
	if (len(tail) == 0 && r.Method == http.MethodPost) || (len(tail) == 1 && r.Method == http.MethodPatch) {
		var input struct {
			Name string `json:"name"`
			IntegrationConfig
			SettingsOverride *Settings `json:"settings,omitempty"`
		}
		if !decode(w, r, &input) {
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		pub, err := readPublication(tx.QueryRow(r.Context(), `SELECT `+publicationColumns+` FROM syntelle_accounts.publications WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, input.PublicationID, wid))
		if err != nil {
			accessError(w, ErrNotFound)
			return
		}
		input.IntegrationConfig.Settings = pub.Settings
		if input.SettingsOverride != nil {
			input.IntegrationConfig.Settings = *input.SettingsOverride
		}
		input.Settings.Character = pub.CharacterID
		if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 100 {
			httpapi.WriteError(w, 400, "invalid_name", "Use 1 to 100 characters.")
			return
		}
		if err = input.IntegrationConfig.normalize(); err != nil {
			httpapi.WriteError(w, 400, "invalid_integration", err.Error())
			return
		}
		raw, _ := json.Marshal(input.IntegrationConfig)
		if len(tail) == 0 {
			_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.integrations(id,workspace_id,widget_id,name,draft,created_by) VALUES($1,$2,$3,$4,$5,$6)`, id, wid, "widget_"+randomID(), input.Name, raw, p.AccountID)
		} else {
			id = tail[0]
			action = "integration.edited"
			var n int64
			changed, e := tx.Exec(r.Context(), `UPDATE syntelle_accounts.integrations SET name=$3,draft=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, id, wid, input.Name, raw)
			err = e
			n = changed.RowsAffected()
			if err == nil && n != 1 {
				err = ErrNotFound
			}
		}
	} else if len(tail) == 2 && tail[1] == "publish" && r.Method == http.MethodPost {
		id = tail[0]
		action = "integration.published"
		i, e := scanIntegration(tx.QueryRow(r.Context(), `SELECT `+integrationColumns(false)+` FROM syntelle_accounts.integrations WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL FOR UPDATE`, id, wid), false)
		if e != nil {
			accessError(w, ErrNotFound)
			return
		}
		var valid bool
		err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.publications WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL)`, i.PublicationID, wid).Scan(&valid)
		if err != nil {
			accessError(w, err)
			return
		}
		if !valid || len(i.AllowedOrigins) == 0 {
			httpapi.WriteError(w, 400, "not_publishable", "Choose an available publication and at least one website origin.")
			return
		}
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.integrations SET published=draft,published_name=name,published_revision=revision,enabled=true,daily_limit_seconds=$2,updated_at=now() WHERE id=$1`, id, i.DailyLimitSeconds)
	} else if len(tail) == 1 && r.Method == http.MethodDelete {
		id = tail[0]
		action = "integration.revoked"
		changed, e := tx.Exec(r.Context(), `UPDATE syntelle_accounts.integrations SET enabled=false,revoked_at=now(),updated_at=now() WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, id, wid)
		err = e
		if err == nil && changed.RowsAffected() != 1 {
			err = ErrNotFound
		}
	} else {
		httpapi.WriteError(w, 405, "method_not_allowed", "Unsupported integration action.")
		return
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,$3,$4)`, wid, p.AccountID, action, id)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	if (action == "integration.revoked" || action == "integration.published") && s.OnRevoked != nil {
		s.OnRevoked("integration", id)
	}
	if action == "integration.revoked" {
		httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	i, err := scanIntegration(s.Pool.QueryRow(r.Context(), `SELECT `+integrationColumns(false)+` FROM syntelle_accounts.integrations WHERE id=$1`, id), false)
	if err != nil {
		accessError(w, err)
		return
	}
	status := 200
	if action == "integration.created" {
		status = 201
	}
	httpapi.WriteJSON(w, status, i)
}

func (s *Service) published(ctx context.Context, column, id string) (PublishedIntegration, error) {
	// column is selected internally, never supplied by an HTTP caller.
	i, err := scanIntegration(s.Pool.QueryRow(ctx, `SELECT `+integrationColumns(true)+` FROM syntelle_accounts.integrations WHERE `+column+`=$1 AND enabled AND revoked_at IS NULL AND published IS NOT NULL AND EXISTS(SELECT 1 FROM syntelle_accounts.workspaces w WHERE w.id=workspace_id AND w.suspended_at IS NULL)`, id), true)
	if err != nil {
		return PublishedIntegration{}, ErrNotFound
	}
	pub, err := readPublication(s.Pool.QueryRow(ctx, `SELECT `+publicationColumns+` FROM syntelle_accounts.publications WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, i.PublicationID, i.WorkspaceID))
	if err != nil {
		return PublishedIntegration{}, ErrNotFound
	}
	i.Settings.Character = pub.CharacterID
	return PublishedIntegration{Integration: i, Publication: pub}, nil
}
func (s *Service) ResolveWidget(ctx context.Context, id string) (PublishedIntegration, error) {
	return s.published(ctx, "widget_id", id)
}
func (s *Service) ResolveIntegration(ctx context.Context, p Principal, id string) (PublishedIntegration, error) {
	if p.WorkspaceID == "" || p.IntegrationID != id || (p.Kind != "workspace_key" && p.Kind != "widget") {
		return PublishedIntegration{}, ErrForbidden
	}
	i, err := s.published(ctx, "id", id)
	if err != nil || i.WorkspaceID != p.WorkspaceID || i.PublishedRevision != p.IntegrationRevision || i.PublicationID != p.PublicationID {
		return PublishedIntegration{}, ErrForbidden
	}
	return i, nil
}
func (s *Service) AuthorizeWorkspaceBilling(w http.ResponseWriter, r *http.Request, wid string) (actorID, role string, suspended bool, ok bool) {
	if r.Header.Get("Authorization") != "" {
		accessError(w, ErrForbidden)
		return
	}
	p, valid := s.workspaceAccount(w, r)
	if !valid {
		return
	}
	role, suspended, err := s.memberRole(r, wid, p.AccountID)
	if err != nil {
		accessError(w, err)
		return
	}
	return p.AccountID, role, suspended, true
}
