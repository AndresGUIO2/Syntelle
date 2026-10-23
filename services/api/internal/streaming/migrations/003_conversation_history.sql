ALTER TABLE syntelle_streaming.sessions DROP CONSTRAINT IF EXISTS sessions_conversation_id_key;
CREATE INDEX IF NOT EXISTS streaming_sessions_conversation ON syntelle_streaming.sessions(conversation_id,created_at DESC);
CREATE TABLE IF NOT EXISTS syntelle_streaming.saved_conversations (
 conversation_id text PRIMARY KEY,
 account_id text NOT NULL,
 last_activity_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS saved_conversations_account ON syntelle_streaming.saved_conversations(account_id,last_activity_at DESC,conversation_id DESC);
DO $$ BEGIN
 IF to_regclass('syntelle_accounts.accounts') IS NOT NULL THEN
  INSERT INTO syntelle_streaming.saved_conversations(conversation_id,account_id,last_activity_at)
  SELECT s.conversation_id,a.id,max(t.created_at)
  FROM syntelle_streaming.sessions s
  JOIN syntelle_accounts.accounts a ON s.principal_id='account:'||a.id
  JOIN syntelle_conversation.turns t ON t.conversation_id=s.conversation_id
  GROUP BY s.conversation_id,a.id
  ON CONFLICT(conversation_id) DO NOTHING;
 END IF;
END $$;
