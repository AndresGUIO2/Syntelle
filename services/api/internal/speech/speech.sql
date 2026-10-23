CREATE SCHEMA IF NOT EXISTS syntelle_speech;
CREATE TABLE IF NOT EXISTS syntelle_speech.budgets (
 id text PRIMARY KEY, max_requests integer NOT NULL, max_characters integer NOT NULL,
 halted boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO syntelle_speech.budgets(id,max_requests,max_characters)
 VALUES ('authorized-dragon-motion-2026-10-19',12,6000) ON CONFLICT DO NOTHING;
-- Separate owner-authorized continuation; preserve the original ledger/limits.
INSERT INTO syntelle_speech.budgets(id,max_requests,max_characters)
 VALUES ('authorized-dragon-continuation-2026-10-20',20,5000) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS syntelle_speech.requests (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 budget_id text NOT NULL REFERENCES syntelle_speech.budgets(id),
 request_key text NOT NULL UNIQUE, binding text NOT NULL UNIQUE,
 characters integer NOT NULL, request jsonb NOT NULL,
 status text NOT NULL CHECK(status IN ('started','received','failed')),
 response jsonb, audio bytea, error text,
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz
);
