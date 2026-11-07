CREATE TABLE syntelle_accounts.publications (
 id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id) ON DELETE CASCADE,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 100), character_id text NOT NULL,
 package_hash text NOT NULL CHECK(package_hash ~ '^[a-f0-9]{64}$'), settings jsonb NOT NULL,
 portrait_bytes bytea, portrait_mime text, created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz,
 UNIQUE(workspace_id,id)
);
CREATE TABLE syntelle_accounts.integrations (
 id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id) ON DELETE CASCADE,
 widget_id text NOT NULL UNIQUE, name text NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
 draft jsonb NOT NULL, published jsonb, published_name text, revision bigint NOT NULL DEFAULT 1,
 published_revision bigint NOT NULL DEFAULT 0, enabled boolean NOT NULL DEFAULT false,
 created_by text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 revoked_at timestamptz, daily_limit_seconds integer NOT NULL DEFAULT 3600 CHECK(daily_limit_seconds BETWEEN 60 AND 86400), UNIQUE(workspace_id,id)
);
ALTER TABLE syntelle_accounts.workspace_keys ADD COLUMN integration_id text REFERENCES syntelle_accounts.integrations(id);
CREATE TABLE syntelle_accounts.widget_bootstraps (
 id text PRIMARY KEY, integration_id text NOT NULL REFERENCES syntelle_accounts.integrations(id) ON DELETE CASCADE,
 revision bigint NOT NULL, parent_origin text NOT NULL, nonce_hash text NOT NULL,
 expires_at timestamptz NOT NULL, consumed_at timestamptz, request_id text,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX widget_bootstraps_expiry ON syntelle_accounts.widget_bootstraps(expires_at);
CREATE INDEX publications_package_reference ON syntelle_accounts.publications(package_hash);
