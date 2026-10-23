package accounts

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

type Mailer interface {
	SendCode(ctx context.Context, email, code, challengeID string) (deliveryID string, err error)
}

type ResendMailer struct {
	Key, From string
	Client    *http.Client
}

func (m *ResendMailer) SendCode(ctx context.Context, email, code, challengeID string) (string, error) {
	// Self-contained HTML has no remote fonts, images, links or template
	// trackers. Provider-level domain tracking must remain disabled in Resend.
	// The text alternative is retained; neither rendered body is persisted.
	html, text, err := renderSignInEmail(code)
	if err != nil {
		return "", errors.New("email could not be prepared")
	}
	body, err := json.Marshal(map[string]any{
		"from": m.From, "to": []string{email}, "subject": "Your Syntelle sign-in code",
		"html": html, "text": text,
	})
	if err != nil {
		return "", errors.New("email could not be prepared")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("email delivery request is invalid")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.Key)
	req.Header.Set("Idempotency-Key", "syntelle-login/"+challengeID)
	res, err := m.Client.Do(req)
	if err != nil {
		return "", errors.New("email delivery outcome is uncertain")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 16<<10))
		return "", errors.New("email provider rejected delivery")
	}
	var result struct {
		ID string `json:"id"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 16<<10)).Decode(&result); err != nil || result.ID == "" || len(result.ID) > 200 {
		return "", errors.New("email delivery response is invalid")
	}
	return result.ID, nil
}

type emailChallenge struct {
	ID, Email, PendingID, Provider, Status string
	Expires                                time.Time
	Created                                time.Time
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || address.Name != "" || len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("enter a valid email address")
	}
	return value, nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (s *Service) emailStart(w http.ResponseWriter, r *http.Request) {
	if len(s.cfg.Secret) < 32 || s.cfg.Mailer == nil {
		httpapi.WriteError(w, 503, "email_unavailable", "Email sign-in needs a configured verified sender.")
		return
	}
	var input struct {
		Email  string `json:"email"`
		FlowID string `json:"flow_id,omitempty"`
	}
	if !decode(w, r, &input) {
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		httpapi.WriteError(w, 400, "invalid_email", "Enter a valid email address.")
		return
	}
	p, err := s.Resolve(w, r, true)
	if err != nil {
		accessError(w, err)
		return
	}
	if p.Kind == "key" {
		accessError(w, ErrForbidden)
		return
	}
	now := s.cfg.Now()
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = randomID()
	}
	if len(key) > 128 {
		httpapi.WriteError(w, 400, "invalid_request", "Invalid request identifier.")
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,187200044))", p.SessionID+":"+key); err != nil {
		accessError(w, err)
		return
	}
	var existing emailChallenge
	err = tx.QueryRow(r.Context(), `SELECT id,email,coalesce(pending_identity_id,''),delivery_status,created_at,expires_at FROM syntelle_accounts.email_challenges WHERE session_id=$1 AND request_key=$2`, p.SessionID, key).Scan(&existing.ID, &existing.Email, &existing.PendingID, &existing.Status, &existing.Created, &existing.Expires)
	if err == nil {
		if existing.Email != email || existing.PendingID != input.FlowID {
			httpapi.WriteError(w, 409, "request_conflict", "This request identifier belongs to different input.")
			return
		}
		if existing.Status != "sent" && existing.Status != "started" {
			httpapi.WriteError(w, 503, "email_delivery_failed", "The code could not be sent. Request a new code after the cooldown.")
			return
		}
		s.challengeResponse(w, existing)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		accessError(w, err)
		return
	}
	if input.FlowID != "" {
		var required string
		err = tx.QueryRow(r.Context(), `SELECT provider,required_email FROM syntelle_accounts.pending_identities WHERE id=$1 AND session_id=$2 AND expires_at>$3 AND consumed_at IS NULL`, input.FlowID, p.SessionID, now).Scan(&existing.Provider, &required)
		if err != nil || (required != "" && required != email) {
			httpapi.WriteError(w, 400, "invalid_flow", "This sign-in needs its original email address. Start sign-in again if it expired.")
			return
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	ipHash := s.verifier("email-ip", ip)
	locks := []string{"email:" + email, "ip:" + ipHash}
	sort.Strings(locks)
	for _, lock := range locks {
		if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,187200042))", lock); err != nil {
			accessError(w, err)
			return
		}
	}
	var emailCount, ipCount int
	var latest *time.Time
	err = tx.QueryRow(r.Context(), `SELECT count(*),max(created_at) FROM syntelle_accounts.email_challenges WHERE email=$1 AND created_at>$2`, email, now.Add(-time.Hour)).Scan(&emailCount, &latest)
	if err == nil {
		err = tx.QueryRow(r.Context(), `SELECT count(*) FROM syntelle_accounts.email_challenges WHERE ip_hash=$1 AND created_at>$2`, ipHash, now.Add(-time.Hour)).Scan(&ipCount)
	}
	if err != nil {
		accessError(w, err)
		return
	}
	if emailCount >= 5 || ipCount >= 20 || (latest != nil && now.Sub(*latest) < 60*time.Second) {
		w.Header().Set("Retry-After", "60")
		httpapi.WriteError(w, 429, "email_rate_limited", "Wait before requesting another code.")
		return
	}
	code, err := randomCode()
	if err != nil {
		accessError(w, err)
		return
	}
	challenge := emailChallenge{ID: randomID(), Email: email, PendingID: input.FlowID, Provider: existing.Provider, Status: "started", Created: now, Expires: now.Add(10 * time.Minute)}
	if _, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.email_challenges SET consumed_at=$3 WHERE session_id=$1 AND email=$2 AND consumed_at IS NULL`, p.SessionID, email, now); err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.email_challenges(id,session_id,email,verifier,request_key,pending_identity_id,ip_hash,created_at,expires_at) VALUES($1,$2,$3,$4,$5,nullif($6,''),$7,$8,$9)`, challenge.ID, p.SessionID, email, s.verifier("email-code", challenge.ID, email, code), key, input.FlowID, ipHash, now, challenge.Expires)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	// The challenge is durable before sending. A provider error is never retried.
	deliveryID, sendErr := s.cfg.Mailer.SendCode(r.Context(), email, code, challenge.ID)
	status := "sent"
	if sendErr != nil {
		status = "uncertain"
	}
	archiveCtx, stop := context.WithTimeout(context.WithoutCancel(r.Context()), 3*time.Second)
	defer stop()
	_, err = s.Pool.Exec(archiveCtx, `UPDATE syntelle_accounts.email_challenges SET delivery_status=$2,delivery_id=$3 WHERE id=$1`, challenge.ID, status, deliveryID)
	if err != nil {
		accessError(w, err)
		return
	}
	if sendErr != nil {
		httpapi.WriteError(w, 502, "email_delivery_failed", "The code could not be sent. Request a new code after the cooldown.")
		return
	}
	s.challengeResponse(w, challenge)
}

func (s *Service) challengeResponse(w http.ResponseWriter, c emailChallenge) {
	purpose := "login"
	if c.PendingID != "" {
		purpose = "link"
	}
	httpapi.WriteJSON(w, 202, map[string]any{"challenge_id": c.ID, "expires_in": max(0, int(c.Expires.Sub(s.cfg.Now()).Seconds())), "resend_after": 60, "purpose": purpose, "provider": c.Provider, "requires_confirmation": c.PendingID != ""})
}

func (s *Service) emailVerify(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
		ConfirmLink bool   `json:"confirm_link,omitempty"`
	}
	if !decode(w, r, &input) {
		return
	}
	p, err := s.Resolve(w, r, false)
	if err != nil {
		accessError(w, err)
		return
	}
	if p.Kind == "key" {
		accessError(w, ErrForbidden)
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		accessError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var email, expected, pending, status string
	var attempts int
	var expires time.Time
	var consumed *time.Time
	err = tx.QueryRow(r.Context(), `SELECT email,verifier,coalesce(pending_identity_id,''),attempts,expires_at,consumed_at,delivery_status FROM syntelle_accounts.email_challenges WHERE id=$1 AND session_id=$2 FOR UPDATE`, input.ChallengeID, p.SessionID).Scan(&email, &expected, &pending, &attempts, &expires, &consumed, &status)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			accessError(w, err)
			return
		}
		invalidCode(w)
		return
	}
	now := s.cfg.Now()
	if consumed != nil || attempts >= 5 || !expires.After(now) || status != "sent" {
		invalidCode(w)
		return
	}
	if !same(expected, s.verifier("email-code", input.ChallengeID, email, input.Code)) {
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.email_challenges SET attempts=attempts+1 WHERE id=$1`, input.ChallengeID)
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			accessError(w, err)
			return
		}
		invalidCode(w)
		return
	}
	if pending != "" && !input.ConfirmLink {
		httpapi.WriteError(w, 400, "confirmation_required", "Confirm connecting this sign-in method before continuing.")
		return
	}
	var provider, subject, name, target string
	if pending != "" {
		var required string
		err = tx.QueryRow(r.Context(), `SELECT provider,subject,display_name,required_email,coalesce(target_account_id,'') FROM syntelle_accounts.pending_identities WHERE id=$1 AND session_id=$2 AND expires_at>$3 AND consumed_at IS NULL FOR UPDATE`, pending, p.SessionID, now).Scan(&provider, &subject, &name, &required, &target)
		if err != nil || (required != "" && required != email) {
			httpapi.WriteError(w, 400, "invalid_flow", "This sign-in has expired. Start again.")
			return
		}
	}
	account, err := s.verifiedAccount(r.Context(), tx, email, name)
	if err != nil {
		accessError(w, err)
		return
	}
	if target != "" && account.ID != target {
		accessError(w, ErrForbidden)
		return
	}
	if pending != "" {
		if err = s.attachIdentity(r.Context(), tx, account.ID, provider, subject); err != nil {
			httpapi.WriteError(w, 409, "identity_conflict", "That sign-in method already belongs to another account.")
			return
		}
		_, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.pending_identities SET consumed_at=$2 WHERE id=$1`, pending, now)
		if err != nil {
			accessError(w, err)
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `UPDATE syntelle_accounts.email_challenges SET consumed_at=$2 WHERE id=$1`, input.ChallengeID, now); err != nil {
		accessError(w, err)
		return
	}
	token, expiry, err := s.accountSession(r.Context(), tx, account.ID, p.SessionID)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		accessError(w, err)
		return
	}
	s.cookie(w, token, expiry)
	if s.OnRevoked != nil {
		s.OnRevoked("session", p.SessionID)
	}
	httpapi.WriteJSON(w, 200, map[string]any{"account": account})
}

func invalidCode(w http.ResponseWriter) {
	httpapi.WriteError(w, 400, "invalid_code", "The code is invalid, expired or already used. Request a new code if needed.")
}

func (s *Service) verifiedAccount(ctx context.Context, tx pgx.Tx, email, name string) (*Account, error) {
	// The caller has just proved inbox control or a verified social identity.
	_, err := tx.Exec(ctx, `INSERT INTO syntelle_accounts.accounts(id,email,display_name) VALUES($1,$2,$3) ON CONFLICT(email) DO NOTHING`, randomID(), email, name)
	if err != nil {
		return nil, err
	}
	a := &Account{}
	err = tx.QueryRow(ctx, `SELECT id,email,display_name FROM syntelle_accounts.accounts WHERE email=$1 AND disabled_at IS NULL`, email).Scan(&a.ID, &a.Email, &a.Name)
	return a, err
}

func (s *Service) attachIdentity(ctx context.Context, tx pgx.Tx, accountID, provider, subject string) error {
	_, err := tx.Exec(ctx, `INSERT INTO syntelle_accounts.identities(provider,subject,account_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, provider, subject, accountID)
	if err != nil {
		return err
	}
	var owner string
	if err = tx.QueryRow(ctx, `SELECT account_id FROM syntelle_accounts.identities WHERE provider=$1 AND subject=$2`, provider, subject).Scan(&owner); err != nil {
		return err
	}
	if owner != accountID {
		return ErrForbidden
	}
	return nil
}

func (s *Service) accountSession(ctx context.Context, tx pgx.Tx, accountID, previousID string) (string, time.Time, error) {
	token, id := randomID(), randomID()
	expires := s.cfg.Now().Add(7 * 24 * time.Hour)
	_, err := tx.Exec(ctx, `INSERT INTO syntelle_accounts.sessions(id,token_hash,account_id,expires_at) VALUES($1,$2,$3,$4)`, id, digest(token), accountID, expires)
	if err == nil && previousID != "" {
		_, err = tx.Exec(ctx, `UPDATE syntelle_accounts.sessions SET revoked_at=$2 WHERE id=$1`, previousID, s.cfg.Now())
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO syntelle_accounts.audit(account_id,event,resource_id) VALUES($1,'signed_in',$2)`, accountID, id)
	}
	return token, expires, err
}
