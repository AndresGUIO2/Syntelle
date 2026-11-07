package billing

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	"time"
)

type usageSession struct {
	ID, Workspace, Integration string
	Reserved, Used             int64
	Started, Released          *time.Time
	Deadline, Lease            time.Time
}

func (s *Service) lockWorkspace(ctx context.Context, tx pgx.Tx, wid string) (time.Time, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO syntelle_billing.workspaces(workspace_id) VALUES($1) ON CONFLICT DO NOTHING`, wid); err != nil {
		return time.Time{}, err
	}
	var now time.Time
	err := tx.QueryRow(ctx, `SELECT clock_timestamp() FROM syntelle_billing.workspaces WHERE workspace_id=$1 FOR UPDATE`, wid).Scan(&now)
	return now, err
}
func getUsage(ctx context.Context, tx pgx.Tx, id string) (usageSession, error) {
	var u usageSession
	err := tx.QueryRow(ctx, `SELECT session_id,workspace_id,integration_id,reserved_seconds,used_seconds,started_at,released_at,deadline,lease_expires_at FROM syntelle_billing.usage_sessions WHERE session_id=$1 FOR UPDATE`, id).Scan(&u.ID, &u.Workspace, &u.Integration, &u.Reserved, &u.Used, &u.Started, &u.Released, &u.Deadline, &u.Lease)
	return u, err
}
func (s *Service) meter(ctx context.Context, tx pgx.Tx, u usageSession, now time.Time, release bool) error {
	if u.Released != nil {
		return nil
	}
	until := now
	if until.After(u.Deadline) {
		until = u.Deadline
	}
	if until.After(u.Lease) {
		until = u.Lease
	}
	used := u.Used
	if u.Started != nil && until.After(*u.Started) {
		used = max(used, min(u.Reserved, int64(math.Ceil(until.Sub(*u.Started).Seconds()))))
	}
	delta := used - u.Used
	rows, err := tx.Query(ctx, `SELECT a.grant_id,a.reserved_seconds,a.used_seconds FROM syntelle_billing.usage_allocations a JOIN syntelle_billing.grants g ON g.id=a.grant_id WHERE session_id=$1 ORDER BY CASE WHEN g.kind='topup' THEN 1 ELSE 0 END,g.expires_at NULLS LAST,g.id FOR UPDATE OF a`, u.ID)
	if err != nil {
		return err
	}
	type allocation struct{ id, reserved, used int64 }
	var allocations []allocation
	for rows.Next() {
		var a allocation
		if err = rows.Scan(&a.id, &a.reserved, &a.used); err != nil {
			rows.Close()
			return err
		}
		allocations = append(allocations, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range allocations {
		consumed := min(delta, a.reserved-a.used)
		delta -= consumed
		if consumed > 0 {
			if _, err = tx.Exec(ctx, `UPDATE syntelle_billing.usage_allocations SET used_seconds=used_seconds+$2 WHERE session_id=$1 AND grant_id=$3`, u.ID, consumed, a.id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO syntelle_billing.ledger(workspace_id,session_id,grant_id,kind,seconds) VALUES($1,$2,$3,'consume',$4)`, u.Workspace, u.ID, a.id, consumed); err != nil {
				return err
			}
		}
		if release {
			unused := a.reserved - a.used - consumed
			if unused > 0 {
				if _, err = tx.Exec(ctx, `UPDATE syntelle_billing.grants SET available_seconds=available_seconds+$2 WHERE id=$1`, a.id, unused); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `INSERT INTO syntelle_billing.ledger(workspace_id,session_id,grant_id,kind,seconds) VALUES($1,$2,$3,'release',$4)`, u.Workspace, u.ID, a.id, unused); err != nil {
					return err
				}
			}
		}
	}
	if delta != 0 {
		return errors.New("usage allocation invariant failed")
	}
	_, err = tx.Exec(ctx, `UPDATE syntelle_billing.usage_sessions SET used_seconds=$2,released_at=CASE WHEN $3 THEN $4 ELSE released_at END WHERE session_id=$1`, u.ID, used, release, now)
	return err
}
func (s *Service) reap(ctx context.Context, tx pgx.Tx, wid string, now time.Time) error {
	rows, err := tx.Query(ctx, `SELECT session_id FROM syntelle_billing.usage_sessions WHERE workspace_id=$1 AND released_at IS NULL AND (lease_expires_at<=$2 OR deadline<=$2)`, wid, now)
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
		u, e := getUsage(ctx, tx, id)
		if e != nil {
			return e
		}
		if e = s.meter(ctx, tx, u, now, true); e != nil {
			return e
		}
	}
	return nil
}

