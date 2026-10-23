package speech

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"strings"
	"syntelle/api/internal/conversation"
	"testing"
	"time"
)

func TestLocalOwnerSpeechDoesNotMutateHistoricalAllowances(t *testing.T) {
	dsn := os.Getenv("SYNTELLE_CONVERSATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Set isolated PostgreSQL test configuration")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal("Cannot connect to test database")
	}
	defer admin.Close(ctx)
	name := fmt.Sprintf("syntelle_owner_test_%d", time.Now().UnixNano())
	if !strings.HasPrefix(name, "syntelle_owner_test_") {
		t.Fatal("Invalid test name")
	}
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+quoted); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)")
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	u.Path = "/" + name
	q := u.Query()
	q.Del("dbname")
	u.RawQuery = q.Encode()
	verify, e := pgx.Connect(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	var actual string
	e = verify.QueryRow(ctx, "SELECT current_database()").Scan(&actual)
	verify.Close(ctx)
	if e != nil || actual != name {
		t.Fatal("Refusing mutations outside isolated owner test database")
	}
	legacy, e := OpenTrialLedger(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer legacy.pool.Close()
	store, e := conversation.Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Pool.Close()
	owner := &localOwnerSpeechLedger{store}
	provider := &fakeSpeechProvider{}
	service := &Service{provider, owner, "canadacentral"}
	for i := 0; i < 22; i++ {
		input := validSpeechInput()
		input.Text = strings.Repeat("x", 499) + fmt.Sprint(i%10)
		input.RequestKey = fmt.Sprintf("owner-test-%08d", i)
		input.Text = fmt.Sprintf("%03d", i) + input.Text[3:]
		if _, e = service.synthesize(ctx, input); e != nil {
			t.Fatal(e)
		}
	}
	if provider.calls != 22 {
		t.Fatal("Owner requests were capped")
	}
	var count, maxRequests, maxCharacters int
	_ = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_speech.requests`).Scan(&count)
	_ = store.Pool.QueryRow(ctx, `SELECT max_requests,max_characters FROM syntelle_speech.budgets WHERE id=$1`, speechBudget).Scan(&maxRequests, &maxCharacters)
	if count != 0 || maxRequests != 20 || maxCharacters != 5000 {
		t.Fatal("Historical allowance changed")
	}
	input := validSpeechInput()
	input.Text = "Historical real-cache fixture."
	id, _, e := legacy.Reserve(ctx, input, "historical-cache", 30)
	if e != nil {
		t.Fatal(e)
	}
	result := Result{Audio: make([]byte, 44), DurationMS: 1000, Visemes: []Viseme{{0, 0}, {2, 100}}}
	if e = legacy.Finish(ctx, id, &result, "", false); e != nil {
		t.Fatal(e)
	}
	_, cached, e := owner.Reserve(ctx, input, "historical-cache", 30)
	if e != nil || cached == nil || len(cached.Audio) != 44 {
		t.Fatal("Historical speech was not reused", e)
	}
	// Long conversation preparation uses the same owner ledger, preserving each
	// native result and a separately identified, derived sequence.
	longProvider := &sequenceProviderFixture{}
	longService := NewService(longProvider, owner, "canadacentral")
	text := strings.Repeat("An explanation with useful detail. ", 210)
	prepared, e := NewConversationSpeech(longService).Prepare(ctx, text, "en-US", "en-US-Ava:DragonHDLatestNeural", "owner-long-test-01")
	if e != nil {
		t.Fatal(e)
	}
	joined := prepared.(Result)
	if len(joined.Parts) < 2 || strings.Join(longProvider.texts, "") != text {
		t.Fatal("long reply lost content")
	}
	var sequenceAudio []byte
	if e = store.Pool.QueryRow(ctx, `SELECT audio FROM syntelle_conversation.operations WHERE kind='dragon-sequence' AND request_key='chat-owner-long-test-01'`).Scan(&sequenceAudio); e != nil || string(sequenceAudio) != string(joined.Audio) {
		t.Fatal("derived sequence evidence missing", e)
	}
	calls := len(longProvider.texts)
	if _, e = NewConversationSpeech(longService).Prepare(ctx, text, "en-US", "en-US-Ava:DragonHDLatestNeural", "owner-long-test-01"); e != nil || len(longProvider.texts) != calls {
		t.Fatal("cached parts repeated provider work", e)
	}
	if e = store.Pool.QueryRow(ctx, `SELECT count(*) FROM syntelle_speech.requests`).Scan(&count); e != nil || count != 1 {
		t.Fatal("long speech altered historical ledger", e, count)
	}
}
