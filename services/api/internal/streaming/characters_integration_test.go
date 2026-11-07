//go:build integration

package streaming

import (
	"context"
	"strings"
	"syntelle/api/internal/accounts"
	"testing"
)

func TestCharacterDeletionStopsOnlyItsOwnedSessions(t *testing.T) {
	db := streamingTestStore(t)
	ctx := context.Background()
	store, err := openStore(ctx, db.Pool)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{ctx: ctx, store: store, sessions: map[string]*streamSession{}}
	character := "char_" + strings.Repeat("1", 32)
	for _, row := range []struct{ id, account, character string }{{"stream_deleted", "owner", character}, {"stream_other_owner", "other", character}, {"stream_default", "owner", "syntelle"}} {
		_, cancel := context.WithCancel(ctx)
		defer cancel()
		settings := accounts.Defaults()
		settings.Character = row.character
		session := &streamSession{id: row.id, cid: "conversation_" + row.id, principal: accounts.Principal{ID: "account:" + row.account, Kind: "account", AccountID: row.account}, settings: settings, cancel: cancel}
		if err := store.allocateUnmetered(ctx, session, strings.Repeat("a", 64), false); err != nil {
			t.Fatal(err)
		}
		service.sessions[row.id] = session
	}
	service.CharacterDeleted("owner", character)
	if len(service.sessions) != 2 || service.sessions["stream_deleted"] != nil || service.sessions["stream_other_owner"] == nil || service.sessions["stream_default"] == nil {
		t.Fatal("deletion crossed session ownership")
	}
	var reason string
	if err := db.Pool.QueryRow(ctx, `SELECT reason FROM syntelle_streaming.sessions WHERE id='stream_deleted' AND closed_at IS NOT NULL`).Scan(&reason); err != nil || reason != "character deleted" {
		t.Fatal(reason, err)
	}
	service.CharacterDeleted("owner", character)
	if len(service.sessions) != 2 {
		t.Fatal("duplicate deletion changed another session")
	}
}
