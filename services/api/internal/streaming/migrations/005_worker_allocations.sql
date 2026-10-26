-- Each cloud renderer has its own durable launch lease. The earlier singleton
-- table remains for historical local tests and rollout compatibility.
CREATE TABLE IF NOT EXISTS syntelle_streaming.worker_allocations (
 id text PRIMARY KEY,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS worker_allocations_expiry ON syntelle_streaming.worker_allocations(expires_at);
