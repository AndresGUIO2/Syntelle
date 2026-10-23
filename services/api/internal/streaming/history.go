package streaming

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/accounts"
)

const historyLifetime = 90 * 24 * time.Hour

type historyItem struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Language  string          `json:"language"`
	UpdatedAt time.Time       `json:"updated_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Settings  json.RawMessage `json:"settings,omitempty"`
}

func (s *Service) history(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !localRequest(r) {
		writeError(w, 403, "Local preview required.")
		return
	}
	p, err := s.resolve(w, r, false)
	if err != nil || p.Kind != "account" {
		writeError(w, 401, "Sign in to view conversations.")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/conversations"), "/")
	if path == "" && r.Method == http.MethodGet {
		s.listHistory(w, r, p)
		return
	}
	if path == "claimable" && r.Method == http.MethodGet {
		s.claimableHistory(w, r, p)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 0 || len(parts) > 2 || !identifier.MatchString(parts[0]) {
		writeError(w, 404, "Conversation not found.")
		return
	}
	if len(parts) == 2 && parts[1] == "claim" && r.Method == http.MethodPost {
		s.claimHistory(w, r, p, parts[0])
		return
	}
	if len(parts) != 1 {
		writeError(w, 404, "Conversation not found.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getHistory(w, r, p, parts[0])
	case http.MethodDelete:
		s.deleteHistory(w, r, p, parts[0])
	default:
		writeError(w, 405, "Unsupported conversation action.")
	}
}

func (s *Service) listHistory(w http.ResponseWriter, r *http.Request, p accounts.Principal) {
	before, beforeID := time.Now().Add(time.Hour), "~"
	if cursor := r.URL.Query().Get("before"); cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		parts := strings.SplitN(string(decoded), "|", 2)
		if err != nil || len(parts) != 2 || !identifier.MatchString(parts[1]) {
			writeError(w, 400, "Invalid history cursor.")
			return
		}
		before, err = time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			writeError(w, 400, "Invalid history cursor.")
			return
		}
		beforeID = parts[1]
	}
	rows, err := s.store.pool.Query(r.Context(), `SELECT h.conversation_id,left(coalesce((SELECT t.messages->>'user' FROM syntelle_conversation.turns t WHERE t.conversation_id=h.conversation_id ORDER BY t.sequence LIMIT 1),'Conversation'),80),coalesce(c.language,'en-US'),h.last_activity_at FROM syntelle_streaming.saved_conversations h JOIN syntelle_conversation.sessions c ON c.id=h.conversation_id WHERE h.account_id=$1 AND h.last_activity_at>$2 AND (h.last_activity_at,h.conversation_id)<($3,$4) ORDER BY h.last_activity_at DESC,h.conversation_id DESC LIMIT 21`, p.AccountID, time.Now().Add(-historyLifetime), before, beforeID)
	if err != nil {
		writeError(w, 503, "Conversation history is unavailable.")
		return
	}
	defer rows.Close()
	items := []historyItem{}
	for rows.Next() {
		var x historyItem
		if err = rows.Scan(&x.ID, &x.Title, &x.Language, &x.UpdatedAt); err != nil {
			break
		}
		x.ExpiresAt = x.UpdatedAt.Add(historyLifetime)
		items = append(items, x)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeError(w, 503, "Conversation history is unavailable.")
		return
	}
	next := ""
	if len(items) > 20 {
		items = items[:20]
		next = base64.RawURLEncoding.EncodeToString([]byte(items[19].UpdatedAt.Format(time.RFC3339Nano) + "|" + items[19].ID))
	}
	writeJSON(w, 200, map[string]any{"items": items, "next": next})
}

func (s *Service) getHistory(w http.ResponseWriter, r *http.Request, p accounts.Principal, id string) {
	var item historyItem
	err := s.store.pool.QueryRow(r.Context(), `SELECT h.conversation_id,left(coalesce((SELECT t.messages->>'user' FROM syntelle_conversation.turns t WHERE t.conversation_id=h.conversation_id ORDER BY t.sequence LIMIT 1),'Conversation'),80),c.language,h.last_activity_at,(SELECT settings FROM syntelle_streaming.sessions WHERE conversation_id=h.conversation_id ORDER BY created_at DESC LIMIT 1) FROM syntelle_streaming.saved_conversations h JOIN syntelle_conversation.sessions c ON c.id=h.conversation_id WHERE h.conversation_id=$1 AND h.account_id=$2 AND h.last_activity_at>$3`, id, p.AccountID, time.Now().Add(-historyLifetime)).Scan(&item.ID, &item.Title, &item.Language, &item.UpdatedAt, &item.Settings)
	if err != nil {
		writeError(w, 404, "Conversation not found.")
		return
	}
	item.ExpiresAt = item.UpdatedAt.Add(historyLifetime)
	before, parseErr := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if r.URL.Query().Has("before") && (parseErr != nil || before <= 0) {
		writeError(w, 400, "Invalid transcript cursor.")
		return
	}
	if before <= 0 {
		before = 1 << 62
	}
	rows, err := s.store.pool.Query(r.Context(), `SELECT id,sequence,status,coalesce(messages->>'user',''),coalesce(model_output->>'text',''),coalesce(spoken->>'text','') FROM syntelle_conversation.turns WHERE conversation_id=$1 AND sequence<$2 ORDER BY sequence DESC LIMIT 100`, id, before)
	if err != nil {
		writeError(w, 503, "Conversation transcript is unavailable.")
		return
	}
	defer rows.Close()
	type turn struct {
		ID       string `json:"id"`
		Sequence int64  `json:"sequence"`
		Status   string `json:"status"`
		User     string `json:"user"`
		Reply    string `json:"reply"`
		Spoken   string `json:"spoken"`
	}
	turns := []turn{}
	for rows.Next() {
		var t turn
		if err = rows.Scan(&t.ID, &t.Sequence, &t.Status, &t.User, &t.Reply, &t.Spoken); err != nil {
			break
		}
		turns = append(turns, t)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeError(w, 503, "Conversation transcript is unavailable.")
		return
	}
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
	next := int64(0)
	if len(turns) == 100 {
		next = turns[0].Sequence
	}
	writeJSON(w, 200, map[string]any{"conversation": item, "turns": turns, "before": next})
}

func (s *Service) claimableHistory(w http.ResponseWriter, r *http.Request, p accounts.Principal) {
	rows, err := s.store.pool.Query(r.Context(), `SELECT DISTINCT st.conversation_id,left(coalesce((SELECT t.messages->>'user' FROM syntelle_conversation.turns t WHERE t.conversation_id=st.conversation_id ORDER BY t.sequence LIMIT 1),'Conversation'),80),c.language,max(t.created_at) FROM syntelle_accounts.sessions a JOIN syntelle_accounts.sessions guest ON guest.id=a.guest_session_id JOIN syntelle_streaming.sessions st ON st.principal_id='guest:'||guest.id JOIN syntelle_conversation.sessions c ON c.id=st.conversation_id JOIN syntelle_conversation.turns t ON t.conversation_id=c.id LEFT JOIN syntelle_streaming.saved_conversations h ON h.conversation_id=c.id WHERE a.id=$1 AND guest.expires_at>now() AND h.conversation_id IS NULL GROUP BY st.conversation_id,c.language ORDER BY max(t.created_at) DESC`, p.SessionID)
	if err != nil {
		writeError(w, 503, "Guest conversations are unavailable.")
		return
	}
	defer rows.Close()
	items := []historyItem{}
	for rows.Next() {
		var x historyItem
		if err = rows.Scan(&x.ID, &x.Title, &x.Language, &x.UpdatedAt); err != nil {
			break
		}
		items = append(items, x)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeError(w, 503, "Guest conversations are unavailable.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Service) claimHistory(w http.ResponseWriter, r *http.Request, p accounts.Principal, id string) {
	result, err := s.store.pool.Exec(r.Context(), `INSERT INTO syntelle_streaming.saved_conversations(conversation_id,account_id,last_activity_at) SELECT st.conversation_id,$2,max(t.created_at) FROM syntelle_accounts.sessions a JOIN syntelle_accounts.sessions guest ON guest.id=a.guest_session_id JOIN syntelle_streaming.sessions st ON st.principal_id='guest:'||guest.id JOIN syntelle_conversation.turns t ON t.conversation_id=st.conversation_id WHERE a.id=$3 AND guest.expires_at>now() AND st.conversation_id=$1 GROUP BY st.conversation_id ON CONFLICT(conversation_id) DO NOTHING`, id, p.AccountID, p.SessionID)
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "Guest conversation is unavailable.")
		return
	}
	writeJSON(w, 200, map[string]bool{"saved": true})
}

func (s *Service) deleteHistory(w http.ResponseWriter, r *http.Request, p accounts.Principal, id string) {
	s.mu.Lock()
	for _, session := range s.sessions {
		if session.cid == id {
			s.mu.Unlock()
			writeError(w, 409, "Stop this conversation before deleting it.")
			return
		}
	}
	s.mu.Unlock()
	if err := s.store.purgeConversation(r.Context(), id, p.AccountID, false); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 404, "Conversation not found.")
		} else {
			writeError(w, 503, "Conversation could not be deleted.")
		}
		return
	}
	w.WriteHeader(204)
}

func (s *recordStore) saveTurn(ctx context.Context, cid, accountID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO syntelle_streaming.saved_conversations(conversation_id,account_id,last_activity_at) SELECT $1,$2,max(created_at) FROM syntelle_conversation.turns WHERE conversation_id=$1 GROUP BY conversation_id ON CONFLICT(conversation_id) DO UPDATE SET last_activity_at=greatest(syntelle_streaming.saved_conversations.last_activity_at,excluded.last_activity_at) WHERE syntelle_streaming.saved_conversations.account_id=excluded.account_id`, cid, accountID)
	return err
}

