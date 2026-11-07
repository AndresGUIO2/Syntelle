CREATE SCHEMA IF NOT EXISTS syntelle_billing;
CREATE TABLE syntelle_billing.workspaces (
 workspace_id text PRIMARY KEY REFERENCES syntelle_accounts.workspaces(id),
 subscription_id text UNIQUE, customer_id text,
 plan text NOT NULL DEFAULT '', status text NOT NULL DEFAULT 'none',
 period_start timestamptz, period_end timestamptz,
 pending_plan text NOT NULL DEFAULT '', cancel_at_period_end boolean NOT NULL DEFAULT false,
 subscription_event_at timestamptz NOT NULL DEFAULT '1970-01-01T00:00:00Z',
 review_required boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE syntelle_billing.actions (
 id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 actor_id text NOT NULL, idempotency_key text NOT NULL, kind text NOT NULL,
 input_hash text NOT NULL, request jsonb NOT NULL DEFAULT '{}', plan text NOT NULL DEFAULT '',
 state text NOT NULL DEFAULT 'reserved' CHECK (state IN ('reserved','completed','failed','unknown')),
 provider_id text, response jsonb, failure_code text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), completed_at timestamptz,
 UNIQUE(workspace_id,idempotency_key)
);
CREATE TABLE syntelle_billing.provider_requests (
 id text PRIMARY KEY, action_id text NOT NULL REFERENCES syntelle_billing.actions(id),
 method text NOT NULL, path text NOT NULL, request bytea NOT NULL,
 response bytea, status integer, outcome text NOT NULL DEFAULT 'reserved',
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), completed_at timestamptz
);
CREATE TABLE syntelle_billing.webhook_inbox (
 event_id text PRIMARY KEY, event_type text NOT NULL, occurred_at timestamptz NOT NULL,
 payload bytea NOT NULL, payload_hash text NOT NULL,
 source text NOT NULL DEFAULT 'signed_webhook',
 state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','processed','rejected')),
 reason text NOT NULL DEFAULT '', received_at timestamptz NOT NULL DEFAULT clock_timestamp(), attempted_at timestamptz, processed_at timestamptz
);
CREATE INDEX billing_pending_events ON syntelle_billing.webhook_inbox(occurred_at) WHERE state='pending';
CREATE TABLE syntelle_billing.fulfillments (
 transaction_id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 event_id text NOT NULL REFERENCES syntelle_billing.webhook_inbox(event_id),
 plan text NOT NULL, seconds bigint NOT NULL CHECK(seconds>=0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE syntelle_billing.grants (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 transaction_id text NOT NULL REFERENCES syntelle_billing.fulfillments(transaction_id),
 grant_key text NOT NULL UNIQUE, kind text NOT NULL CHECK(kind IN ('monthly','upgrade','topup')),
 amount_seconds bigint NOT NULL CHECK(amount_seconds>=0),
 available_seconds bigint NOT NULL CHECK(available_seconds>=0),
 expires_at timestamptz, frozen boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK(available_seconds<=amount_seconds)
);
CREATE INDEX billing_workspace_grants ON syntelle_billing.grants(workspace_id,expires_at,id);
CREATE TABLE syntelle_billing.usage_sessions (
 session_id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 integration_id text NOT NULL, reserved_seconds bigint NOT NULL CHECK(reserved_seconds BETWEEN 1 AND 300),
 used_seconds bigint NOT NULL DEFAULT 0 CHECK(used_seconds>=0),
 reserved_at timestamptz NOT NULL, started_at timestamptz,
 deadline timestamptz NOT NULL, lease_expires_at timestamptz NOT NULL,
 released_at timestamptz, CHECK(used_seconds<=reserved_seconds)
);
CREATE INDEX billing_workspace_live_sessions ON syntelle_billing.usage_sessions(workspace_id,lease_expires_at) WHERE released_at IS NULL;
CREATE TABLE syntelle_billing.usage_allocations (
 session_id text NOT NULL REFERENCES syntelle_billing.usage_sessions(session_id),
 grant_id bigint NOT NULL REFERENCES syntelle_billing.grants(id),
 reserved_seconds bigint NOT NULL CHECK(reserved_seconds>0),
 used_seconds bigint NOT NULL DEFAULT 0 CHECK(used_seconds>=0 AND used_seconds<=reserved_seconds),
 PRIMARY KEY(session_id,grant_id)
);
CREATE TABLE syntelle_billing.ledger (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 session_id text, transaction_id text, grant_id bigint REFERENCES syntelle_billing.grants(id),
 kind text NOT NULL CHECK(kind IN ('grant','reserve','consume','release','freeze')),
 seconds bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX billing_ledger_workspace ON syntelle_billing.ledger(workspace_id,created_at);
