package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"syntelle/api/internal/httpapi"
)

type oauthPayload struct {
	Verifier      string `json:"verifier"`
	Nonce         string `json:"nonce"`
	TargetAccount string `json:"target_account,omitempty"`
}

type socialIdentity struct {
	Provider, Subject, Email, Name string
	Verified                       bool
}

func (s *Service) oauthConfig(provider string) (*oauth2.Config, bool) {
	cfg := &oauth2.Config{RedirectURL: s.cfg.Origin + "/api/v1/auth/" + provider + "/callback"}
	switch provider {
	case "google":
		cfg.ClientID, cfg.ClientSecret = s.cfg.GoogleClientID, s.cfg.GoogleClientSecret
		cfg.Scopes = []string{oidc.ScopeOpenID, "email", "profile"}
		cfg.Endpoint = oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}
	case "github":
		cfg.ClientID, cfg.ClientSecret = s.cfg.GitHubClientID, s.cfg.GitHubClientSecret
		cfg.Scopes = []string{"read:user", "user:email"}
		cfg.Endpoint = oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token"}
	default:
		return nil, false
	}
	return cfg, len(s.cfg.Secret) >= 32 && cfg.ClientID != "" && cfg.ClientSecret != ""
}

func (s *Service) oauth(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/auth/"), "/")
	if r.Method != http.MethodGet || len(parts) != 2 || (parts[1] != "start" && parts[1] != "callback") {
		httpapi.WriteError(w, 404, "not_found", "This sign-in action is unavailable.")
		return
	}
	cfg, ready := s.oauthConfig(parts[0])
	if !ready {
		s.oauthError(w, r, "provider_unavailable")
		return
	}
	if parts[1] == "start" {
		s.oauthStart(w, r, parts[0], cfg)
	} else {
		s.oauthCallback(w, r, parts[0], cfg)
	}
}

