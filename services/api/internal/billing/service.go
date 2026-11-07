package billing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"syntelle/api/internal/httpapi"
	"time"
)

type Service struct {
	pool   *pgxpool.Pool
	cfg    Config
	client *http.Client
}

func New(pool *pgxpool.Pool, cfg Config) *Service {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	// Even injected clients cannot redirect a credential to another host.
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Service{pool: pool, cfg: cfg, client: &copy}
}
func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func digest(b []byte) string     { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func financial(role string) bool { return role == "owner" || role == "billing" }
func fail(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteError(w, status, code, message)
}
func (s *Service) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/billing/paddle/webhook" {
			s.webhook(w, r)
			return
		}
		prefix := "/api/v1/workspaces/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			next.ServeHTTP(w, r)
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
		if len(parts) < 2 || (parts[1] != "billing" && parts[1] != "usage") {
			next.ServeHTTP(w, r)
			return
		}
		if s.pool == nil || s.cfg.AuthorizeWorkspace == nil {
			fail(w, 503, "billing_unavailable", "Workspace billing is unavailable.")
			return
		}
		actor, role, suspended, ok := s.cfg.AuthorizeWorkspace(w, r, parts[0])
		if !ok {
			return
		}
		if parts[1] == "usage" && len(parts) == 2 && r.Method == http.MethodGet {
			usage, err := s.Usage(r.Context(), parts[0])
			if err != nil {
				fail(w, 503, "usage_unavailable", "Usage could not be loaded.")
				return
			}
			httpapi.WriteJSON(w, 200, usage)
			return
		}
		if !financial(role) {
			fail(w, 403, "forbidden", "Only the workspace owner or billing manager can manage billing.")
			return
		}
		if len(parts) == 2 && parts[1] == "billing" && r.Method == http.MethodGet {
			s.overview(w, r, parts[0])
			return
		}
		if suspended {
			fail(w, 403, "workspace_suspended", "This workspace is suspended.")
			return
		}
		if r.Method != http.MethodPost || len(parts) != 3 {
			fail(w, 405, "method_not_allowed", "Unsupported billing action.")
			return
		}
		if !s.cfg.configured() {
			fail(w, 503, "billing_unavailable", "Paddle sandbox is not configured. No payment was requested.")
			return
		}
		s.action(w, r, parts[0], actor, parts[2])
	})
}

type Subscription struct {
	ID      string     `json:"id"`
	Plan    string     `json:"plan"`
	Status  string     `json:"status"`
	Start   *time.Time `json:"current_period_start"`
	End     *time.Time `json:"current_period_end"`
	Cancel  bool       `json:"cancel_at_period_end"`
	Pending string     `json:"pending_plan"`
}

func (s *Service) overview(w http.ResponseWriter, r *http.Request, wid string) {
	var sub Subscription
	var id *string
	var review bool
	err := s.pool.QueryRow(r.Context(), `SELECT subscription_id,plan,status,period_start,period_end,cancel_at_period_end,pending_plan,review_required FROM syntelle_billing.workspaces WHERE workspace_id=$1`, wid).Scan(&id, &sub.Plan, &sub.Status, &sub.Start, &sub.End, &sub.Cancel, &sub.Pending, &review)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fail(w, 503, "billing_unavailable", "Billing could not be loaded.")
		return
	}
	var subscription *Subscription
	if id != nil {
		sub.ID = *id
		subscription = &sub
	}
	reason := ""
	token := ""
	if !s.cfg.configured() {
		reason = "Paddle sandbox is not configured. Payments are unavailable."
	} else {
		token = s.cfg.ClientToken
	}
	httpapi.WriteJSON(w, 200, map[string]any{"configured": s.cfg.configured(), "environment": "sandbox", "reason": reason, "client_token": token, "can_manage": true, "subscription": subscription, "review_required": review, "catalog": map[string]any{"plans": plans, "topups": []Plan{topup}}})
}
func decode(w http.ResponseWriter, r *http.Request, input any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(input); err != nil && err != io.EOF {
		fail(w, 400, "invalid_request", "Invalid billing request.")
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		fail(w, 400, "invalid_request", "Invalid billing request.")
		return false
	}
	return true
}

// Run drains the durable inbox and expires abandoned usage reservations. Provider
// failures are retained; this loop does not repeat provider mutations.
func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	var reportedAt time.Time
	for {
		if err := s.Reconcile(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if reportedAt.IsZero() || time.Since(reportedAt) >= time.Minute {
				slog.Error("sandbox billing reconciliation paused", "reason", storageFailureCode(err))
				reportedAt = time.Now()
			}
		} else if !reportedAt.IsZero() {
			slog.Info("sandbox billing reconciliation recovered")
			reportedAt = time.Time{}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
