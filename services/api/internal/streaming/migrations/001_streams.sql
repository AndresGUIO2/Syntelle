CREATE TABLE syntelle_streaming.sessions (
 id text PRIMARY KEY,
 conversation_id text NOT NULL UNIQUE,
 principal_id text NOT NULL,
 principal_kind text NOT NULL,
 browser_session_id text NOT NULL DEFAULT '',
 key_id text NOT NULL DEFAULT '',
 settings jsonb NOT NULL,
 package_hash text NOT NULL,
 state text NOT NULL,
 reason text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 closed_at timestamptz
);
CREATE INDEX streaming_sessions_owner ON syntelle_streaming.sessions(principal_id,created_at DESC);
CREATE TABLE syntelle_streaming.commands (
 session_id text NOT NULL REFERENCES syntelle_streaming.sessions(id),
 id text NOT NULL,
 turn_id text NOT NULL,
 kind text NOT NULL,
 request jsonb NOT NULL,
 status text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 PRIMARY KEY(session_id,id)
);
CREATE TABLE syntelle_streaming.events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 session_id text NOT NULL REFERENCES syntelle_streaming.sessions(id),
 kind text NOT NULL,
 data jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
