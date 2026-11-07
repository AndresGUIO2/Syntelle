package streaming

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"syntelle/api/internal/accounts"
	"syntelle/api/internal/httpapi"
)

// NewTurnstileVerifier keeps the private validation key on the API. The returned
// callback can be replaced by a local fixture without introducing a public bypass.
func NewTurnstileVerifier(secret, hostname string, client *http.Client) func(context.Context, string, string, string) error {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return func(ctx context.Context, token, ip, nonce string) error {
		if secret == "" || hostname == "" || token == "" || len(token) > 2048 || nonce == "" {
			return errors.New("Visitor verification is unavailable.")
		}
		form := url.Values{"secret": {secret}, "response": {token}, "remoteip": {ip}}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := client.Do(request)
		if err != nil {
			return errors.New("Visitor verification failed.")
		}
		defer response.Body.Close()
		var result struct {
			Success  bool   `json:"success"`
			Hostname string `json:"hostname"`
			Action   string `json:"action"`
			Data     string `json:"cdata"`
		}
		if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 16384)).Decode(&result) != nil || !result.Success || result.Hostname != hostname || result.Action != "widget_start" || result.Data != nonce {
			return errors.New("Visitor verification failed.")
		}
		return nil
	}
}
func nonceDigest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
func originAllowed(i accounts.PublishedIntegration, raw string) bool {
	normalized, err := accounts.NormalizeWidgetOrigin(raw)
	if err != nil || normalized != raw {
		return false
	}
	for _, allowed := range i.AllowedOrigins {
		if allowed == normalized {
			return true
		}
	}
	return false
}
func (s *Service) widgetRate(w http.ResponseWriter, r *http.Request, id string, start bool) bool {
	if s.admission == nil {
		writeError(w, 503, "Visitor admission is unavailable.")
		return false
	}
	suffix := "bootstrap"
	perIP, perWidget := 30, 120
	if start {
		suffix = "start"
		perIP, perWidget = 5, 30
	}
	ip := s.admission.clientIP(r)
	if retry := s.admission.allow("widget-ip-"+suffix, ip+":"+id, perIP, time.Now()); retry != 0 {
		limited(w, retry, "Please wait before starting another conversation.")
		return false
	}
	if retry := s.admission.allow("widget-"+suffix, id, perWidget, time.Now()); retry != 0 {
		limited(w, retry, "This character is busy. Try again shortly.")
		return false
	}
	return true
}
func (s *Service) widget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !localRequest(r) || s.config.ResolveWidget == nil {
		writeError(w, 404, "Widget is unavailable.")
		return
	}
	tail := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/widgets/"), "/")
	if len(tail) < 1 || len(tail) > 2 || !identifier.MatchString(tail[0]) {
		writeError(w, 404, "Widget is unavailable.")
		return
	}
	if !s.allowStreamRequest(w, r, "") {
		return
	}
	i, err := s.config.ResolveWidget(r.Context(), tail[0])
	if err != nil {
		writeError(w, 404, "Widget is unavailable.")
		return
	}
	if len(tail) == 1 && r.Method == http.MethodGet {
		poster := ""
		if i.Publication.PortraitAvailable {
			poster = "/api/v1/widgets/" + i.WidgetID + "/portrait"
		}
		available := s.config.TurnstileSiteKey != "" && s.config.VerifyWidgetChallenge != nil && s.config.ReserveIntegrationUsage != nil
		reason := ""
		if !available {
			reason = "This widget is not ready to start conversations."
		}
		writeJSON(w, 200, map[string]any{"id": i.WidgetID, "name": i.Name, "allowed_origins": i.AllowedOrigins, "appearance": i.Appearance, "player": map[string]any{"character_name": i.Publication.Name, "background": i.Settings.Background, "blur": i.Settings.Blur, "poster": poster}, "turnstile_site_key": s.config.TurnstileSiteKey, "available": available, "reason": reason})
		return
	}
	if len(tail) == 2 && tail[1] == "portrait" && r.Method == http.MethodGet {
		var data []byte
		var mime string
		err := s.store.pool.QueryRow(r.Context(), `SELECT portrait_bytes,portrait_mime FROM syntelle_accounts.publications WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL AND portrait_bytes IS NOT NULL`, i.PublicationID, i.WorkspaceID).Scan(&data, &mime)
		if err != nil {
			writeError(w, 404, "Portrait is unavailable.")
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		_, _ = w.Write(data)
		return
	}
	// Public widget routes deliberately ignore account cookies. Every mutation
	// originates from our iframe; the parent origin is a separate allowlist check.
	if r.Method != http.MethodPost || r.Header.Get("Origin") == "" || !httpapi.AllowedOrigin(r) {
		writeError(w, 403, "Widget request is not allowed.")
		return
	}
	if len(tail) == 2 && tail[1] == "bootstrap" {
		if !s.widgetRate(w, r, i.ID, false) {
			return
		}
		var input struct {
			ParentOrigin string `json:"parent_origin"`
		}
		if decode(w, r, &input) != nil || !originAllowed(i, input.ParentOrigin) {
			writeError(w, 403, "This website is not enabled for the widget.")
			return
		}
		id, err := randomID("bootstrap_")
		if err != nil {
			writeError(w, 500, "Visitor verification could not start.")
			return
		}
		nonce, err := randomID("")
		if err != nil {
			writeError(w, 500, "Visitor verification could not start.")
			return
		}
		expires := time.Now().Add(5 * time.Minute)
		_, err = s.store.pool.Exec(r.Context(), `INSERT INTO syntelle_accounts.widget_bootstraps(id,integration_id,revision,parent_origin,nonce_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, id, i.ID, i.PublishedRevision, input.ParentOrigin, nonceDigest(nonce), expires)
		if err != nil {
			writeError(w, 503, "Visitor verification could not start.")
			return
		}
		writeJSON(w, 201, map[string]any{"bootstrap_id": id, "nonce": nonce, "expires_at": expires})
		return
	}
	if len(tail) == 2 && tail[1] == "sessions" {
		if !s.widgetRate(w, r, i.ID, true) {
			return
		}
		var input struct {
			BootstrapID    string `json:"bootstrap_id"`
			Nonce          string `json:"nonce"`
			ParentOrigin   string `json:"parent_origin"`
			TurnstileToken string `json:"turnstile_token"`
			RequestID      string `json:"request_id"`
		}
		if decode(w, r, &input) != nil || !identifier.MatchString(input.BootstrapID) || !identifier.MatchString(input.RequestID) || len(input.Nonce) > 128 || !originAllowed(i, input.ParentOrigin) {
			writeError(w, 403, "Invalid visitor session request.")
			return
		}
		if s.config.VerifyWidgetChallenge == nil {
			writeError(w, 503, "Visitor verification is unavailable.")
			return
		}
		var valid bool
		err = s.store.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.widget_bootstraps WHERE id=$1 AND integration_id=$2 AND revision=$3 AND parent_origin=$4 AND nonce_hash=$5 AND expires_at>now() AND consumed_at IS NULL)`, input.BootstrapID, i.ID, i.PublishedRevision, input.ParentOrigin, nonceDigest(input.Nonce)).Scan(&valid)
		if err != nil || !valid {
			writeError(w, 409, "Visitor verification expired or was already used. Start again.")
			return
		}
		verifyCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		err = s.config.VerifyWidgetChallenge(verifyCtx, input.TurnstileToken, s.admission.clientIP(r), input.Nonce)
		cancel()
		if err != nil {
			writeError(w, 403, "Visitor verification failed. Start again.")
			return
		}
		changed, err := s.store.pool.Exec(r.Context(), `UPDATE syntelle_accounts.widget_bootstraps SET consumed_at=now(),request_id=$6 WHERE id=$1 AND integration_id=$2 AND revision=$3 AND parent_origin=$4 AND nonce_hash=$5 AND expires_at>now() AND consumed_at IS NULL`, input.BootstrapID, i.ID, i.PublishedRevision, input.ParentOrigin, nonceDigest(input.Nonce), input.RequestID)
		if err != nil || changed.RowsAffected() != 1 {
			writeError(w, 409, "Visitor verification was already used. Start again.")
			return
		}
		visitor, err := randomID("")
		if err != nil {
			writeError(w, 500, "Visitor session could not be created.")
			return
		}
		p := accounts.Principal{ID: "widget:" + visitor, Kind: "widget", VisitorID: visitor, WorkspaceID: i.WorkspaceID, IntegrationID: i.ID, PublicationID: i.PublicationID, IntegrationRevision: i.PublishedRevision}
		s.createInput(w, r, p, streamCreateInput{IntegrationID: i.ID})
		return
	}
	writeError(w, 404, "Widget action not found.")
}

func (s *Service) resolveStreamPrincipal(w http.ResponseWriter, r *http.Request) (accounts.Principal, error) {
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer syntelle_wp_") {
		return s.resolve(w, r, true)
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	if len(token) > 128 {
		return accounts.Principal{}, accounts.ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(token))
	var principal accounts.Principal
	s.mu.Lock()
	for _, session := range s.sessions {
		if session.principal.Kind == "widget" && time.Now().Before(session.deadline) && subtle.ConstantTimeCompare(hash[:], session.capabilityHash[:]) == 1 {
			principal = session.principal
			break
		}
	}
	s.mu.Unlock()
	if principal.ID == "" || s.config.ValidatePrincipal == nil {
		return accounts.Principal{}, accounts.ErrUnauthorized
	}
	if err := s.config.ValidatePrincipal(r.Context(), principal); err != nil {
		return accounts.Principal{}, err
	}
	return principal, nil
}
