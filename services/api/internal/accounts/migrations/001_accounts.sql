CREATE SCHEMA IF NOT EXISTS syntelle_accounts;
CREATE TABLE IF NOT EXISTS syntelle_accounts.migrations (
 version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS syntelle_accounts.accounts (
 id text PRIMARY KEY, email text NOT NULL UNIQUE, display_name text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), disabled_at timestamptz
);
CREATE TABLE IF NOT EXISTS syntelle_accounts.identities (
 provider text NOT NULL, subject text NOT NULL,
 account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(provider,subject)
);
CREATE TABLE IF NOT EXISTS syntelle_accounts.sessions (
 id text PRIMARY KEY, token_hash text NOT NULL UNIQUE,
 account_id text REFERENCES syntelle_accounts.accounts(id),
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
 revoked_at timestamptz
);
CREATE INDEX IF NOT EXISTS sessions_expiry ON syntelle_accounts.sessions(expires_at);
CREATE TABLE IF NOT EXISTS syntelle_accounts.configurations (
 id text PRIMARY KEY, account_id text REFERENCES syntelle_accounts.accounts(id),
 guest_session_id text REFERENCES syntelle_accounts.sessions(id), name text NOT NULL,
 package_hash text NOT NULL, settings jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz,
 CHECK ((account_id IS NOT NULL AND guest_session_id IS NULL AND expires_at IS NULL)
 OR (account_id IS NULL AND guest_session_id IS NOT NULL AND expires_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS configurations_owner ON syntelle_accounts.configurations(account_id);
CREATE INDEX IF NOT EXISTS configurations_guest ON syntelle_accounts.configurations(guest_session_id);
CREATE TABLE IF NOT EXISTS syntelle_accounts.api_keys (
 id text PRIMARY KEY, account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 name text NOT NULL, secret_hash text NOT NULL, scope text NOT NULL DEFAULT 'streams',
 created_at timestamptz NOT NULL DEFAULT now(), last_used_at timestamptz, revoked_at timestamptz
);
CREATE INDEX IF NOT EXISTS keys_owner ON syntelle_accounts.api_keys(account_id);
CREATE TABLE IF NOT EXISTS syntelle_accounts.oauth_flows (
 state_hash text PRIMARY KEY, session_id text NOT NULL REFERENCES syntelle_accounts.sessions(id),
 provider text NOT NULL, payload bytea NOT NULL, return_to text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL, consumed_at timestamptz
);
CREATE TABLE IF NOT EXISTS syntelle_accounts.pending_identities (
 id text PRIMARY KEY, session_id text NOT NULL REFERENCES syntelle_accounts.sessions(id),
 provider text NOT NULL, subject text NOT NULL, display_name text NOT NULL DEFAULT '',
 required_email text NOT NULL DEFAULT '', target_account_id text REFERENCES syntelle_accounts.accounts(id),
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL, consumed_at timestamptz
);
CREATE TABLE IF NOT EXISTS syntelle_accounts.email_challenges (
 id text PRIMARY KEY, session_id text NOT NULL REFERENCES syntelle_accounts.sessions(id),
 email text NOT NULL, verifier text NOT NULL, request_key text NOT NULL,
 pending_identity_id text REFERENCES syntelle_accounts.pending_identities(id),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 delivery_status text NOT NULL DEFAULT 'started', delivery_id text NOT NULL DEFAULT '',
 ip_hash text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
 consumed_at timestamptz, UNIQUE(session_id,request_key)
);
CREATE INDEX IF NOT EXISTS challenges_address ON syntelle_accounts.email_challenges(email,created_at);
CREATE INDEX IF NOT EXISTS challenges_ip ON syntelle_accounts.email_challenges(ip_hash,created_at);
CREATE TABLE IF NOT EXISTS syntelle_accounts.audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, account_id text,
 event text NOT NULL, resource_id text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
