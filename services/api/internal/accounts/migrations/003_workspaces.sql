CREATE TABLE syntelle_accounts.workspaces (
 id text PRIMARY KEY, name text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
 created_by text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 created_at timestamptz NOT NULL DEFAULT now(), suspended_at timestamptz
);
CREATE TABLE syntelle_accounts.memberships (
 workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 role text NOT NULL CHECK (role IN ('owner','administrator','editor','billing')),
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(workspace_id,account_id)
);
CREATE UNIQUE INDEX one_workspace_owner ON syntelle_accounts.memberships(workspace_id) WHERE role='owner';
CREATE INDEX memberships_account ON syntelle_accounts.memberships(account_id);
CREATE TABLE syntelle_accounts.workspace_invitations (
 id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 email text NOT NULL, role text NOT NULL CHECK (role IN ('administrator','editor','billing')),
 token_hash text NOT NULL UNIQUE, invited_by text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
 accepted_at timestamptz, revoked_at timestamptz
);
CREATE TABLE syntelle_accounts.ownership_transfers (
 id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 from_account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 to_account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL,
 accepted_at timestamptz, revoked_at timestamptz
);
CREATE TABLE syntelle_accounts.workspace_configurations (
 id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 name text NOT NULL, package_hash text NOT NULL, settings jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX workspace_configurations_owner ON syntelle_accounts.workspace_configurations(workspace_id);
CREATE TABLE syntelle_accounts.workspace_keys (
 id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES syntelle_accounts.workspaces(id),
 name text NOT NULL, secret_hash text NOT NULL, scope text NOT NULL CHECK(scope='streams'),
 created_by text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz
);
CREATE INDEX workspace_keys_owner ON syntelle_accounts.workspace_keys(workspace_id);
CREATE TABLE syntelle_accounts.staff_grants (
 account_id text PRIMARY KEY REFERENCES syntelle_accounts.accounts(id),
 role text NOT NULL CHECK(role IN ('administrator','support')),
 granted_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz
);
CREATE TABLE syntelle_accounts.staff_challenges (
 id text PRIMARY KEY, account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 session_id text NOT NULL REFERENCES syntelle_accounts.sessions(id),
 purpose text NOT NULL CHECK(purpose IN ('staff_entry','ownership_transfer','staff_grant')),
 resource_id text NOT NULL DEFAULT '',
 verifier text NOT NULL, attempts int NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
 consumed_at timestamptz, verified_at timestamptz, action_used_at timestamptz,
 delivery_status text NOT NULL DEFAULT 'started',
 request_ip_hash text NOT NULL
);
CREATE INDEX staff_challenges_account ON syntelle_accounts.staff_challenges(account_id,created_at);
CREATE INDEX staff_challenges_ip ON syntelle_accounts.staff_challenges(request_ip_hash,created_at);
CREATE TABLE syntelle_accounts.staff_access (
 session_id text PRIMARY KEY REFERENCES syntelle_accounts.sessions(id),
 account_id text NOT NULL REFERENCES syntelle_accounts.accounts(id),
 token_hash text NOT NULL,
 verified_at timestamptz NOT NULL, last_used_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL, revoked_at timestamptz
);
CREATE TABLE syntelle_accounts.workspace_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 workspace_id text REFERENCES syntelle_accounts.workspaces(id),
 actor_account_id text REFERENCES syntelle_accounts.accounts(id),
 action text NOT NULL, target_id text NOT NULL DEFAULT '',
 outcome text NOT NULL DEFAULT 'succeeded' CHECK (outcome IN ('succeeded','failed')),
 reason text NOT NULL DEFAULT '', before_state jsonb NOT NULL DEFAULT '{}'::jsonb,
 after_state jsonb NOT NULL DEFAULT '{}'::jsonb,
 created_at timestamptz NOT NULL DEFAULT now()
);