func (s *Service) oauthStart(w http.ResponseWriter, r *http.Request, provider string, cfg *oauth2.Config) {
	p, err := s.Resolve(w, r, true)
	if err != nil || p.Kind == "key" {
		s.oauthError(w, r, "sign_in_failed")
		return
	}
	state := randomID()
	payload := oauthPayload{Verifier: oauth2.GenerateVerifier(), Nonce: randomID()}
	if r.URL.Query().Get("link") == "1" {
		if p.AccountID == "" {
			s.oauthError(w, r, "sign_in_required")
			return
		}
		payload.TargetAccount = p.AccountID
	}
	b, err := json.Marshal(payload)
	if err == nil {
		b, err = s.encrypt(b)
	}
	if err != nil {
		s.oauthError(w, r, "sign_in_failed")
		return
	}
	returnTo := safeReturn(r.URL.Query().Get("return_to"))
	_, err = s.Pool.Exec(r.Context(), `INSERT INTO syntelle_accounts.oauth_flows(state_hash,session_id,provider,payload,return_to,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, digest(state), p.SessionID, provider, b, returnTo, s.cfg.Now().Add(10*time.Minute))
	if err != nil {
		s.oauthError(w, r, "sign_in_failed")
		return
	}
	options := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(payload.Verifier)}
	if provider == "google" {
		options = append(options, oidc.Nonce(payload.Nonce))
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, cfg.AuthCodeURL(state, options...), http.StatusSeeOther)
}

func (s *Service) oauthCallback(w http.ResponseWriter, r *http.Request, provider string, cfg *oauth2.Config) {
	p, err := s.Resolve(w, r, false)
	if err != nil || p.Kind == "key" || len(r.URL.Query().Get("state")) > 256 {
		s.oauthError(w, r, "invalid_state")
		return
	}
	var encrypted []byte
	var returnTo string
	err = s.Pool.QueryRow(r.Context(), `UPDATE syntelle_accounts.oauth_flows SET consumed_at=$4
WHERE state_hash=$1 AND session_id=$2 AND provider=$3 AND expires_at>$4 AND consumed_at IS NULL RETURNING payload,return_to`, digest(r.URL.Query().Get("state")), p.SessionID, provider, s.cfg.Now()).Scan(&encrypted, &returnTo)
	if err != nil {
		s.oauthError(w, r, "invalid_state")
		return
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" || len(r.URL.Query().Get("code")) > 8192 {
		s.oauthError(w, r, "sign_in_cancelled")
		return
	}
	b, err := s.decrypt(encrypted)
	var payload oauthPayload
	if err != nil || json.Unmarshal(b, &payload) != nil {
		s.oauthError(w, r, "invalid_state")
		return
	}
	ctx, stop := context.WithTimeout(r.Context(), 25*time.Second)
	defer stop()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, s.cfg.HTTPClient)
	token, err := cfg.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(payload.Verifier))
	if err != nil {
		s.oauthError(w, r, "provider_failed")
		return
	}
	identity, err := s.socialIdentity(ctx, provider, cfg.ClientID, payload.Nonce, token)
	if err != nil {
		s.oauthError(w, r, "identity_invalid")
		return
	}
	s.finishSocial(w, r, p, identity, payload.TargetAccount, returnTo)
}

func (s *Service) socialIdentity(ctx context.Context, provider, clientID, nonce string, token *oauth2.Token) (socialIdentity, error) {
	identity := socialIdentity{Provider: provider}
	if provider == "google" {
		raw, ok := token.Extra("id_token").(string)
		if !ok || len(raw) > 32<<10 {
			return identity, errors.New("missing Google identity token")
		}
		google, err := oidc.NewProvider(oidc.ClientContext(ctx, s.cfg.HTTPClient), "https://accounts.google.com")
		if err != nil {
			return identity, errors.New("Google verification unavailable")
		}
		id, err := google.Verifier(&oidc.Config{ClientID: clientID}).Verify(ctx, raw)
		if err != nil || !same(id.Nonce, nonce) {
			return identity, errors.New("invalid Google identity")
		}
		var claims struct {
			Email    string `json:"email"`
			Verified bool   `json:"email_verified"`
			Name     string `json:"name"`
		}
		if err = id.Claims(&claims); err != nil {
			return identity, errors.New("invalid Google claims")
		}
		identity.Subject, identity.Name = id.Subject, claims.Name
		if email, err := normalizeEmail(claims.Email); err == nil {
			identity.Email, identity.Verified = email, claims.Verified
		}
	} else {
		var user struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Login string `json:"login"`
		}
		if err := s.githubJSON(ctx, token.AccessToken, "/user", &user); err != nil || user.ID <= 0 {
			return identity, errors.New("invalid GitHub identity")
		}
		identity.Subject, identity.Name = strconv.FormatInt(user.ID, 10), user.Name
		if identity.Name == "" {
			identity.Name = user.Login
		}
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := s.githubJSON(ctx, token.AccessToken, "/user/emails", &emails); err == nil {
			for _, email := range emails {
				if normalized, err := normalizeEmail(email.Email); err == nil && email.Verified && (identity.Email == "" || email.Primary) {
					identity.Email, identity.Verified = normalized, true
					if email.Primary {
						break
					}
				}
			}
		}
	}
	if identity.Subject == "" || len(identity.Subject) > 256 {
		return identity, errors.New("missing stable identity")
	}
	if len([]rune(identity.Name)) > 100 {
		identity.Name = string([]rune(identity.Name)[:100])
	}
	return identity, nil
}

func (s *Service) githubJSON(ctx context.Context, accessToken, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Syntelle-local")
	res, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return errors.New("GitHub profile unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("GitHub profile rejected")
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(target); err != nil {
		return errors.New("GitHub profile invalid")
	}
	return nil
}

func (s *Service) finishSocial(w http.ResponseWriter, r *http.Request, p Principal, identity socialIdentity, target, returnTo string) {
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.oauthError(w, r, "storage_failed")
		return
	}
	defer tx.Rollback(r.Context())
	var owner string
	err = tx.QueryRow(r.Context(), `SELECT i.account_id FROM syntelle_accounts.identities i JOIN syntelle_accounts.accounts a ON a.id=i.account_id WHERE i.provider=$1 AND i.subject=$2 AND a.disabled_at IS NULL`, identity.Provider, identity.Subject).Scan(&owner)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		s.oauthError(w, r, "storage_failed")
		return
	}
	if owner != "" {
		if target != "" && target != owner {
			s.oauthError(w, r, "identity_conflict")
			return
		}
		s.completeSocialSession(w, r, tx, owner, p.SessionID, returnTo)
		return
	}
	if target != "" {
		var email string
		if err = tx.QueryRow(r.Context(), `SELECT email FROM syntelle_accounts.accounts WHERE id=$1 AND disabled_at IS NULL`, target).Scan(&email); err != nil {
			s.oauthError(w, r, "identity_conflict")
			return
		}
		identity.Email, identity.Verified = email, true
	}
	var existing string
	if identity.Verified && identity.Email != "" {
		err = tx.QueryRow(r.Context(), `SELECT id FROM syntelle_accounts.accounts WHERE email=$1`, identity.Email).Scan(&existing)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			s.oauthError(w, r, "storage_failed")
			return
		}
	}
	if target != "" || existing != "" || !identity.Verified || identity.Email == "" {
		flowID := randomID()
		required := ""
		if identity.Verified {
			required = identity.Email
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.pending_identities(id,session_id,provider,subject,display_name,required_email,target_account_id,expires_at) VALUES($1,$2,$3,$4,$5,$6,nullif($7,''),$8)`, flowID, p.SessionID, identity.Provider, identity.Subject, identity.Name, required, target, s.cfg.Now().Add(10*time.Minute))
		if err == nil {
			err = tx.Commit(r.Context())
		}
		if err != nil {
			s.oauthError(w, r, "storage_failed")
			return
		}
		http.Redirect(w, r, s.cfg.Origin+"/?auth=verify-email&flow_id="+url.QueryEscape(flowID), http.StatusSeeOther)
		return
	}
	// Serialize first signup for an email so a concurrent social signup cannot
	// silently attach another provider based on the email match alone.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,187200043))`, identity.Email); err != nil {
		s.oauthError(w, r, "storage_failed")
		return
	}
	if err = tx.QueryRow(r.Context(), `SELECT id FROM syntelle_accounts.accounts WHERE email=$1`, identity.Email).Scan(&existing); err == nil {
		s.oauthError(w, r, "account_link_required")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		s.oauthError(w, r, "storage_failed")
		return
	}
	account := &Account{ID: randomID(), Email: identity.Email, Name: identity.Name}
	inserted, err := tx.Exec(r.Context(), `INSERT INTO syntelle_accounts.accounts(id,email,display_name) VALUES($1,$2,$3) ON CONFLICT(email) DO NOTHING`, account.ID, account.Email, account.Name)
	if err == nil && inserted.RowsAffected() == 0 {
		// Email-code signup may have committed concurrently. That does not
		// authorize attaching this additional social identity without proof.
		s.oauthError(w, r, "account_link_required")
		return
	}
	if err == nil {
		err = s.attachIdentity(r.Context(), tx, account.ID, identity.Provider, identity.Subject)
	}
	if err != nil {
		s.oauthError(w, r, "storage_failed")
		return
	}
	s.completeSocialSession(w, r, tx, account.ID, p.SessionID, returnTo)
}

func (s *Service) completeSocialSession(w http.ResponseWriter, r *http.Request, tx pgx.Tx, accountID, previousID, returnTo string) {
	token, expiry, err := s.accountSession(r.Context(), tx, accountID, previousID)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		s.oauthError(w, r, "storage_failed")
		return
	}
	s.cookie(w, token, expiry)
	if s.OnRevoked != nil {
		s.OnRevoked("session", previousID)
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, s.cfg.Origin+safeReturn(returnTo), http.StatusSeeOther)
}

func safeReturn(path string) string {
	// Account redirects only return to the workspace, never a caller URL.
	if path == "/" || path == "/?view=stream" {
		return path
	}
	return "/"
}

func (s *Service) oauthError(w http.ResponseWriter, r *http.Request, code string) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, s.cfg.Origin+"/?auth_error="+url.QueryEscape(code), http.StatusSeeOther)
}
