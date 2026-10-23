CREATE SCHEMA IF NOT EXISTS syntelle_conversation;
CREATE TABLE IF NOT EXISTS syntelle_conversation.migrations (
 version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS syntelle_conversation.sessions (
 id text PRIMARY KEY, character text NOT NULL, language text NOT NULL,
 instructions jsonb NOT NULL, status text NOT NULL DEFAULT 'open',
 next_sequence bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS syntelle_conversation.turns (
 id text PRIMARY KEY, conversation_id text NOT NULL REFERENCES syntelle_conversation.sessions(id),
 sequence bigint NOT NULL, status text NOT NULL,
 messages jsonb NOT NULL, model_output jsonb NOT NULL DEFAULT '{}',
 spoken jsonb NOT NULL DEFAULT '{}', interruption jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 UNIQUE(conversation_id,sequence)
);
CREATE TABLE IF NOT EXISTS syntelle_conversation.operations (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 conversation_id text REFERENCES syntelle_conversation.sessions(id),
 turn_id text REFERENCES syntelle_conversation.turns(id),
 kind text NOT NULL, request_key text NOT NULL, binding text NOT NULL DEFAULT '',
 status text NOT NULL CHECK(status IN ('started','received','cancelled','failed','uncertain')),
 request jsonb NOT NULL, result jsonb NOT NULL DEFAULT '{}',
 usage jsonb NOT NULL DEFAULT '{}', diagnostics jsonb NOT NULL DEFAULT '{}', audio bytea,
 created_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 UNIQUE(kind,request_key)
);
CREATE INDEX IF NOT EXISTS operations_cache ON syntelle_conversation.operations(kind,binding) WHERE status='received';
CREATE TABLE IF NOT EXISTS syntelle_conversation.recognition_results (
 operation_id bigint NOT NULL REFERENCES syntelle_conversation.operations(id),
 result_id text NOT NULL, result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(operation_id,result_id)
);
