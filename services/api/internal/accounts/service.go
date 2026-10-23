package accounts

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"syntelle/api/internal/httpapi"
)

//go:embed migrations/*.sql
var migrations embed.FS

const sessionCookie = "syntelle_session"

type Service struct {
	Pool      *pgxpool.Pool
	cfg       Config
	OnRevoked func(kind, id string)
}

func Open(ctx context.Context, pool *pgxpool.Pool, cfg Config) (*Service, error) {
	if pool == nil {
		return nil, errors.New("account database is unavailable")
	}
	u, err := url.Parse(cfg.Origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("account origin must be an HTTP origin without a path")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Mailer == nil && cfg.ResendAPIKey != "" && cfg.ResendFrom != "" {
		cfg.Mailer = &ResendMailer{Key: cfg.ResendAPIKey, From: cfg.ResendFrom, Client: cfg.HTTPClient}
	}
	s := &Service{Pool: pool, cfg: cfg}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin account migration: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(187200041)"); err != nil {
		return nil, fmt.Errorf("lock account migrations: %w", err)
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read account migrations: %w", err)
	}
	for _, file := range files {
		version, err := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("parse account migration: %w", err)
		}
		var applied bool
		if _, err = tx.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS syntelle_accounts; CREATE TABLE IF NOT EXISTS syntelle_accounts.migrations(version integer PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
			return nil, fmt.Errorf("initialize account migrations: %w", err)
		}
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM syntelle_accounts.migrations WHERE version=$1)", version).Scan(&applied); err != nil {
			return nil, fmt.Errorf("inspect account migration: %w", err)
		}
		if applied {
			continue
		}
		b, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			return nil, fmt.Errorf("load account migration: %w", err)
		}
		if _, err = tx.Exec(ctx, string(b)); err != nil {
			return nil, fmt.Errorf("apply account migration: %w", err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO syntelle_accounts.migrations(version) VALUES($1)", version); err != nil {
			return nil, fmt.Errorf("record account migration: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, "DELETE FROM syntelle_accounts.configurations WHERE expires_at <= $1", cfg.Now()); err != nil {
		return nil, fmt.Errorf("expire guest configurations: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit account migrations: %w", err)
	}
	return s, nil
}

// AllowedRequest keeps the account MVP local and protects cookie writes from
// cross-site requests. Bearer integrations may omit Origin; browser writes may not.
func (s *Service) AllowedRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return false
	}
	origin := r.Header.Get("Origin")
	allowed := origin == s.cfg.Origin || origin == "http://127.0.0.1:5173" || origin == "http://localhost:5173"
	if origin != "" && !allowed {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			return true
		}
		return allowed && r.Header.Get("X-Syntelle-Request") == "1"
	}
	return true
}

