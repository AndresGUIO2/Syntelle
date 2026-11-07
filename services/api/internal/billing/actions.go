package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syntelle/api/internal/httpapi"
	"time"
)

type actionInput struct {
	Plan  string `json:"plan,omitempty"`
	Topup string `json:"topup,omitempty"`
}

func (s *Service) action(w http.ResponseWriter, r *http.Request, wid, actor, kind string) {
	if kind != "checkout" && kind != "subscription" && kind != "topup" && kind != "cancel" && kind != "payment-method" && kind != "reconcile" {
		fail(w, 404, "not_found", "Billing action not found.")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 || strings.ContainsAny(key, "\r\n\x00") {
		fail(w, 400, "idempotency_required", "Use a unique Idempotency-Key for this billing action.")
		return
	}
	var input actionInput
	if !decode(w, r, &input) {
		return
	}
	target := input.Plan
	if kind == "topup" {
		target = input.Topup
	}
	if kind == "checkout" || kind == "subscription" {
		if _, ok := planByID(target); !ok {
			fail(w, 400, "invalid_plan", "Choose a sandbox test plan.")
			return
		}
	}
	if kind == "topup" && target != "test_topup" {
		fail(w, 400, "invalid_topup", "Choose a sandbox test top-up.")
		return
	}
	raw, _ := json.Marshal(input)
	hash := digest(append([]byte(kind+":"), raw...))
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		fail(w, 503, "billing_unavailable", "Billing is unavailable.")
		return
	}
	defer tx.Rollback(r.Context())
	// Membership mutations use the account workspace lock. Recheck authority
	// under that same lock before a durable financial action is admitted.
	var currentRole string
	var workspaceSuspended bool
	err = tx.QueryRow(r.Context(), `SELECT m.role,w.suspended_at IS NOT NULL FROM syntelle_accounts.workspaces w JOIN syntelle_accounts.memberships m ON m.workspace_id=w.id WHERE w.id=$1 AND m.account_id=$2 FOR UPDATE OF w`, wid, actor).Scan(&currentRole, &workspaceSuspended)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!financial(currentRole) || workspaceSuspended) {
		fail(w, 403, "forbidden", "Workspace payment authority changed. Reload billing before continuing.")
		return
	}
	if err != nil {
		fail(w, 503, "billing_unavailable", "Billing is unavailable.")
		return
	}
	_, err = s.lockWorkspace(r.Context(), tx, wid)
	if err != nil {
		fail(w, 503, "billing_unavailable", "Billing is unavailable.")
		return
	}
	var oldID, oldHash, state string
	var cached []byte
	err = tx.QueryRow(r.Context(), `SELECT id,input_hash,state,response FROM syntelle_billing.actions WHERE workspace_id=$1 AND idempotency_key=$2`, wid, key).Scan(&oldID, &oldHash, &state, &cached)
	if err == nil {
		if oldHash != hash {
			fail(w, 409, "idempotency_conflict", "This action key already belongs to another request.")
			return
		}
		if state == "completed" {
			httpapi.WriteJSON(w, 200, json.RawMessage(cached))
			return
		}
		fail(w, 409, "billing_action_pending", "This billing action was already attempted. Reconcile its result before trying again.")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		fail(w, 503, "billing_unavailable", "Billing is unavailable.")
		return
	}
	var subscription, customer *string
	var current, status string
	var end *time.Time
	err = tx.QueryRow(r.Context(), `SELECT subscription_id,customer_id,plan,status,period_end FROM syntelle_billing.workspaces WHERE workspace_id=$1`, wid).Scan(&subscription, &customer, &current, &status, &end)
	if err != nil {
		fail(w, 503, "billing_unavailable", "Billing is unavailable.")
		return
	}
	if kind == "checkout" && subscription != nil && status != "canceled" && status != "none" {
		fail(w, 409, "subscription_exists", "Change the current subscription instead of starting another.")
		return
	}
	if kind != "checkout" && kind != "reconcile" && (subscription == nil || current == "") {
		fail(w, 409, "subscription_required", "Start a subscription first.")
		return
	}
	if (kind == "subscription" || kind == "topup") && status != "active" {
		fail(w, 409, "subscription_inactive", "The subscription must be active for this action.")
		return
	}
	if kind == "subscription" && target == current {
		fail(w, 409, "plan_unchanged", "This plan is already active.")
		return
	}
	// Only one unresolved provider mutation per workspace; timeouts are not retried.
	var unresolved bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM syntelle_billing.actions WHERE workspace_id=$1 AND state IN ('reserved','unknown'))`, wid).Scan(&unresolved)
	if err != nil || (unresolved && kind != "reconcile") {
		fail(w, 409, "billing_action_pending", "A previous billing action needs reconciliation.")
		return
	}
	if kind == "checkout" {
		var previous []byte
		err = tx.QueryRow(r.Context(), `SELECT response FROM syntelle_billing.actions WHERE workspace_id=$1 AND kind='checkout' AND plan=$2 AND state='completed' ORDER BY created_at DESC LIMIT 1`, wid, target).Scan(&previous)
		if err == nil && status == "none" {
			httpapi.WriteJSON(w, 200, json.RawMessage(previous))
			return
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			fail(w, 503, "billing_unavailable", "Billing is unavailable.")
			return
		}
	}
	id := randomID()
	_, err = tx.Exec(r.Context(), `INSERT INTO syntelle_billing.actions(id,workspace_id,actor_id,idempotency_key,kind,input_hash,request,plan) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, wid, actor, key, kind, hash, raw, target)
	if err != nil || tx.Commit(r.Context()) != nil {
		fail(w, 503, "billing_unavailable", "Billing action could not be reserved.")
		return
	}
	result, err := s.executeAction(r.Context(), wid, id, kind, target, current, subscription, customer, end)
	if err != nil {
		state := "unknown"
		var known *knownActionFailure
		var rejected *providerRejection
		if errors.As(err, &known) || (errors.As(err, &rejected) && rejected.status >= 400 && rejected.status < 500 && rejected.status != 408) {
			state = "failed"
		}
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 3*time.Second)
		defer cancel()
		_, _ = s.pool.Exec(persistCtx, `UPDATE syntelle_billing.actions SET state=$2,failure_code='provider_action_incomplete' WHERE id=$1 AND state='reserved'`, id, state)
		fail(w, 502, "billing_action_incomplete", "The sandbox action did not complete. Its result must be reconciled; no automatic retry was made.")
		return
	}
	body, _ := json.Marshal(result)
	providerID, _ := result["transaction_id"].(string)
	_, err = s.pool.Exec(r.Context(), `UPDATE syntelle_billing.actions SET state='completed',response=$2,provider_id=NULLIF($3,''),completed_at=clock_timestamp() WHERE id=$1`, id, body, providerID)
	if err != nil {
		fail(w, 503, "billing_action_pending", "The sandbox accepted this action, but local confirmation is pending. Do not repeat it.")
		return
	}
	httpapi.WriteJSON(w, 200, result)
}
func (s *Service) verifyPrice(ctx context.Context, plan string) error {
	actual, err := s.provider(ctx, http.MethodGet, "/prices/"+url.PathEscape(s.cfg.price(plan)), nil)
	if err != nil {
		return err
	}
	expected, ok := planByID(plan)
	if !ok {
		expected = topup
	}
	if actual.ID != s.cfg.price(plan) || actual.UnitPrice.Currency != "USD" || actual.UnitPrice.Amount != strconv.Itoa(expected.Amount) || actual.Status != "active" {
		return errors.New("sandbox catalog does not match the configured test plan")
	}
	if plan == "test_topup" {
		if actual.BillingCycle != nil {
			return errors.New("top-up must be non-recurring")
		}
	} else if actual.BillingCycle == nil || actual.BillingCycle.Interval != "month" || actual.BillingCycle.Frequency != 1 {
		return errors.New("test plans must be monthly")
	}
	return nil
}
func (s *Service) executeAction(ctx context.Context, wid, actionID, kind, target, current string, subscription, customer *string, end *time.Time) (map[string]any, error) {
	ctx = context.WithValue(ctx, actionContextKey{}, actionID)
	result := map[string]any{"environment": "sandbox"}
	if kind == "checkout" || kind == "topup" || kind == "subscription" {
		if err := s.verifyPrice(ctx, target); err != nil {
			return nil, &knownActionFailure{err}
		}
	}
	metadata := map[string]string{"workspace_id": wid, "action_id": actionID}
	switch kind {
	case "checkout", "topup":
		input := map[string]any{"items": []map[string]any{{"price_id": s.cfg.price(target), "quantity": 1}}, "collection_mode": "automatic", "currency_code": "USD", "custom_data": metadata}
		if customer != nil {
			input["customer_id"] = *customer
		}
		data, err := s.provider(ctx, http.MethodPost, "/transactions", input)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(data.ID, "txn_") {
			return nil, errors.New("missing transaction ID")
		}
		result["transaction_id"] = data.ID
	case "subscription":
		old, _ := planByID(current)
		next, _ := planByID(target)
		mode := "prorated_immediately"
		if next.IncludedMinutes < old.IncludedMinutes {
			mode = "do_not_bill"
		}
		data, err := s.provider(ctx, http.MethodPatch, "/subscriptions/"+url.PathEscape(*subscription), map[string]any{"items": []map[string]any{{"price_id": s.cfg.price(target), "quantity": 1}}, "proration_billing_mode": mode, "on_payment_failure": "prevent_change", "custom_data": metadata})
		if err != nil {
			return nil, err
		}
		if data.ID != *subscription {
			return nil, errors.New("subscription mismatch")
		}
		if mode == "do_not_bill" {
			_, err = s.pool.Exec(ctx, `UPDATE syntelle_billing.workspaces SET pending_plan=$2 WHERE workspace_id=$1`, wid, target)
			if err != nil {
				return nil, err
			}
			result["status"] = "scheduled"
			result["effective_at"] = end
		} else {
			result["status"] = "pending_payment"
			result["effective_at"] = nil
		}
	case "cancel":
		data, err := s.provider(ctx, http.MethodPost, "/subscriptions/"+url.PathEscape(*subscription)+"/cancel", map[string]string{"effective_from": "next_billing_period"})
		if err != nil {
			return nil, err
		}
		if data.ID != *subscription || data.ScheduledChange == nil || data.ScheduledChange.Action != "cancel" {
			return nil, errors.New("cancellation was not scheduled")
		}
		_, err = s.pool.Exec(ctx, `UPDATE syntelle_billing.workspaces SET cancel_at_period_end=true WHERE workspace_id=$1`, wid)
		if err != nil {
			return nil, err
		}
		result["status"] = "scheduled"
		result["effective_at"] = data.ScheduledChange.EffectiveAt
	case "payment-method":
		data, err := s.provider(ctx, http.MethodGet, "/subscriptions/"+url.PathEscape(*subscription)+"/update-payment-method-transaction", nil)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(data.ID, "txn_") || data.SubscriptionID != *subscription {
			return nil, errors.New("payment-method transaction is not bound to the workspace subscription")
		}
		result["transaction_id"] = data.ID
	case "reconcile":
		if err := s.reconcileWorkspace(ctx, wid, actionID, subscription); err != nil {
			return nil, &knownActionFailure{err}
		}
		result["status"] = "reconciled"
	default:
		return nil, fmt.Errorf("unsupported billing action %q", kind)
	}
	return result, nil
}

