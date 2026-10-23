CREATE TABLE IF NOT EXISTS syntelle_streaming.worker_allocation (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 id text NOT NULL,
 expires_at timestamptz NOT NULL
);