func (s *Service) Resolve(w http.ResponseWriter, r *http.Request, allowGuest bool) (Principal, error) {
	if !s.AllowedRequest(r) {
		return Principal{}, ErrForbidden
	}
	if authorization := r.Header.Get("Authorization"); authorization != "" {
		return s.resolveKey(r.Context(), authorization)
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil && len(cookie.Value) <= 256 {
		var p Principal
		var accountID *string
		err = s.Pool.QueryRow(r.Context(), `SELECT s.id,s.account_id FROM syntelle_accounts.sessions s
LEFT JOIN syntelle_accounts.accounts a ON a.id=s.account_id
WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>$2 AND (s.account_id IS NULL OR a.disabled_at IS NULL)`, digest(cookie.Value), s.cfg.Now()).Scan(&p.SessionID, &accountID)
		if err == nil {
			if accountID == nil {
				p.Kind, p.ID = "guest", "guest:"+p.SessionID
			} else {
				p.Kind, p.AccountID = "account", *accountID
				p.ID = "account:" + p.AccountID
			}
			return p, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Principal{}, fmt.Errorf("resolve browser session: %w", err)
		}
	}
	if !allowGuest {
		return Principal{}, ErrUnauthorized
	}
	id, token := randomID(), randomID()
	expires := s.cfg.Now().Add(24 * time.Hour)
	_, err := s.Pool.Exec(r.Context(), "INSERT INTO syntelle_accounts.sessions(id,token_hash,expires_at) VALUES($1,$2,$3)", id, digest(token), expires)
	if err != nil {
		return Principal{}, fmt.Errorf("create guest session: %w", err)
	}
	s.cookie(w, token, expires)
	return Principal{ID: "guest:" + id, Kind: "guest", SessionID: id}, nil
}

// ValidatePrincipal rechecks a previously authenticated upgraded connection.
// Possession of a connection ticket does not extend its session or key lifetime.
func (s *Service) ValidatePrincipal(ctx context.Context, p Principal) error {
	var active bool
	var err error
	switch p.Kind {
	case "key":
		if p.AccountID == "" || p.KeyID == "" || p.ID != "account:"+p.AccountID {
			return ErrUnauthorized
		}
		err = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.api_keys k JOIN syntelle_accounts.accounts a ON a.id=k.account_id WHERE k.id=$1 AND k.account_id=$2 AND k.revoked_at IS NULL AND a.disabled_at IS NULL AND k.scope='streams')`, p.KeyID, p.AccountID).Scan(&active)
	case "account", "guest":
		if (p.Kind == "guest" && (p.AccountID != "" || p.ID != "guest:"+p.SessionID)) || (p.Kind == "account" && (p.AccountID == "" || p.ID != "account:"+p.AccountID)) {
			return ErrUnauthorized
		}
		err = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.sessions s LEFT JOIN syntelle_accounts.accounts a ON a.id=s.account_id WHERE s.id=$1 AND coalesce(s.account_id,'')=$2 AND s.revoked_at IS NULL AND s.expires_at>$3 AND (s.account_id IS NULL OR a.disabled_at IS NULL))`, p.SessionID, p.AccountID, s.cfg.Now()).Scan(&active)
	default:
		return ErrUnauthorized
	}
	if err != nil {
		return fmt.Errorf("validate product principal: %w", err)
	}
	if !active {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) cookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: s.secureCookie(), SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(expires.Sub(s.cfg.Now()).Seconds())})
}

func (s *Service) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/v1/auth/") && path != "/api/v1/configurations" && !strings.HasPrefix(path, "/api/v1/configurations/") && path != "/api/v1/keys" && !strings.HasPrefix(path, "/api/v1/keys/") {
			next.ServeHTTP(w, r)
			return
		}
		if !s.AllowedRequest(r) {
			httpapi.WriteError(w, 403, "forbidden", "This local request is not allowed.")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		switch {
		case path == "/api/v1/auth/status" && r.Method == http.MethodGet:
			s.status(w)
		case path == "/api/v1/auth/me" && r.Method == http.MethodGet:
			s.me(w, r)
		case path == "/api/v1/auth/logout" && r.Method == http.MethodPost:
			s.logout(w, r)
		case path == "/api/v1/auth/email/start" && r.Method == http.MethodPost:
			s.emailStart(w, r)
		case path == "/api/v1/auth/email/verify" && r.Method == http.MethodPost:
			s.emailVerify(w, r)
		case strings.HasPrefix(path, "/api/v1/auth/google/") || strings.HasPrefix(path, "/api/v1/auth/github/"):
			s.oauth(w, r)
		case path == "/api/v1/configurations" || strings.HasPrefix(path, "/api/v1/configurations/"):
			s.configurations(w, r)
		case path == "/api/v1/keys" || strings.HasPrefix(path, "/api/v1/keys/"):
			s.keys(w, r)
		default:
			httpapi.WriteError(w, 404, "not_found", "This account action is unavailable.")
		}
	})
}