// ReserveIntegrationUsage claims a concurrency slot and credit before a worker starts.
// No browser clock, environment bypass, or provisional payment can grant access.
func (s *Service) ReserveIntegrationUsage(ctx context.Context, wid, integration, id string) (time.Time, error) {
	if s.pool == nil || !s.cfg.configured() {
		return time.Time{}, ErrUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	now, err := s.lockWorkspace(ctx, tx, wid)
	if err != nil {
		return time.Time{}, err
	}
	if err = s.reap(ctx, tx, wid, now); err != nil {
		return time.Time{}, err
	}
	existing, err := getUsage(ctx, tx, id)
	if err == nil {
		if existing.Workspace != wid || existing.Integration != integration || existing.Released != nil {
			return time.Time{}, ErrExpired
		}
		return existing.Deadline, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, err
	}
	var plan, status string
	var end *time.Time
	var review, suspended bool
	err = tx.QueryRow(ctx, `SELECT b.plan,b.status,b.period_end,b.review_required,w.suspended_at IS NOT NULL FROM syntelle_billing.workspaces b JOIN syntelle_accounts.workspaces w ON w.id=b.workspace_id WHERE b.workspace_id=$1`, wid).Scan(&plan, &status, &end, &review, &suspended)
	if err != nil {
		return time.Time{}, err
	}
	p, ok := planByID(plan)
	if !ok || status != "active" || end == nil || !end.After(now) || review || suspended {
		return time.Time{}, ErrQuota
	}
	var concurrent int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM syntelle_billing.usage_sessions WHERE workspace_id=$1 AND released_at IS NULL AND lease_expires_at>$2 AND deadline>$2`, wid, now).Scan(&concurrent)
	if err != nil {
		return time.Time{}, err
	}
	if concurrent >= p.Concurrency {
		return time.Time{}, ErrConcurrency
	}
	var dailyLimit, dailySpent int64
	err = tx.QueryRow(ctx, `SELECT daily_limit_seconds FROM syntelle_accounts.integrations WHERE id=$1 AND workspace_id=$2 AND enabled AND revoked_at IS NULL`, integration, wid).Scan(&dailyLimit)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrQuota
	}
	if err != nil {
		return time.Time{}, err
	}
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN released_at IS NULL THEN reserved_seconds ELSE used_seconds END),0) FROM syntelle_billing.usage_sessions WHERE workspace_id=$1 AND integration_id=$2 AND (reserved_at >= (date_trunc('day',$3::timestamptz AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') OR released_at IS NULL OR released_at >= (date_trunc('day',$3::timestamptz AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'))`, wid, integration, now).Scan(&dailySpent)
	if err != nil {
		return time.Time{}, err
	}
	limit := min(int64(300), dailyLimit-dailySpent)
	if limit < 1 {
		return time.Time{}, ErrQuota
	}
	rows, err := tx.Query(ctx, `SELECT id,available_seconds FROM syntelle_billing.grants WHERE workspace_id=$1 AND NOT frozen AND available_seconds>0 AND (expires_at IS NULL OR expires_at>$2) ORDER BY CASE WHEN kind='topup' THEN 1 ELSE 0 END,expires_at NULLS LAST,id FOR UPDATE`, wid, now)
	if err != nil {
		return time.Time{}, err
	}
	type grant struct{ id, seconds int64 }
	var selected []grant
	var total int64
	for rows.Next() {
		var g grant
		if err = rows.Scan(&g.id, &g.seconds); err != nil {
			rows.Close()
			return time.Time{}, err
		}
		g.seconds = min(g.seconds, limit-total)
		if g.seconds > 0 {
			selected = append(selected, g)
			total += g.seconds
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return time.Time{}, err
	}
	if total < 1 {
		return time.Time{}, ErrQuota
	}
	deadline := now.Add(time.Duration(total) * time.Second)
	if deadline.After(*end) {
		deadline = *end
		total = min(total, int64(math.Floor(end.Sub(now).Seconds())))
	}
	if total < 1 {
		return time.Time{}, ErrQuota
	}
	_, err = tx.Exec(ctx, `INSERT INTO syntelle_billing.usage_sessions(session_id,workspace_id,integration_id,reserved_seconds,reserved_at,deadline,lease_expires_at) VALUES($1,$2,$3,$4,$5::timestamptz,$6::timestamptz,LEAST($6::timestamptz,$5::timestamptz+interval '30 seconds'))`, id, wid, integration, total, now, deadline)
	if err != nil {
		return time.Time{}, err
	}
	remaining := total
	for _, g := range selected {
		n := min(g.seconds, remaining)
		remaining -= n
		if n == 0 {
			continue
		}
		if _, err = tx.Exec(ctx, `UPDATE syntelle_billing.grants SET available_seconds=available_seconds-$2 WHERE id=$1`, g.id, n); err != nil {
			return time.Time{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO syntelle_billing.usage_allocations(session_id,grant_id,reserved_seconds) VALUES($1,$2,$3)`, id, g.id, n); err != nil {
			return time.Time{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO syntelle_billing.ledger(workspace_id,session_id,grant_id,kind,seconds) VALUES($1,$2,$3,'reserve',$4)`, wid, id, g.id, n); err != nil {
			return time.Time{}, err
		}
	}
	return deadline, tx.Commit(ctx)
}
func (s *Service) changeUsage(ctx context.Context, id, action string) (time.Time, error) {
	if s.pool == nil {
		return time.Time{}, ErrUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	var wid string
	err = tx.QueryRow(ctx, `SELECT workspace_id FROM syntelle_billing.usage_sessions WHERE session_id=$1`, id).Scan(&wid)
	if errors.Is(err, pgx.ErrNoRows) && action == "release" {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	now, err := s.lockWorkspace(ctx, tx, wid)
	if err != nil {
		return time.Time{}, err
	}
	u, err := getUsage(ctx, tx, id)
	if err != nil {
		return time.Time{}, err
	}
	if u.Released != nil {
		if action == "release" {
			return u.Deadline, nil
		}
		return time.Time{}, ErrExpired
	}
	expired := !u.Lease.After(now) || !u.Deadline.After(now)
	if action == "release" || expired {
		if err = s.meter(ctx, tx, u, now, true); err != nil {
			return time.Time{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return time.Time{}, err
		}
		if expired && action != "release" {
			return time.Time{}, ErrExpired
		}
		return u.Deadline, nil
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT b.status='active' AND NOT b.review_required AND b.period_end>$2 AND w.suspended_at IS NULL AND EXISTS(SELECT 1 FROM syntelle_accounts.integrations i WHERE i.id=$3 AND i.workspace_id=b.workspace_id AND i.enabled AND i.revoked_at IS NULL) FROM syntelle_billing.workspaces b JOIN syntelle_accounts.workspaces w ON w.id=b.workspace_id WHERE b.workspace_id=$1`, wid, now, u.Integration).Scan(&valid)
	if err != nil {
		return time.Time{}, err
	}
	if !valid {
		if err = s.meter(ctx, tx, u, now, true); err != nil {
			return time.Time{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return time.Time{}, err
		}
		return time.Time{}, ErrQuota
	}
	if action == "start" && u.Started == nil {
		if _, err = tx.Exec(ctx, `UPDATE syntelle_billing.usage_sessions SET started_at=$2 WHERE session_id=$1`, id, now); err != nil {
			return time.Time{}, err
		}
	} else if err = s.meter(ctx, tx, u, now, false); err != nil {
		return time.Time{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE syntelle_billing.usage_sessions SET lease_expires_at=LEAST(deadline,$2::timestamptz+interval '30 seconds') WHERE session_id=$1`, id, now)
	if err != nil {
		return time.Time{}, err
	}
	return u.Deadline, tx.Commit(ctx)
}
func (s *Service) StartIntegrationUsage(ctx context.Context, id string) error {
	_, err := s.changeUsage(ctx, id, "start")
	return err
}
func (s *Service) RenewIntegrationUsage(ctx context.Context, id string) (time.Time, error) {
	return s.changeUsage(ctx, id, "renew")
}
func (s *Service) ReleaseIntegrationUsage(ctx context.Context, id string) error {
	_, err := s.changeUsage(ctx, id, "release")
	return err
}

type IntegrationUsage struct {
	IntegrationID string `json:"integration_id"`
	Seconds       int64  `json:"seconds"`
	Sessions      int    `json:"sessions"`
}
type Usage struct {
	Included      int64              `json:"included_seconds_remaining"`
	Topup         int64              `json:"topup_seconds_remaining"`
	Used          int64              `json:"used_seconds"`
	Reserved      int64              `json:"reserved_seconds"`
	Active        int                `json:"active_sessions"`
	Concurrency   int                `json:"concurrency_limit"`
	MaxSession    int                `json:"max_session_seconds"`
	PeriodStart   *time.Time         `json:"period_start"`
	PeriodEnd     *time.Time         `json:"period_end"`
	ByIntegration []IntegrationUsage `json:"by_integration"`
}

func (s *Service) Usage(ctx context.Context, wid string) (Usage, error) {
	result := Usage{MaxSession: 300, ByIntegration: []IntegrationUsage{}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	now, err := s.lockWorkspace(ctx, tx, wid)
	if err != nil {
		return result, err
	}
	if err = s.reap(ctx, tx, wid, now); err != nil {
		return result, err
	}
	var plan string
	err = tx.QueryRow(ctx, `SELECT plan,period_start,period_end FROM syntelle_billing.workspaces WHERE workspace_id=$1`, wid).Scan(&plan, &result.PeriodStart, &result.PeriodEnd)
	if err != nil {
		return result, err
	}
	p, _ := planByID(plan)
	result.Concurrency = p.Concurrency
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(available_seconds) FILTER(WHERE kind<>'topup'),0),COALESCE(sum(available_seconds) FILTER(WHERE kind='topup'),0) FROM syntelle_billing.grants WHERE workspace_id=$1 AND NOT frozen AND (expires_at IS NULL OR expires_at>$2)`, wid, now).Scan(&result.Included, &result.Topup)
	if err != nil {
		return result, err
	}
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(used_seconds),0),COALESCE(sum(reserved_seconds-used_seconds) FILTER(WHERE released_at IS NULL),0),count(*) FILTER(WHERE released_at IS NULL) FROM syntelle_billing.usage_sessions WHERE workspace_id=$1 AND ($2::timestamptz IS NULL OR reserved_at >= $2)`, wid, result.PeriodStart).Scan(&result.Used, &result.Reserved, &result.Active)
	if err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT integration_id,COALESCE(sum(used_seconds),0),count(*) FROM syntelle_billing.usage_sessions WHERE workspace_id=$1 AND ($2::timestamptz IS NULL OR reserved_at >= $2) GROUP BY integration_id ORDER BY integration_id`, wid, result.PeriodStart)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var row IntegrationUsage
		if err = rows.Scan(&row.IntegrationID, &row.Seconds, &row.Sessions); err != nil {
			rows.Close()
			return result, err
		}
		result.ByIntegration = append(result.ByIntegration, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, fmt.Errorf("read integration usage: %w", err)
	}
	return result, tx.Commit(ctx)
}
