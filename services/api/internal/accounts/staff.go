package accounts

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

func (s *Service) staffRole(r *http.Request, accountID string) (string, error) {
	var role string
	err := s.Pool.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.staff_grants WHERE account_id=$1 AND revoked_at IS NULL`, accountID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrForbidden
	}
	return role, err
}
func clientIPHash(s *Service, r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return s.verifier("email-ip", ip)
}
func (s *Service) challengeStart(w http.ResponseWriter, r *http.Request, p Principal, purpose, resource string) {
	if s.cfg.Mailer == nil || len(s.cfg.Secret) < 32 {
		httpapi.WriteError(w, 503, "email_unavailable", "Email verification is unavailable.")
		return
	}
	if purpose == "staff_entry" {
		if _, err := s.staffRole(r, p.AccountID); err != nil {
			accessError(w, err)
			return
		}
	}
	var email string
	err := s.Pool.QueryRow(r.Context(), `SELECT email FROM syntelle_accounts.accounts WHERE id=$1 AND disabled_at IS NULL`, p.AccountID).Scan(&email)
	if err != nil {
		accessError(w, ErrForbidden)
		return
	}
	now := s.cfg.Now()
	ipHash := clientIPHash(s, r)
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	// Serialize limit checks per address and peer, like the existing sign-in flow.
	for _, key := range []string{"email:" + email, "ip:" + ipHash} {
		if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,187200042))`, key); err != nil {
			accessError(w, err)
			return
		}
	}
	var staffCount, loginCount, staffIPCount, loginIPCount int
	var staffLatest, loginLatest *time.Time
	err = tx.QueryRow(r.Context(), `SELECT count(*),max(created_at) FROM syntelle_accounts.staff_challenges WHERE account_id=$1 AND created_at>$2`, p.AccountID, now.Add(-time.Hour)).Scan(&staffCount, &staffLatest)
	if err == nil {
		err = tx.QueryRow(r.Context(), `SELECT count(*),max(created_at) FROM syntelle_accounts.email_challenges WHERE email=$1 AND created_at>$2`, email, now.Add(-time.Hour)).Scan(&loginCount, &loginLatest)
	}
	if err == nil {
		err = tx.QueryRow(r.Context(), `SELECT count(*) FROM syntelle_accounts.staff_challenges WHERE request_ip_hash=$1 AND created_at>$2`, ipHash, now.Add(-time.Hour)).Scan(&staffIPCount)
	}
	if err == nil {
		err = tx.QueryRow(r.Context(), `SELECT count(*) FROM syntelle_accounts.email_challenges WHERE ip_hash=$1 AND created_at>$2`, ipHash, now.Add(-time.Hour)).Scan(&loginIPCount)
	}
	if err != nil {
		accessError(w, err)
		return
	}
	if staffCount+loginCount >= 5 || staffIPCount+loginIPCount >= 20 || (staffLatest != nil && now.Sub(*staffLatest) < time.Minute) || (loginLatest != nil && now.Sub(*loginLatest) < time.Minute) {
		httpapi.WriteError(w, 429, "email_rate_limited", "Wait before requesting another code.")
		return
	}
	if purpose == "staff_entry" {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_access SET revoked_at=$2 WHERE session_id=$1 AND revoked_at IS NULL`, p.SessionID, now)
		if err != nil {
			accessError(w, err)
			return
		}
	}
	id, code := randomID(), ""
	code, err = randomCode()
	if err != nil {
		accessError(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.staff_challenges(id,account_id,session_id,purpose,resource_id,verifier,created_at,expires_at,request_ip_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, p.AccountID, p.SessionID, purpose, resource, s.verifier("privileged-code", id, email, code), now, now.Add(10*time.Minute), ipHash)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	delivery := error(nil)
	// Mailer fixtures use SendCode; production ResendMailer uses the purpose-specific variant.
	if special, ok := s.cfg.Mailer.(PurposeMailer); ok {
		_, delivery = special.SendPurposeCode(r.Context(), email, code, id, purpose)
	} else {
		_, delivery = s.cfg.Mailer.SendCode(r.Context(), email, code, id)
	}
	status := "sent"
	if delivery != nil {
		status = "failed"
	}
	_, updateErr := s.Pool.Exec(r.Context(), `UPDATE syntelle_accounts.staff_challenges SET delivery_status=$2 WHERE id=$1`, id, status)
	if delivery != nil || updateErr != nil {
		httpapi.WriteError(w, 503, "email_unavailable", "The verification code could not be sent.")
		return
	}
	httpapi.WriteJSON(w, 202, map[string]any{"challenge_id": id, "expires_in": 600, "resend_after": 60})
}
func (s *Service) challengeVerify(w http.ResponseWriter, r *http.Request, p Principal, purpose, resource string) {
	var input struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}
	if !decode(w, r, &input) {
		return
	}
	s.verifyChallenge(w, r, p, purpose, resource, input.ChallengeID, input.Code)
}
func (s *Service) verifyChallenge(w http.ResponseWriter, r *http.Request, p Principal, purpose, resource, challengeID, code string) {
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var email, expected, status string
	var expires time.Time
	var consumed *time.Time
	var attempts int
	err = tx.QueryRow(r.Context(), `SELECT a.email,c.verifier,c.delivery_status,c.expires_at,c.consumed_at,c.attempts FROM syntelle_accounts.staff_challenges c JOIN syntelle_accounts.accounts a ON a.id=c.account_id WHERE c.id=$1 AND c.account_id=$2 AND c.session_id=$3 AND c.purpose=$4 AND c.resource_id=$5 FOR UPDATE`, challengeID, p.AccountID, p.SessionID, purpose, resource).Scan(&email, &expected, &status, &expires, &consumed, &attempts)
	if err != nil || status != "sent" || consumed != nil || attempts >= 5 || !expires.After(s.cfg.Now()) {
		accessError(w, ErrForbidden)
		return
	}
	if len(code) != 6 || !same(expected, s.verifier("privileged-code", challengeID, email, code)) {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_challenges SET attempts=attempts+1 WHERE id=$1`, challengeID)
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		accessError(w, ErrForbidden)
		return
	}
	now := s.cfg.Now()
	var token string
	if purpose == "staff_entry" {
		var role string
		err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.staff_grants WHERE account_id=$1 AND revoked_at IS NULL`, p.AccountID).Scan(&role)
		if err != nil {
			accessError(w, ErrForbidden)
			return
		}
		token = randomID()
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.staff_access(session_id,account_id,token_hash,verified_at,last_used_at,expires_at) VALUES($1,$2,$3,$4,$4,$5) ON CONFLICT(session_id) DO UPDATE SET account_id=$2,token_hash=$3,verified_at=$4,last_used_at=$4,expires_at=$5,revoked_at=NULL`, p.SessionID, p.AccountID, digest(token), now, now.Add(8*time.Hour))
		if err != nil {
			accessError(w, err)
			return
		}
	}
	_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_challenges SET consumed_at=$2,verified_at=$2 WHERE id=$1`, challengeID, now)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	if purpose == "staff_entry" {
		httpapi.WriteJSON(w, 200, map[string]string{"access_token": token})
		return
	}
	httpapi.WriteJSON(w, 200, map[string]string{"proof_id": challengeID})
}
func (s *Service) staffAccess(r *http.Request, p Principal) (string, error) {
	token := r.Header.Get("X-Syntelle-Staff-Access")
	if len(token) != 43 {
		return "", ErrForbidden
	}
	var role, expected string
	var last, expires time.Time
	err := s.Pool.QueryRow(r.Context(), `SELECT g.role,a.token_hash,a.last_used_at,a.expires_at FROM syntelle_accounts.staff_access a JOIN syntelle_accounts.staff_grants g ON g.account_id=a.account_id WHERE a.session_id=$1 AND a.account_id=$2 AND a.revoked_at IS NULL AND g.revoked_at IS NULL`, p.SessionID, p.AccountID).Scan(&role, &expected, &last, &expires)
	if err != nil || !same(expected, digest(token)) || !expires.After(s.cfg.Now()) || s.cfg.Now().Sub(last) > 30*time.Minute {
		return "", ErrForbidden
	}
	_, err = s.Pool.Exec(r.Context(), `UPDATE syntelle_accounts.staff_access SET last_used_at=$2 WHERE session_id=$1 AND token_hash=$3`, p.SessionID, s.cfg.Now(), expected)
	return role, err
}
func (s *Service) admin(w http.ResponseWriter, r *http.Request) {
	p, ok := s.workspaceAccount(w, r)
	if !ok {
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin"), "/")
	if rest == "eligibility" && r.Method == http.MethodGet {
		role, err := s.staffRole(r, p.AccountID)
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]string{"role": role})
		return
	}
	if rest == "challenge/start" && r.Method == http.MethodPost {
		s.challengeStart(w, r, p, "staff_entry", "")
		return
	}
	if rest == "challenge/verify" && r.Method == http.MethodPost {
		s.challengeVerify(w, r, p, "staff_entry", "")
		return
	}
	role, err := s.staffAccess(r, p)
	if err != nil {
		accessError(w, err)
		return
	}
	switch {
	case rest == "" && r.Method == http.MethodGet:
		httpapi.WriteJSON(w, 200, map[string]string{"role": role})
	case rest == "exit" && r.Method == http.MethodPost:
		_, err = s.Pool.Exec(r.Context(), `UPDATE syntelle_accounts.staff_access SET revoked_at=$2 WHERE session_id=$1`, p.SessionID, s.cfg.Now())
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
	case rest == "customers" && r.Method == http.MethodGet:
		s.adminCustomers(w, r, role)
	case rest == "plans" && r.Method == http.MethodGet:
		httpapi.WriteJSON(w, 200, map[string]string{"status": "unavailable"})
	case rest == "usage-costs" && r.Method == http.MethodGet:
		httpapi.WriteJSON(w, 200, map[string]string{"status": "unavailable"})
	case rest == "service-health" && r.Method == http.MethodGet:
		httpapi.WriteJSON(w, 200, map[string]string{"api": "available", "other_services": "unverified"})
	case rest == "audit" && r.Method == http.MethodGet:
		s.adminAudit(w, r, role)
	case rest == "staff/grant-email/start" && r.Method == http.MethodPost:
		if role != "administrator" {
			accessError(w, ErrForbidden)
			return
		}
		var input struct {
			AccountID string `json:"account_id"`
		}
		if !decode(w, r, &input) {
			return
		}
		s.challengeStart(w, r, p, "staff_grant", input.AccountID)
	case rest == "staff/grant-email/verify" && r.Method == http.MethodPost:
		if role != "administrator" {
			accessError(w, ErrForbidden)
			return
		}
		var input struct {
			AccountID   string `json:"account_id"`
			ChallengeID string `json:"challenge_id"`
			Code        string `json:"code"`
		}
		if !decode(w, r, &input) {
			return
		}
		s.verifyChallenge(w, r, p, "staff_grant", input.AccountID, input.ChallengeID, input.Code)
	case rest == "staff/grants" && r.Method == http.MethodPost:
		s.adminStaffGrant(w, r, p, role)
	case strings.HasPrefix(rest, "staff/grants/") && r.Method == http.MethodDelete:
		s.adminStaffRevoke(w, r, p, role, strings.TrimPrefix(rest, "staff/grants/"))
	case strings.HasPrefix(rest, "workspaces/") && r.Method == http.MethodDelete:
		s.adminKeyRevoke(w, r, p, role, strings.Split(rest, "/"))
	case strings.HasPrefix(rest, "workspaces/") && r.Method == http.MethodPost:
		s.adminWorkspaceAction(w, r, p, role, strings.Split(rest, "/"))
	default:
		httpapi.WriteError(w, 404, "not_found", "Admin action not found.")
	}
}
func (s *Service) adminCustomers(w http.ResponseWriter, r *http.Request, role string) {
	rows, err := s.Pool.Query(r.Context(), `SELECT id,name,suspended_at IS NOT NULL,created_at FROM syntelle_accounts.workspaces ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		accessError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name string
		var suspended bool
		var created time.Time
		if err = rows.Scan(&id, &name, &suspended, &created); err != nil {
			accessError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "name": name, "suspended": suspended, "created_at": created})
	}
	if err = rows.Err(); err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"items": items})
}
func (s *Service) adminAudit(w http.ResponseWriter, r *http.Request, role string) {
	if role != "administrator" {
		accessError(w, ErrForbidden)
		return
	}
	rows, err := s.Pool.Query(r.Context(), `SELECT workspace_id,actor_account_id,action,target_id,reason,created_at FROM syntelle_accounts.workspace_audit ORDER BY id DESC LIMIT 100`)
	if err != nil {
		accessError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var wid, actor *string
		var action, target, reason string
		var at time.Time
		if err = rows.Scan(&wid, &actor, &action, &target, &reason, &at); err != nil {
			accessError(w, err)
			return
		}
		items = append(items, map[string]any{"workspace_id": wid, "actor_account_id": actor, "action": action, "target_id": target, "reason": reason, "created_at": at})
	}
	if err = rows.Err(); err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"items": items})
}
func (s *Service) adminWorkspaceAction(w http.ResponseWriter, r *http.Request, p Principal, role string, parts []string) {
	if role != "administrator" || len(parts) != 3 || parts[0] != "workspaces" || (parts[2] != "suspend" && parts[2] != "resume") {
		accessError(w, ErrForbidden)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if len([]rune(input.Reason)) < 3 || len([]rune(input.Reason)) > 500 {
		httpapi.WriteError(w, 400, "invalid_reason", "Provide a reason (3 to 500 characters).")
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var prior bool
	err = tx.QueryRow(r.Context(), `SELECT suspended_at IS NOT NULL FROM syntelle_accounts.workspaces WHERE id=$1 FOR UPDATE`, parts[1]).Scan(&prior)
	if err != nil {
		accessError(w, ErrNotFound)
		return
	}
	next := parts[2] == "suspend"
	if next {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.workspaces SET suspended_at=$2 WHERE id=$1`, parts[1], s.cfg.Now())
	} else {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.workspaces SET suspended_at=NULL WHERE id=$1`, parts[1])
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id,reason,before_state,after_state) VALUES($1,$2,$3,$1,$4,jsonb_build_object('suspended',$5::boolean),jsonb_build_object('suspended',$6::boolean))`, parts[1], p.AccountID, "workspace."+parts[2], input.Reason, prior, next)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]bool{"suspended": next})
}

func (s *Service) adminStaffGrant(w http.ResponseWriter, r *http.Request, p Principal, role string) {
	if role != "administrator" {
		accessError(w, ErrForbidden)
		return
	}
	var input struct {
		AccountID string `json:"account_id"`
		ProofID   string `json:"proof_id"`
		Reason    string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.AccountID == "" || len([]rune(input.Reason)) < 3 || len([]rune(input.Reason)) > 500 {
		httpapi.WriteError(w, 400, "invalid_request", "Target and reason are required.")
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var actorRole string
	err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.staff_grants WHERE account_id=$1 AND revoked_at IS NULL FOR UPDATE`, p.AccountID).Scan(&actorRole)
	if err != nil || actorRole != "administrator" {
		accessError(w, ErrForbidden)
		return
	}
	var proof bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.staff_challenges WHERE id=$1 AND account_id=$2 AND session_id=$3 AND purpose='staff_grant' AND resource_id=$4 AND verified_at>$5 AND action_used_at IS NULL)`, input.ProofID, p.AccountID, p.SessionID, input.AccountID, s.cfg.Now().Add(-5*time.Minute)).Scan(&proof)
	if err != nil || !proof {
		accessError(w, ErrForbidden)
		return
	}
	var verified bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.accounts a WHERE a.id=$1 AND a.disabled_at IS NULL)`, input.AccountID).Scan(&verified)
	if err != nil || !verified {
		accessError(w, ErrNotFound)
		return
	}
	changed, err := tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.staff_grants(account_id,role) VALUES($1,'support') ON CONFLICT(account_id) DO UPDATE SET role='support',granted_at=$2,revoked_at=NULL WHERE syntelle_accounts.staff_grants.role='support'`, input.AccountID, s.cfg.Now())
	if err != nil {
		accessError(w, err)
		return
	}
	if changed.RowsAffected() != 1 {
		accessError(w, ErrForbidden)
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_challenges SET action_used_at=$2 WHERE id=$1 AND action_used_at IS NULL`, input.ProofID, s.cfg.Now())
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(actor_account_id,action,target_id,reason,after_state) VALUES($1,'staff.support_granted',$2,$3,jsonb_build_object('role','support'))`, p.AccountID, input.AccountID, input.Reason)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	httpapi.WriteJSON(w, 200, map[string]any{"account_id": input.AccountID, "role": "support"})
}

func (s *Service) adminStaffRevoke(w http.ResponseWriter, r *http.Request, p Principal, role, target string) {
	if role != "administrator" || target == "" || strings.Contains(target, "/") {
		accessError(w, ErrForbidden)
		return
	}
	var input struct {
		ProofID string `json:"proof_id"`
		Reason  string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if len([]rune(input.Reason)) < 3 || len([]rune(input.Reason)) > 500 {
		httpapi.WriteError(w, 400, "invalid_reason", "Provide a reason (3 to 500 characters).")
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var actorRole string
	err = tx.QueryRow(r.Context(), `SELECT role FROM syntelle_accounts.staff_grants WHERE account_id=$1 AND revoked_at IS NULL FOR UPDATE`, p.AccountID).Scan(&actorRole)
	if err != nil || actorRole != "administrator" {
		accessError(w, ErrForbidden)
		return
	}
	var proof bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.staff_challenges WHERE id=$1 AND account_id=$2 AND session_id=$3 AND purpose='staff_grant' AND resource_id=$4 AND verified_at>$5 AND action_used_at IS NULL)`, input.ProofID, p.AccountID, p.SessionID, target, s.cfg.Now().Add(-5*time.Minute)).Scan(&proof)
	if err != nil || !proof {
		accessError(w, ErrForbidden)
		return
	}
	changed, err := tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_grants SET revoked_at=$2 WHERE account_id=$1 AND role='support' AND revoked_at IS NULL`, target, s.cfg.Now())
	if err != nil {
		accessError(w, err)
		return
	}
	if changed.RowsAffected() != 1 {
		accessError(w, ErrNotFound)
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_access SET revoked_at=$2 WHERE account_id=$1 AND revoked_at IS NULL`, target, s.cfg.Now())
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.staff_challenges SET action_used_at=$2 WHERE id=$1`, input.ProofID, s.cfg.Now())
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(actor_account_id,action,target_id,reason) VALUES($1,'staff.support_revoked',$2,$3)`, p.AccountID, target, input.Reason)
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