func (s *recordStore) continuation(ctx context.Context, id, accountID string) (json.RawMessage, error) {
	var settings json.RawMessage
	err := s.pool.QueryRow(ctx, `SELECT st.settings FROM syntelle_streaming.saved_conversations h JOIN syntelle_streaming.sessions st ON st.conversation_id=h.conversation_id WHERE h.conversation_id=$1 AND h.account_id=$2 AND h.last_activity_at>$3 ORDER BY st.created_at DESC LIMIT 1`, id, accountID, time.Now().Add(-historyLifetime)).Scan(&settings)
	return settings, err
}

func (s *recordStore) purgeConversation(ctx context.Context, id, accountID string, guest bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if guest {
		err = tx.QueryRow(ctx, `SELECT conversation_id FROM syntelle_streaming.sessions WHERE conversation_id=$1 AND principal_kind='guest' AND closed_at IS NOT NULL FOR UPDATE`, id).Scan(new(string))
	} else {
		err = tx.QueryRow(ctx, `SELECT conversation_id FROM syntelle_streaming.saved_conversations WHERE conversation_id=$1 AND account_id=$2 FOR UPDATE`, id, accountID).Scan(new(string))
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM syntelle_conversation.recognition_results WHERE operation_id IN (SELECT id FROM syntelle_conversation.operations WHERE conversation_id=$1)`, id)
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM syntelle_conversation.operations WHERE conversation_id=$1`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM syntelle_conversation.turns WHERE conversation_id=$1`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM syntelle_conversation.sessions WHERE id=$1`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM syntelle_streaming.events WHERE session_id IN (SELECT id FROM syntelle_streaming.sessions WHERE conversation_id=$1)`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM syntelle_streaming.commands WHERE session_id IN (SELECT id FROM syntelle_streaming.sessions WHERE conversation_id=$1)`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE syntelle_streaming.sessions SET settings='{}'::jsonb,conversation_id='' WHERE conversation_id=$1 AND closed_at IS NOT NULL`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM syntelle_streaming.saved_conversations WHERE conversation_id=$1`, id)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) purgeOldHistory(ctx context.Context) {
	rows, err := s.store.pool.Query(ctx, `SELECT conversation_id,account_id FROM syntelle_streaming.saved_conversations WHERE last_activity_at<=now()-interval '90 days'`)
	if err != nil {
		return
	}
	type old struct{ id, account string }
	pending := []old{}
	for rows.Next() {
		var x old
		if rows.Scan(&x.id, &x.account) == nil {
			pending = append(pending, x)
		}
	}
	rows.Close()
	for _, x := range pending {
		s.mu.Lock()
		active := false
		for _, session := range s.sessions {
			if session.cid == x.id {
				active = true
				break
			}
		}
		s.mu.Unlock()
		if !active {
			_ = s.store.purgeConversation(ctx, x.id, x.account, false)
		}
	}
	guestRows, err := s.store.pool.Query(ctx, `SELECT DISTINCT st.conversation_id FROM syntelle_streaming.sessions st JOIN syntelle_accounts.sessions a ON st.principal_id='guest:'||a.id LEFT JOIN syntelle_streaming.saved_conversations h ON h.conversation_id=st.conversation_id WHERE st.principal_kind='guest' AND st.conversation_id<>'' AND st.closed_at IS NOT NULL AND a.expires_at<=now() AND h.conversation_id IS NULL`)
	if err != nil {
		return
	}
	guests := []string{}
	for guestRows.Next() {
		var id string
		if guestRows.Scan(&id) == nil {
			guests = append(guests, id)
		}
	}
	guestRows.Close()
	for _, id := range guests {
		s.mu.Lock()
		active := false
		for _, session := range s.sessions {
			if session.cid == id {
				active = true
				break
			}
		}
		s.mu.Unlock()
		if !active {
			_ = s.store.purgeConversation(ctx, id, "", true)
		}
	}
}
