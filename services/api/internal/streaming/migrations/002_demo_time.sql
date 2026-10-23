-- Historical streams are retained without inventing pre-policy usage.
ALTER TABLE syntelle_streaming.sessions
 ADD COLUMN used_ms bigint NOT NULL DEFAULT 0 CHECK (used_ms BETWEEN 0 AND 300000),
 ADD COLUMN lease_ms bigint NOT NULL DEFAULT 0 CHECK (lease_ms BETWEEN 0 AND 300000),
 ADD COLUMN lease_started_at timestamptz,
 ADD CONSTRAINT streaming_session_time_bound CHECK (used_ms + lease_ms <= 300000);