func (s *Service) adminKeyRevoke(w http.ResponseWriter, r *http.Request, p Principal, role string, parts []string) {
	if role != "administrator" || len(parts) != 4 || parts[0] != "workspaces" || parts[2] != "keys" {
		accessError(w, ErrForbidden)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if len([]rune(input.Reason)) < 3 || len([]rune(input.Reason)) > 500 {
		httpapi.WriteError(w, 400, "invalid_reason", "Provide a reason (3 to 500 characters).")
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var wid string
	err = tx.QueryRow(r.Context(), `SELECT id FROM syntelle_accounts.workspaces WHERE id=$1 FOR UPDATE`, parts[1]).Scan(&wid)
	if err != nil {
		accessError(w, ErrNotFound)
		return
	}
	changed, err := tx.Exec(r.Context(), `UPDATE syntelle_accounts.workspace_keys SET revoked_at=$3 WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`, parts[3], wid, s.cfg.Now())
	if err != nil {
		accessError(w, err)
		return
	}
	if changed.RowsAffected() != 1 {
		accessError(w, ErrNotFound)
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.workspace_audit(workspace_id,actor_account_id,action,target_id,reason) VALUES($1,$2,'key.revoked_by_staff',$3,$4)`, wid, p.AccountID, parts[3], input.Reason)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	if s.OnRevoked != nil {
		s.OnRevoked("workspace_key", parts[3])
	}
	httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
}
