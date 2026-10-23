CREATE TABLE IF NOT EXISTS archive_schema (
  version integer PRIMARY KEY,
  installed_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO archive_schema(version) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS blobs (
  sha256 text PRIMARY KEY CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  data bytea NOT NULL,
  byte_length bigint NOT NULL CHECK (byte_length = octet_length(data)),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS experiments (
  id text PRIMARY KEY,
  metadata jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS snapshots (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  experiment_id text NOT NULL REFERENCES experiments(id),
  manifest_sha256 text NOT NULL REFERENCES blobs(sha256),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(experiment_id, manifest_sha256)
);
CREATE TABLE IF NOT EXISTS snapshot_files (
  snapshot_id bigint NOT NULL REFERENCES snapshots(id),
  path text NOT NULL,
  blob_sha256 text NOT NULL REFERENCES blobs(sha256),
  PRIMARY KEY(snapshot_id, path)
);
CREATE TABLE IF NOT EXISTS budgets (
  id text PRIMARY KEY,
  binding text NOT NULL,
  limits jsonb NOT NULL,
  halted boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS requests (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  budget_id text NOT NULL REFERENCES budgets(id),
  request_key text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('image', 'analysis')),
  binding text NOT NULL,
  status text NOT NULL CHECK (status IN ('started','received','failed')),
  usage jsonb NOT NULL DEFAULT '{}',
  metadata jsonb NOT NULL DEFAULT '{}',
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  UNIQUE(budget_id, request_key)
);
CREATE TABLE IF NOT EXISTS events (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  experiment_id text NOT NULL REFERENCES experiments(id),
  kind text NOT NULL,
  record_sha256 text NOT NULL REFERENCES blobs(sha256),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(experiment_id, kind, record_sha256)
);
