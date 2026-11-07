// bootstrap-staff grants the first Syntelle platform administrator to a
// verified existing account. It is a deliberate operator command, never a
// signup side effect or public API.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	accountID := strings.TrimSpace(os.Getenv("SYNTELLE_BOOTSTRAP_ACCOUNT_ID"))
	operator := strings.TrimSpace(os.Getenv("SYNTELLE_BOOTSTRAP_OPERATOR"))
	reason := strings.TrimSpace(os.Getenv("SYNTELLE_BOOTSTRAP_REASON"))
	dsn := os.Getenv("SYNTELLE_DATABASE_URL")
	if accountID == "" || operator == "" || len(reason) < 3 || dsn == "" {
		log.Fatal("set account ID, operator, reason and database URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("database unavailable")
	}
	defer pool.Close()
	if err = bootstrap(ctx, pool, accountID, operator, reason); err != nil {
		log.Fatal(err)
	}
	log.Print("first Syntelle administrator granted and audited")
}

func bootstrap(ctx context.Context, pool *pgxpool.Pool, accountID, operator, reason string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(187200061)`); err != nil {
		return err
	}
	var anyStaff, verified bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.staff_grants)`).Scan(&anyStaff); err != nil {
		return err
	}
	if anyStaff {
		return errors.New("staff has already been bootstrapped")
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.accounts a WHERE a.id=$1 AND a.disabled_at IS NULL)`, accountID).Scan(&verified); err != nil {
		return err
	}
	if !verified {
		return errors.New("target is not an existing verified account")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO syntelle_accounts.staff_grants(account_id,role) VALUES($1,'administrator')`, accountID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO syntelle_accounts.workspace_audit(action,target_id,reason,after_state) VALUES('staff.bootstrapped',$1,$2,jsonb_build_object('role','administrator','operator',$3::text))`, accountID, reason, operator); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
