ALTER TABLE syntelle_streaming.sessions ADD COLUMN workspace_id text;
ALTER TABLE syntelle_streaming.sessions ADD COLUMN integration_id text;
ALTER TABLE syntelle_streaming.sessions ADD COLUMN publication_id text;
ALTER TABLE syntelle_streaming.sessions ADD COLUMN integration_revision bigint;
CREATE INDEX streaming_workspace_sessions ON syntelle_streaming.sessions(workspace_id,created_at);