func (s *Service) status(w http.ResponseWriter) {
	secretReady := len(s.cfg.Secret) >= 32
	provider := func(ready bool, reason string) map[string]any {
		if ready {
			reason = ""
		}
		return map[string]any{"configured": ready, "reason": reason}
	}
	httpapi.WriteJSON(w, 200, map[string]any{
		"email":  provider(secretReady && s.cfg.Mailer != nil, "Configure RESEND_API_KEY, RESEND_FROM and SYNTELLE_AUTH_SECRET with a verified sender."),
		"google": provider(secretReady && s.cfg.GoogleClientID != "" && s.cfg.GoogleClientSecret != "", "Configure GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and SYNTELLE_AUTH_SECRET."),
		"github": provider(secretReady && s.cfg.GitHubClientID != "" && s.cfg.GitHubClientSecret != "", "Configure GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET and SYNTELLE_AUTH_SECRET."),
	})
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	p, err := s.Resolve(w, r, true)
	if err != nil {
		accessError(w, err)
		return
	}
	if p.Kind == "key" {
		accessError(w, ErrForbidden)
		return
	}
	var account *Account
	if p.AccountID != "" {
		account, err = s.account(r.Context(), p.AccountID)
		if err != nil {
			accessError(w, err)
			return
		}
	}
	httpapi.WriteJSON(w, 200, map[string]any{"account": account, "principal": p})
}

func (s *Service) account(ctx context.Context, id string) (*Account, error) {
	a := &Account{}
	err := s.Pool.QueryRow(ctx, "SELECT id,email,display_name FROM syntelle_accounts.accounts WHERE id=$1 AND disabled_at IS NULL", id).Scan(&a.ID, &a.Email, &a.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnauthorized
	}
	return a, err
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	p, err := s.Resolve(w, r, false)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			s.clearCookie(w)
			httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		accessError(w, err)
		return
	}
	if p.Kind == "key" {
		accessError(w, ErrForbidden)
		return
	}
	if _, err = s.Pool.Exec(r.Context(), "UPDATE syntelle_accounts.sessions SET revoked_at=$2 WHERE id=$1", p.SessionID, s.cfg.Now()); err != nil {
		accessError(w, err)
		return
	}
	s.clearCookie(w)
	if s.OnRevoked != nil {
		s.OnRevoked("session", p.SessionID)
	}
	httpapi.WriteJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Service) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: s.secureCookie(), SameSite: http.SameSiteLaxMode})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		httpapi.WriteError(w, 400, "invalid_request", "Check the submitted fields.")
		return false
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		httpapi.WriteError(w, 400, "invalid_request", "Send one JSON object.")
		return false
	}
	return true
}

func accessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		httpapi.WriteError(w, 401, "sign_in_required", "Sign in to continue.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, 403, "forbidden", "This action is not available to this account.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, 404, "not_found", "This resource is unavailable.")
	default:
		httpapi.WriteError(w, 503, "account_unavailable", "Account storage is temporarily unavailable.")
	}
}

func randomID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// The runtime cannot safely issue any authentication material if its
		// operating-system entropy source is broken.
		panic("system cryptographic randomness unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func digest(value string) string {
	b := sha256.Sum256([]byte(value))
	return hex.EncodeToString(b[:])
}

func (s *Service) verifier(parts ...string) string {
	h := hmac.New(sha256.New, []byte(s.cfg.Secret))
	for _, p := range parts {
		h.Write([]byte(strconv.Itoa(len(p)) + ":" + p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func same(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

func (s *Service) encrypt(value []byte) ([]byte, error) {
	key := sha256.Sum256([]byte(s.cfg.Secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, value, []byte("syntelle-oauth-flow/1")), nil
}

func (s *Service) decrypt(value []byte) ([]byte, error) {
	key := sha256.Sum256([]byte(s.cfg.Secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(value) < gcm.NonceSize() {
		return nil, errors.New("invalid encrypted login flow")
	}
	return gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], []byte("syntelle-oauth-flow/1"))
}