// Reconciliation reads only identifiers already bound by this service. It never
// repeats a mutation or accepts a browser-supplied transaction/subscription ID.
func (s *Service) reconcileWorkspace(ctx context.Context, wid, actionID string, subscription *string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if subscription != nil {
		data, err := s.provider(ctx, http.MethodGet, "/subscriptions/"+url.PathEscape(*subscription), nil)
		if err != nil {
			return err
		}
		if data.ID != *subscription {
			return errors.New("subscription mismatch")
		}
		if err = s.ingestReconciliation(ctx, "subscription.updated", data); err != nil {
			return err
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT a.provider_id FROM syntelle_billing.actions a LEFT JOIN syntelle_billing.fulfillments f ON f.transaction_id=a.provider_id WHERE a.workspace_id=$1 AND a.provider_id LIKE 'txn_%' AND f.transaction_id IS NULL AND a.kind IN ('checkout','topup','subscription') LIMIT 5`, wid)
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
		data, err := s.provider(ctx, http.MethodGet, "/transactions/"+url.PathEscape(id), nil)
		if err != nil {
			return err
		}
		if data.ID != id {
			return errors.New("transaction mismatch")
		}
		if data.Status == "completed" {
			if err = s.ingestReconciliation(ctx, "transaction.completed", data); err != nil {
				return err
			}
		}
	}
	if err = s.Reconcile(ctx); err != nil {
		return err
	}
	var unresolved bool
	err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_billing.actions WHERE workspace_id=$1 AND id<>$2 AND state IN ('reserved','unknown'))`, wid, actionID).Scan(&unresolved)
	if err != nil {
		return err
	}
	if unresolved {
		return errors.New("a provider mutation still needs a matching authenticated result")
	}
	return nil
}
