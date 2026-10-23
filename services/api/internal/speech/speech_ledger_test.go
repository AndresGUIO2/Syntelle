package speech

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestSpeechPostgresReservationRecoveryAndLimits(t *testing.T) {
	dsn := os.Getenv("SYNTELLE_SPEECH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Set test database connection to run isolated PostgreSQL integration")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("Cannot connect to local test database")
	}
	defer admin.Close(ctx)
	name := fmt.Sprintf("syntelle_speech_test_%d", time.Now().UnixNano())
	if !strings.HasPrefix(name, "syntelle_speech_test_") {
		t.Fatal("Invalid test database name")
	}
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)")
	// pgx.ConnConfig.ConnString returns the original DSN, ignoring a later
	// Database field change. Construct a distinct URL and verify the server.
	testURL, err := url.Parse(dsn)
	if err != nil || (testURL.Scheme != "postgres" && testURL.Scheme != "postgresql") {
		t.Fatal("The integration test requires a PostgreSQL URL")
	}
	testURL.Path = "/" + name
	query := testURL.Query()
	query.Del("dbname")
	testURL.RawQuery = query.Encode()
	ledger, err := OpenTrialLedger(ctx, testURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.pool.Close()
	var actualDatabase string
	if err = ledger.pool.QueryRow(ctx, "SELECT current_database()").Scan(&actualDatabase); err != nil || actualDatabase != name {
		t.Fatal("Refusing integration mutations outside the isolated test database")
	}
	input := validSpeechInput()
	id, cache, err := ledger.Reserve(ctx, input, "binding-1", 6)
	if err != nil || cache != nil {
		t.Fatal("Reservation failed", err)
	}
	if _, _, err = ledger.Reserve(ctx, input, "binding-1", 6); err == nil {
		t.Fatal("Started attempt was retried")
	}
	result := Result{Audio: []byte{1, 2, 3}, DurationMS: 1000, Visemes: []Viseme{{2, 0}}}
	if err = ledger.Finish(ctx, id, &result, "", false); err != nil {
		t.Fatal(err)
	}
	// A continuation must replay the earlier allowance's audio without charging
	// another request or changing that allowance's original limits.
	if _, err = ledger.pool.Exec(ctx, "UPDATE syntelle_speech.requests SET budget_id=$2 WHERE id=$1", id, previousSpeechBudget); err != nil {
		t.Fatal(err)
	}
	_, cache, err = ledger.Reserve(ctx, input, "binding-1", 6)
	if err != nil || cache == nil || string(cache.Audio) != string(result.Audio) {
		t.Fatal("Actual cached audio bytes not restored")
	}
	if _, _, err = ledger.Reserve(ctx, input, "changed", 6); err == nil {
		t.Fatal("Idempotency binding changed")
	}
	// Two concurrent new requests must not both reserve an active synthesis.
	var wg sync.WaitGroup
	var mutex sync.Mutex
	accepted := []int64{}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			request := input
			request.RequestKey = fmt.Sprintf("parallel-%d", i)
			id, _, err := ledger.Reserve(ctx, request, fmt.Sprintf("parallel-%d", i), 10)
			if err == nil {
				mutex.Lock()
				accepted = append(accepted, id)
				mutex.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if len(accepted) != 1 {
		t.Fatal("Concurrent provider reservations", len(accepted))
	}
	if err = ledger.Finish(ctx, accepted[0], &result, "Provider failure", true); err != nil {
		t.Fatal(err)
	}
	input.RequestKey = "blocked-request"
	if _, _, err = ledger.Reserve(ctx, input, "blocked", 1); err == nil {
		t.Fatal("Provider failure did not halt trial")
	}
	// A separate isolated budget state checks the full persisted character/request cap.
	if _, err = ledger.pool.Exec(ctx, "UPDATE syntelle_speech.budgets SET halted=false; DELETE FROM syntelle_speech.requests"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < speechRequestLimit; i++ {
		input.RequestKey = fmt.Sprintf("bounded-%d", i)
		id, _, err = ledger.Reserve(ctx, input, input.RequestKey, speechCharacterLimit/speechRequestLimit)
		if err != nil {
			t.Fatal(err)
		}
		if err = ledger.Finish(ctx, id, &result, "", false); err != nil {
			t.Fatal(err)
		}
	}
	input.RequestKey = "beyond-budget"
	if _, _, err = ledger.Reserve(ctx, input, "beyond", 1); err == nil {
		t.Fatal("Persisted budget exceeded")
	}
}
