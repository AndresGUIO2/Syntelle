CREATE TABLE syntelle_accounts.character_creations (
 id text PRIMARY KEY CHECK (id ~ '^char_[a-f0-9]{32}$'),
 account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id) ON DELETE CASCADE,
 request_key text NOT NULL,
 name text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
 source_sha256 text NOT NULL CHECK (source_sha256 ~ '^[a-f0-9]{64}$'),
 source_bytes bytea CHECK(octet_length(source_bytes) BETWEEN 1 AND 5000000),
 mime_type text NOT NULL CHECK (mime_type IN ('image/jpeg','image/png','image/webp')),
 width integer NOT NULL CHECK(width > 0),
 height integer NOT NULL CHECK(height > 0),
 status text NOT NULL CHECK(status IN ('queued','processing','ready','failed')),
 package_hash text CHECK(package_hash ~ '^[a-f0-9]{64}$'),
 failure_code text CHECK(failure_code ~ '^[a-z0-9_]{1,64}$'),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 ready_at timestamptz, expires_at timestamptz NOT NULL, deleted_at timestamptz,
 UNIQUE(account_id,request_key),
 CHECK(status <> 'ready' OR (package_hash IS NOT NULL AND ready_at IS NOT NULL))
);
CREATE INDEX character_creations_owner ON syntelle_accounts.character_creations(account_id,created_at DESC);
CREATE UNIQUE INDEX character_creations_one_active ON syntelle_accounts.character_creations(account_id)
 WHERE deleted_at IS NULL AND status IN ('queued','processing');
CREATE INDEX character_creations_queue ON syntelle_accounts.character_creations(status,created_at)
 WHERE deleted_at IS NULL AND status IN ('queued','processing');
CREATE INDEX character_creations_expiry ON syntelle_accounts.character_creations(expires_at) WHERE source_bytes IS NOT NULL;
