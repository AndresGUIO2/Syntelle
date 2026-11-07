package accounts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

type workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Suspended bool   `json:"suspended"`
}
type member struct {
	AccountID string `json:"account_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
}

func (s *Service) workspaceAccount(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	p, err := s.Resolve(w, r, false)
	if err != nil {
		accessError(w, err)
		return Principal{}, false
	}
	if p.Kind != "account" || p.AccountID == "" {
		accessError(w, ErrForbidden)
		return Principal{}, false
	}
	return p, true
}
func (s *Service) memberRole(r *http.Request, id, account string) (string, bool, error) {
	var role string
	var suspended bool
	err := s.Pool.QueryRow(r.Context(), `SELECT m.role,w.suspended_at IS NOT NULL FROM syntelle_accounts.memberships m JOIN syntelle_accounts.workspaces w ON w.id=m.workspace_id WHERE m.workspace_id=$1 AND m.account_id=$2`, id, account).Scan(&role, &suspended)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, ErrNotFound
	}
	return role, suspended, err
}
func operational(role string) bool {
	return role == "owner" || role == "administrator" || role == "editor"
}
func manager(role string) bool { return role == "owner" || role == "administrator" }
func validRole(role string) bool {
	return role == "administrator" || role == "editor" || role == "billing"
}
func (s *Service) workspace(w http.ResponseWriter, r *http.Request) {
	p, ok := s.workspaceAccount(w, r)
	if !ok {
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/workspaces"), "/")
	if rest == "" {
		if r.Method == http.MethodGet {
			s.workspaceList(w, r, p)
			return
		}
		if r.Method == http.MethodPost {
			s.workspaceCreate(w, r, p)
			return
		}
		httpapi.WriteError(w, 405, "method_not_allowed", "Unsupported workspace action.")
		return
	}
	parts := strings.Split(rest, "/")
	if rest == "invitations/accept" && r.Method == http.MethodPost {
		s.invitationAccept(w, r, p)
		return
	}
	if rest == "ownership/accept" && r.Method == http.MethodPost {
		s.ownershipAccept(w, r, p)
		return
	}
	id := parts[0]
	role, suspended, err := s.memberRole(r, id, p.AccountID)
	if err != nil {
		accessError(w, err)
		return
	}
	if suspended && r.Method != http.MethodGet {
		accessError(w, ErrForbidden)
		return
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.workspaceOverview(w, r, id, role, suspended)
	case len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet:
		s.members(w, r, id, role)
	case len(parts) == 2 && parts[1] == "invitations" && r.Method == http.MethodPost:
		s.invite(w, r, id, p, role)
	case len(parts) == 3 && parts[1] == "invitations" && r.Method == http.MethodDelete:
		s.invitationRevoke(w, r, id, parts[2], p, role)
	case len(parts) == 3 && parts[1] == "members" && (r.Method == http.MethodPatch || r.Method == http.MethodDelete):
		s.memberChange(w, r, id, parts[2], p, role)
	case len(parts) == 2 && parts[1] == "ownership-transfer" && r.Method == http.MethodPost:
		s.ownershipStart(w, r, id, p, role)
	case len(parts) == 3 && parts[1] == "ownership-email" && parts[2] == "start" && r.Method == http.MethodPost:
		if role != "owner" {
			accessError(w, ErrForbidden)
			return
		}
		s.challengeStart(w, r, p, "ownership_transfer", id)
	case len(parts) == 3 && parts[1] == "ownership-email" && parts[2] == "verify" && r.Method == http.MethodPost:
		if role != "owner" {
			accessError(w, ErrForbidden)
			return
		}
		s.challengeVerify(w, r, p, "ownership_transfer", id)
	case len(parts) >= 2 && parts[1] == "configurations":
		s.workspaceConfigurations(w, r, id, parts[2:], p, role)
	case len(parts) >= 2 && parts[1] == "keys":
		s.workspaceKeys(w, r, id, parts[2:], p, role)
	case len(parts) == 2 && parts[1] == "activity" && r.Method == http.MethodGet:
		s.workspaceActivity(w, r, id, role)
	default:
		httpapi.WriteError(w, 404, "not_found", "Workspace action not found.")
	}
}
func (s *Service) workspaceList(w http.ResponseWriter, r *http.Request, p Principal) {
	rows, err := s.Pool.Query(r.Context(), `SELECT w.id,w.name,m.role,w.suspended_at IS NOT NULL FROM syntelle_accounts.memberships m JOIN syntelle_accounts.workspaces w ON w.id=m.workspace_id WHERE m.account_id=$1 ORDER BY w.created_at DESC`, p.AccountID)
	if err != nil {
		accessError(w, err)
		return
	}
	defer rows.Close()
	items := []workspace{}
	for rows.Next() {
		var item workspace
		if err = rows.Scan(&item.ID, &item.Name, &item.Role, &item.Suspended); err != nil {
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
}
func (s *Service) workspaceCreate(w http.ResponseWriter, r *http.Request, p Principal) {
	var input struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 100 {
		httpapi.WriteError(w, 400, "invalid_name", "Use 1 to 100 characters.")
		return
	}
	id := randomID()
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspaces(id,name,created_by) VALUES($1,$2,$3)`, id, input.Name, p.AccountID)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.memberships(workspace_id,account_id,role) VALUES($1,$2,'owner')`, id, p.AccountID)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id,after_state) VALUES($1,$2,'workspace.created',$1,jsonb_build_object('name',$3::text))`, id, p.AccountID, input.Name)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 201, workspace{ID: id, Name: input.Name, Role: "owner"})
}
func (s *Service) workspaceOverview(w http.ResponseWriter, r *http.Request, id, role string, suspended bool) {
	var name string
	err := s.Pool.QueryRow(r.Context(), `SELECT name FROM syntelle_accounts.workspaces WHERE id=$1`, id).Scan(&name)
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"workspace": workspace{ID: id, Name: name, Role: role, Suspended: suspended}, "commercial": map[string]any{"status": "unavailable", "included_minutes": nil, "prepaid_balance": nil, "concurrency": nil, "next_billing_date": nil}})
}
func (s *Service) members(w http.ResponseWriter, r *http.Request, id, role string) {
	if !manager(role) {
		accessError(w, ErrForbidden)
		return
	}
	rows, err := s.Pool.Query(r.Context(), `SELECT a.id,a.email,a.display_name,m.role FROM syntelle_accounts.memberships m JOIN syntelle_accounts.accounts a ON a.id=m.account_id WHERE m.workspace_id=$1 ORDER BY m.created_at`, id)
	if err != nil {
		accessError(w, err)
		return
	}
	defer rows.Close()
	items := []member{}
	for rows.Next() {
		var m member
		if err = rows.Scan(&m.AccountID, &m.Email, &m.Name, &m.Role); err != nil {
			accessError(w, err)
			return
		}
		items = append(items, m)
	}
	if err = rows.Err(); err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"items": items})
}
func (s *Service) invite(w http.ResponseWriter, r *http.Request, id string, p Principal, role string) {
	if !manager(role) {
		accessError(w, ErrForbidden)
		return
	}
	var input struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil || !validRole(input.Role) {
		httpapi.WriteError(w, 400, "invalid_invitation", "Invalid email or role.")
		return
	}
	if role != "owner" && input.Role != "editor" {
		accessError(w, ErrForbidden)
		return
	}
	token := randomID()
	inviteID := randomID()
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var current string
	err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, id, p.AccountID).Scan(&current)
	if err != nil || current != role {
		accessError(w, ErrForbidden)
		return
	}
	var existing bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.memberships m JOIN syntelle_accounts.accounts a ON a.id=m.account_id WHERE m.workspace_id=$1 AND a.email=$2)`, id, email).Scan(&existing)
	if err != nil {
		accessError(w, err)
		return
	}
	if existing {
		httpapi.WriteError(w, 409, "already_member", "This person is already a member.")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_invitations(id,workspace_id,email,role,token_hash,invited_by,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, inviteID, id, email, input.Role, digest(token), p.AccountID, s.cfg.Now().Add(7*24*time.Hour))
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id,after_state) VALUES($1,$2,'member.invited',$3,jsonb_build_object('role',$4::text))`, id, p.AccountID, inviteID, input.Role)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	// Delivery is explicit: the creator shares this one-time invitation link with the intended email owner.
	httpapi.WriteJSON(w, 201, map[string]any{"id": inviteID, "email": email, "role": input.Role, "token": token, "expires_at": s.cfg.Now().Add(7 * 24 * time.Hour)})
}
func (s *Service) invitationAccept(w http.ResponseWriter, r *http.Request, p Principal) {
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Token) != 43 {
		accessError(w, ErrNotFound)
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var id, wid, email, role, inviter string
	var expiry time.Time
	var accepted, revoked *time.Time
	err = tx.QueryRow(r.Context(), `SELECT i.id,i.workspace_id,i.email,i.role,i.invited_by,i.expires_at,i.accepted_at,i.revoked_at FROM syntelle_accounts.workspace_invitations i WHERE i.token_hash=$1 FOR UPDATE`, digest(input.Token)).Scan(&id, &wid, &email, &role, &inviter, &expiry, &accepted, &revoked)
	if err != nil || accepted != nil || revoked != nil || !expiry.After(s.cfg.Now()) {
		accessError(w, ErrNotFound)
		return
	}
	var actorRole string
	var suspended bool
	err = tx.QueryRow(r.Context(), `SELECT m.role,w.suspended_at IS NOT NULL FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 FOR UPDATE OF w`, wid, inviter).Scan(&actorRole, &suspended)
	if err != nil || suspended || (!manager(actorRole)) || (actorRole != "owner" && role != "editor") {
		accessError(w, ErrForbidden)
		return
	}
	var actual string
	err = tx.QueryRow(r.Context(), `SELECT email FROM syntelle_accounts.accounts WHERE id=$1 AND disabled_at IS NULL`, p.AccountID).Scan(&actual)
	if err != nil || actual != email {
		accessError(w, ErrForbidden)
		return
	}
	added, err := tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.memberships(workspace_id,account_id,role) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, wid, p.AccountID, role)
	if err == nil && added.RowsAffected() != 1 {
		httpapi.WriteError(w, 409, "already_member", "This person is already a member.")
		return
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.workspace_invitations SET accepted_at=$2 WHERE id=$1 AND accepted_at IS NULL`, id, s.cfg.Now())
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id,after_state) VALUES($1,$2,'member.joined',$2,jsonb_build_object('role',$3::text))`, wid, p.AccountID, role)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"workspace_id": wid, "role": role})
}
func (s *Service) invitationRevoke(w http.ResponseWriter, r *http.Request, id, inviteID string, p Principal, role string) {
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
	var current, invitedRole string
	err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, id, p.AccountID).Scan(&current)
	if err != nil || !manager(current) {
		accessError(w, ErrForbidden)
		return
	}
	err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.workspace_invitations WHERE id=$1 AND workspace_id=$2 AND accepted_at IS NULL AND revoked_at IS NULL FOR UPDATE`, inviteID, id).Scan(&invitedRole)
	if err != nil {
		accessError(w, ErrNotFound)
		return
	}
	if current != "owner" && invitedRole != "editor" {
		accessError(w, ErrForbidden)
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.workspace_invitations SET revoked_at=$2 WHERE id=$1`, inviteID, s.cfg.Now())
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'invitation.revoked',$3)`, id, p.AccountID, inviteID)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Service) memberChange(w http.ResponseWriter, r *http.Request, id, target string, p Principal, role string) {
	if !manager(role) {
		accessError(w, ErrForbidden)
		return
	}
	newRole := ""
	if r.Method == http.MethodPatch {
		var input struct {
			Role string `json:"role"`
		}
		if !decode(w, r, &input) {
			return
		}
		newRole = input.Role
		if !validRole(newRole) {
			httpapi.WriteError(w, 400, "invalid_role", "Invalid role.")
			return
		}
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var actorRole string
	err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, id, p.AccountID).Scan(&actorRole)
	if err != nil || !manager(actorRole) {
		accessError(w, ErrForbidden)
		return
	}
	var oldRole string
	err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND account_id=$2`, id, target).Scan(&oldRole)
	if err != nil {
		accessError(w, ErrNotFound)
		return
	}
	if oldRole == "owner" || target == p.AccountID || (actorRole != "owner" && (oldRole != "editor" || newRole != "")) {
		accessError(w, ErrForbidden)
		return
	}
	if r.Method == http.MethodDelete {
		_, err = tx.Exec(r.Context(), `DELETE FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND account_id=$2`, id, target)
	} else {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.memberships SET role=$3 WHERE workspace_id=$1 AND account_id=$2`, id, target, newRole)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id,before_state,after_state) VALUES($1,$2,'member.changed',$3,jsonb_build_object('role',$4::text),jsonb_build_object('role',$5::text))`, id, p.AccountID, target, oldRole, newRole)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Service) ownershipStart(w http.ResponseWriter, r *http.Request, id string, p Principal, role string) {
	if role != "owner" {
		accessError(w, ErrForbidden)
		return
	}
	var input struct {
		AccountID string `json:"account_id"`
		ProofID   string `json:"proof_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.AccountID == p.AccountID {
		accessError(w, ErrForbidden)
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var current string
	err = tx.QueryRow(r.Context(), `SELECT m.role FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 AND w.suspended_at IS NULL FOR UPDATE OF w`, id, p.AccountID).Scan(&current)
	if err != nil || current != "owner" {
		accessError(w, ErrForbidden)
		return
	}
	var proofValid bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.staff_challenges WHERE id=$1 AND account_id=$2 AND session_id=$3 AND purpose='ownership_transfer' AND resource_id=$4 AND verified_at>$5 AND action_used_at IS NULL)`, input.ProofID, p.AccountID, p.SessionID, id, s.cfg.Now().Add(-5*time.Minute)).Scan(&proofValid)
	if err != nil || !proofValid {
		accessError(w, ErrForbidden)
		return
	}
	var targetRole string
	err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND account_id=$2`, id, input.AccountID).Scan(&targetRole)
	if err != nil {
		accessError(w, ErrNotFound)
		return
	}
	token := randomID()
	transferID := randomID()
	_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.ownership_transfers(id,workspace_id,from_account_id,to_account_id,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, transferID, id, p.AccountID, input.AccountID, digest(token), s.cfg.Now().Add(7*24*time.Hour))
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_challenges SET action_used_at=$2 WHERE id=$1 AND action_used_at IS NULL`, input.ProofID, s.cfg.Now())
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id) VALUES($1,$2,'ownership.proposed',$3)`, id, p.AccountID, input.AccountID)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 201, map[string]any{"id": transferID, "token": token, "expires_at": s.cfg.Now().Add(7 * 24 * time.Hour)})
}
func (s *Service) ownershipAccept(w http.ResponseWriter, r *http.Request, p Principal) {
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Token) != 43 {
		accessError(w, ErrNotFound)
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var id, wid, from, to string
	var expires time.Time
	var accepted, revoked *time.Time
	err = tx.QueryRow(r.Context(), `SELECT id,workspace_id,from_account_id,to_account_id,expires_at,accepted_at,revoked_at FROM syntelle_accounts.ownership_transfers WHERE token_hash=$1 FOR UPDATE`, digest(input.Token)).Scan(&id, &wid, &from, &to, &expires, &accepted, &revoked)
	if err != nil || accepted != nil || revoked != nil || to != p.AccountID || !expires.After(s.cfg.Now()) {
		accessError(w, ErrNotFound)
		return
	}
	var suspended bool
	err = tx.QueryRow(r.Context(), `SELECT suspended_at IS NOT NULL FROM syntelle_accounts.workspaces WHERE id=$1 FOR UPDATE`, wid).Scan(&suspended)
	if err != nil || suspended {
		accessError(w, ErrForbidden)
		return
	}
	var fromRole, toRole string
	err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND account_id=$2`, wid, from).Scan(&fromRole)
	if err != nil || fromRole != "owner" {
		accessError(w, ErrForbidden)
		return
	}
	err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.memberships WHERE workspace_id=$1 AND account_id=$2`, wid, to).Scan(&toRole)
	if err != nil {
		accessError(w, ErrForbidden)
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.memberships SET role='administrator' WHERE workspace_id=$1 AND account_id=$2`, wid, from)
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.memberships SET role='owner' WHERE workspace_id=$1 AND account_id=$2`, wid, to)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.ownership_transfers SET accepted_at=$2 WHERE id=$1`, id, s.cfg.Now())
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id,before_state,after_state) VALUES($1,$2,'ownership.transferred',$3,jsonb_build_object('owner',$4::text,'recipient_role',$5::text),jsonb_build_object('owner',$3::text,'former_owner_role','administrator'))`, wid, to, to, from, toRole)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"workspace_id": wid, "role": "owner"})
}
func (s *Service) workspaceActivity(w http.ResponseWriter, r *http.Request, id, role string) {
	if !manager(role) {
		accessError(w, ErrForbidden)
		return
	}
	rows, err := s.Pool.Query(r.Context(), `SELECT coalesce(a.email,''),x.action,x.target_id,x.reason,x.before_state,x.after_state,x.created_at FROM syntelle_accounts.workspace_audit x LEFT JOIN syntelle_accounts.accounts a ON a.id=x.actor_account_id WHERE x.workspace_id=$1 ORDER BY x.id DESC LIMIT 100`, id)
	if err != nil {
		accessError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var actor, action, target, reason string
		var before, after []byte
		var at time.Time
		if err = rows.Scan(&actor, &action, &target, &reason, &before, &after, &at); err != nil {
			accessError(w, err)
			return
		}
		items = append(items, map[string]any{"actor_email": actor, "action": action, "target_id": target, "reason": reason, "before": json.RawMessage(before), "after": json.RawMessage(after), "created_at": at})
	}
	if err = rows.Err(); err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"items": items})
}
