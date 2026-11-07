package billing

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"net/http"
	"strconv"
	"strings"
	"syntelle/api/internal/httpapi"
	"time"
)

type event struct {
	ID       string       `json:"event_id"`
	Type     string       `json:"event_type"`
	Occurred time.Time    `json:"occurred_at"`
	Data     providerData `json:"data"`
}

func (s *Service) webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "method_not_allowed", "Unsupported webhook method.")
		return
	}
	if s.pool == nil || !s.cfg.configured() {
		fail(w, 503, "billing_unavailable", "Sandbox billing is unavailable.")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		fail(w, 413, "invalid_webhook", "Webhook is too large.")
		return
	}
	if !verifySignature(raw, r.Header.Get("Paddle-Signature"), s.cfg.WebhookSecret, s.cfg.Now()) {
		fail(w, 401, "invalid_signature", "Webhook signature is invalid.")
		return
	}
	var value event
	if json.Unmarshal(raw, &value) != nil || value.ID == "" || len(value.ID) > 120 || value.Type == "" || value.Occurred.IsZero() {
		fail(w, 400, "invalid_webhook", "Webhook is invalid.")
		return
	}
	if err = s.insertEvent(r.Context(), value, raw, "signed_webhook"); err != nil {
		fail(w, 503, "webhook_not_saved", "Webhook could not be saved.")
		return
	}
	// Acknowledgement means durably received. Run/Reconcile owns processing.
	httpapi.WriteJSON(w, 200, map[string]bool{"received": true})
}
func (s *Service) insertEvent(ctx context.Context, value event, raw []byte, source string) error {
	var hash string
	err := s.pool.QueryRow(ctx, `INSERT INTO syntelle_billing.webhook_inbox(event_id,event_type,occurred_at,payload,payload_hash,source) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(event_id) DO UPDATE SET event_id=EXCLUDED.event_id RETURNING payload_hash`, value.ID, value.Type, value.Occurred, raw, digest(raw), source).Scan(&hash)
	if err == nil && hash != digest(raw) {
		return errors.New("duplicate webhook ID has different bytes")
	}
	return err
}
func (s *Service) ingestReconciliation(ctx context.Context, kind string, data providerData) error {
	if data.UpdatedAt.IsZero() {
		return errors.New("provider reconciliation has no update timestamp")
	}
	rawData, _ := json.Marshal(data)
	value := event{ID: "reconcile_" + digest(rawData), Type: kind, Occurred: data.UpdatedAt, Data: data}
	raw, _ := json.Marshal(value)
	if err := s.insertEvent(ctx, value, raw, "authenticated_sandbox_api"); err != nil {
		return err
	}
	return s.processEvent(ctx, value.ID)
}
func (s *Service) Reconcile(ctx context.Context) error {
	if s.pool == nil {
		return ErrUnavailable
	}
	rows, err := s.pool.Query(ctx, `SELECT event_id FROM syntelle_billing.webhook_inbox WHERE state='pending' ORDER BY attempted_at NULLS FIRST,occurred_at,event_id LIMIT 100`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.processEvent(ctx, id); err != nil {
			return err
		}
	}
	rows, err = s.pool.Query(ctx, `SELECT DISTINCT workspace_id FROM syntelle_billing.usage_sessions WHERE released_at IS NULL AND (lease_expires_at<=clock_timestamp() OR deadline<=clock_timestamp()) LIMIT 100`)
	if err != nil {
		return err
	}
	var workspaces []string
	for rows.Next() {
		var wid string
		if err = rows.Scan(&wid); err != nil {
			rows.Close()
			return err
		}
		workspaces = append(workspaces, wid)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, wid := range workspaces {
		tx, e := s.pool.Begin(ctx)
		if e != nil {
			return e
		}
		now, e := s.lockWorkspace(ctx, tx, wid)
		if e == nil {
			e = s.reap(ctx, tx, wid, now)
		}
		if e == nil {
			e = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func storageFailureCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "database_" + pgErr.Code
	}
	if errors.Is(err, context.Canceled) {
		return "processing_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "processing_timeout"
	}
	return "storage_unavailable"
}

func (s *Service) processEvent(ctx context.Context, id string) (resultErr error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
		if resultErr != nil {
			persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			// During an outage the immutable pending inbox is the recovery record;
			// the run loop reports storage failure without logging event payloads.
			_, _ = s.pool.Exec(persistCtx, `UPDATE syntelle_billing.webhook_inbox SET reason=$2,attempted_at=clock_timestamp() WHERE event_id=$1 AND state='pending'`, id, storageFailureCode(resultErr))
		}
	}()
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT payload FROM syntelle_billing.webhook_inbox WHERE event_id=$1 AND state='pending' FOR UPDATE SKIP LOCKED`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var e event
	if err = json.Unmarshal(raw, &e); err != nil {
		return err
	}
	state, reason, err := s.applyEvent(ctx, tx, e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE syntelle_billing.webhook_inbox SET state=$2,reason=$3,attempted_at=clock_timestamp(),processed_at=CASE WHEN $2='pending' THEN NULL ELSE clock_timestamp() END WHERE event_id=$1`, id, state, reason)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) eventWorkspace(ctx context.Context, tx pgx.Tx, e event) (string, string, string, error) {
	var wid, kind, requested string
	sub := e.Data.SubscriptionID
	if strings.HasPrefix(e.Type, "subscription.") {
		sub = e.Data.ID
	}
	if e.Data.CustomData.ActionID != "" {
		err := tx.QueryRow(ctx, `SELECT workspace_id,kind,plan FROM syntelle_billing.actions WHERE id=$1`, e.Data.CustomData.ActionID).Scan(&wid, &kind, &requested)
		if err == nil && wid == e.Data.CustomData.WorkspaceID {
			return wid, kind, requested, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", err
		}
	}
	if sub != "" {
		err := tx.QueryRow(ctx, `SELECT workspace_id FROM syntelle_billing.workspaces WHERE subscription_id=$1`, sub).Scan(&wid)
		if err == nil {
			return wid, "", "", nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", err
		}
	}
	if e.Data.TransactionID != "" {
		err := tx.QueryRow(ctx, `SELECT workspace_id FROM syntelle_billing.fulfillments WHERE transaction_id=$1`, e.Data.TransactionID).Scan(&wid)
		if err == nil {
			return wid, "", "", nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", err
		}
	}
	return "", "", "", nil
}
func (s *Service) applyEvent(ctx context.Context, tx pgx.Tx, e event) (string, string, error) {
	if !strings.HasPrefix(e.Type, "subscription.") && !strings.HasPrefix(e.Type, "transaction.") && !strings.HasPrefix(e.Type, "adjustment.") {
		return "processed", "unrelated_event", nil
	}
	wid, action, requested, err := s.eventWorkspace(ctx, tx, e)
	if err != nil {
		return "", "", err
	}
	if wid == "" {
		return "pending", "awaiting_workspace_binding", nil
	}
	now, err := s.lockWorkspace(ctx, tx, wid)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(e.Type, "adjustment.") {
		if e.Data.TransactionID == "" {
			return "rejected", "adjustment_missing_transaction", nil
		}
		if e.Data.Status != "approved" && e.Data.Type != "chargeback" && e.Data.Action != "chargeback" {
			return "processed", "adjustment_not_final", nil
		}
		if _, err = tx.Exec(ctx, `UPDATE syntelle_billing.grants SET frozen=true WHERE workspace_id=$1 AND transaction_id=$2`, wid, e.Data.TransactionID); err != nil {
			return "", "", err
		}
		if _, err = tx.Exec(ctx, `UPDATE syntelle_billing.workspaces SET review_required=true WHERE workspace_id=$1`, wid); err != nil {
			return "", "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO syntelle_billing.ledger(workspace_id,transaction_id,kind,seconds) VALUES($1,$2,'freeze',0)`, wid, e.Data.TransactionID)
		return "processed", "grant_frozen_for_review", err
	}
	var bound *string
	var current, status string
	var start, end *time.Time
	var last time.Time
	err = tx.QueryRow(ctx, `SELECT subscription_id,plan,status,period_start,period_end,subscription_event_at FROM syntelle_billing.workspaces WHERE workspace_id=$1`, wid).Scan(&bound, &current, &status, &start, &end, &last)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(e.Type, "subscription.") {
		if !strings.HasPrefix(e.Data.ID, "sub_") {
			return "rejected", "invalid_subscription_id", nil
		}
		if bound != nil && *bound != e.Data.ID && status != "canceled" {
			return "rejected", "workspace_subscription_conflict", nil
		}
		if bound == nil && action != "checkout" {
			return "pending", "awaiting_checkout_binding", nil
		}
		if e.Occurred.Before(last) {
			return "processed", "superseded_subscription_event", nil
		}
		cancellation := e.Data.ScheduledChange != nil && e.Data.ScheduledChange.Action == "cancel"
		newStatus := e.Data.Status
		if newStatus == "active" && current == "" {
			newStatus = "pending_payment"
		}
		_, err = tx.Exec(ctx, `UPDATE syntelle_billing.workspaces SET subscription_id=$2,customer_id=COALESCE(NULLIF($3,''),customer_id),status=$4,cancel_at_period_end=$5,subscription_event_at=$6 WHERE workspace_id=$1`, wid, e.Data.ID, e.Data.CustomerID, newStatus, cancellation, e.Occurred)
		if err != nil {
			return "", "", err
		}
		if cancellation {
			response, _ := json.Marshal(map[string]any{"environment": "sandbox", "status": "scheduled", "effective_at": e.Data.ScheduledChange.EffectiveAt})
			_, err = tx.Exec(ctx, `UPDATE syntelle_billing.actions SET state='completed',response=COALESCE(response,$2),completed_at=clock_timestamp() WHERE workspace_id=$1 AND kind='cancel' AND state IN ('reserved','unknown')`, wid, response)
		}
		return "processed", "subscription_state_recorded", err
	}
	if e.Type != "transaction.completed" || e.Data.Status != "completed" {
		return "processed", "transaction_not_completed", nil
	}
	if e.Data.Origin == "subscription_payment_method_change" {
		return "processed", "payment_method_only", nil
	}
	total, parseErr := strconv.ParseInt(e.Data.Details.Totals.Total, 10, 64)
	if !strings.HasPrefix(e.Data.ID, "txn_") || e.Data.Currency != "USD" || parseErr != nil || total <= 0 || len(e.Data.Items) != 1 || e.Data.Items[0].Quantity != 1 {
		return "rejected", "unqualified_paid_transaction", nil
	}
	plan := s.cfg.plan(e.Data.Items[0].Price.ID)
	if plan == "" {
		return "rejected", "unrecognized_price", nil
	}
	var already bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_billing.fulfillments WHERE transaction_id=$1)`, e.Data.ID).Scan(&already); err != nil {
		return "", "", err
	}
	if already {
		return "processed", "transaction_already_fulfilled", nil
	}
	if plan == "test_topup" {
		if action != "topup" || requested != plan || bound == nil {
			return "rejected", "topup_not_authorized", nil
		}
		err = s.grant(ctx, tx, e, wid, plan, "topup", e.Data.ID, 1800, nil)
		if err != nil {
			return "", "", err
		}
		return "processed", "topup_granted", s.completeAction(ctx, tx, e.Data.CustomData.ActionID, e.Data.ID)
	}
	p, _ := planByID(plan)
	if e.Data.SubscriptionID == "" {
		return "pending", "awaiting_subscription_id", nil
	}
	if bound != nil && *bound != e.Data.SubscriptionID && status != "canceled" {
		return "rejected", "workspace_subscription_conflict", nil
	}
	if (bound == nil || current == "") && (action != "checkout" || requested != plan) {
		return "pending", "awaiting_authorized_checkout", nil
	}
	period := e.Data.BillingPeriod
	if period == nil || !period.EndsAt.After(period.StartsAt) {
		return "pending", "awaiting_paid_period", nil
	}
	if !period.EndsAt.After(now) {
		return "processed", "expired_paid_period", nil
	}
	if period.StartsAt.After(now) {
		return "pending", "awaiting_paid_period_start", nil
	}
	if end != nil && period.EndsAt.Before(*end) {
		return "processed", "superseded_paid_period", nil
	}
	if end != nil && period.EndsAt.After(*end) && period.StartsAt.Before(*end) {
		return "rejected", "overlapping_paid_period", nil
	}
	kind := "monthly"
	amount := int64(p.IncludedMinutes * 60)
	grantKey := e.Data.SubscriptionID + ":" + period.StartsAt.UTC().Format(time.RFC3339Nano)
	if start != nil && end != nil && period.EndsAt.Equal(*end) && current != "" {
		old, _ := planByID(current)
		if plan == current {
			return "rejected", "duplicate_paid_period", nil
		}
		if p.IncludedMinutes < old.IncludedMinutes {
			return "rejected", "downgrade_before_renewal", nil
		}
		if action != "subscription" || requested != plan {
			return "rejected", "upgrade_not_authorized", nil
		}
		kind = "upgrade"
		remaining := end.Sub(e.Occurred).Seconds()
		duration := end.Sub(*start).Seconds()
		if remaining < 0 || duration <= 0 {
			return "rejected", "invalid_upgrade_period", nil
		}
		if remaining > duration {
			remaining = duration
		}
		amount = int64(float64((p.IncludedMinutes-old.IncludedMinutes)*60) * remaining / duration)
		grantKey = e.Data.ID
		fullPeriod := *period
		fullPeriod.StartsAt = *start
		fullPeriod.EndsAt = *end
		period = &fullPeriod
	}
	// A later subscription cancellation/past-due event cannot be undone by an older payment.
	nextStatus := "active"
	if e.Occurred.Before(last) && status != "active" && status != "pending_payment" {
		nextStatus = status
	}
	if err = s.grant(ctx, tx, e, wid, plan, kind, grantKey, amount, &period.EndsAt); err != nil {
		return "", "", err
	}
	_, err = tx.Exec(ctx, `UPDATE syntelle_billing.workspaces SET subscription_id=$2,customer_id=COALESCE(NULLIF($3,''),customer_id),plan=$4,status=$5,period_start=$6,period_end=$7,pending_plan='' WHERE workspace_id=$1`, wid, e.Data.SubscriptionID, e.Data.CustomerID, plan, nextStatus, period.StartsAt, period.EndsAt)
	if err != nil {
		return "", "", err
	}
	return "processed", "paid_usage_granted", s.completeAction(ctx, tx, e.Data.CustomData.ActionID, e.Data.ID)
}
func (s *Service) completeAction(ctx context.Context, tx pgx.Tx, id, transaction string) error {
	if id == "" {
		return nil
	}
	response, _ := json.Marshal(map[string]string{"environment": "sandbox", "transaction_id": transaction})
	_, err := tx.Exec(ctx, `UPDATE syntelle_billing.actions SET state='completed',provider_id=$2,response=COALESCE(response,$3),completed_at=clock_timestamp() WHERE id=$1`, id, transaction, response)
	return err
}
func (s *Service) grant(ctx context.Context, tx pgx.Tx, e event, wid, plan, kind, key string, seconds int64, expires *time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO syntelle_billing.fulfillments(transaction_id,workspace_id,event_id,plan,seconds) VALUES($1,$2,$3,$4,$5)`, e.Data.ID, wid, e.ID, plan, seconds)
	if err != nil {
		return err
	}
	var grantID int64
	err = tx.QueryRow(ctx, `INSERT INTO syntelle_billing.grants(workspace_id,transaction_id,grant_key,kind,amount_seconds,available_seconds,expires_at) VALUES($1,$2,$3,$4,$5,$5,$6) RETURNING id`, wid, e.Data.ID, key, kind, seconds, expires).Scan(&grantID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO syntelle_billing.ledger(workspace_id,transaction_id,grant_id,kind,seconds) VALUES($1,$2,$3,'grant',$4)`, wid, e.Data.ID, grantID, seconds)
	return err
}
