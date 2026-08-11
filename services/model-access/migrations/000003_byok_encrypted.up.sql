BEGIN;

-- ---------------------------------------------------------------------------
-- Public/sanitized fingerprint
-- ---------------------------------------------------------------------------

ALTER TABLE model_access_connections
    ADD COLUMN credential_fingerprint TEXT NOT NULL DEFAULT '';

ALTER TABLE model_access_connections
    ADD CONSTRAINT
        model_access_connections_credential_fingerprint_check
    CHECK (
        (
            connection_type = 'byok'
            AND
            (
                credential_fingerprint = ''
                OR credential_fingerprint ~
                   '^fp-[a-z0-9_-]{1,32}:[0-9a-f]{32}$'
            )
        )
        OR
        (
            connection_type <> 'byok'
            AND credential_fingerprint = ''
        )
    );

ALTER TABLE model_access_connection_revisions
    ADD COLUMN credential_fingerprint TEXT NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------------
-- Secret-reference state
--
-- NO RAW API KEY may ever be stored here.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_connection_credentials (
    tenant_id UUID NOT NULL,
    connection_id UUID NOT NULL,

    secret_ref TEXT NOT NULL,

    -- Full HMAC-SHA-256 digest.
    credential_fingerprint BYTEA NOT NULL,

    -- Public truncated representation.
    fingerprint_display TEXT NOT NULL,

    fingerprint_key_id TEXT NOT NULL,

    state TEXT NOT NULL,

    -- Version currently approved/active for inference.
    active_vault_version BIGINT NOT NULL DEFAULT 0,

    -- Highest KV version written, including failed/destroyed rotations.
    -- This is required for KV-v2 CAS semantics.
    vault_latest_version BIGINT NOT NULL DEFAULT 0,

    -- Version currently awaiting provider verification.
    pending_vault_version BIGINT NOT NULL DEFAULT 0,

    pending_fingerprint BYTEA,
    pending_fingerprint_display TEXT,

    credential_version BIGINT NOT NULL DEFAULT 1,

    verified_at TIMESTAMPTZ,
    destroyed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        connection_id
    ),

    FOREIGN KEY (
        tenant_id,
        connection_id
    )
    REFERENCES model_access_connections (
        tenant_id,
        connection_id
    )
    ON DELETE RESTRICT,

    UNIQUE (
        tenant_id,
        secret_ref
    ),

    CHECK (
        octet_length(
            credential_fingerprint
        ) = 32
    ),

    CHECK (
        pending_fingerprint IS NULL
        OR octet_length(
            pending_fingerprint
        ) = 32
    ),

    CHECK (
        fingerprint_display ~
        '^fp-[a-z0-9_-]{1,32}:[0-9a-f]{32}$'
    ),

    CHECK (
        pending_fingerprint_display IS NULL
        OR pending_fingerprint_display ~
           '^fp-[a-z0-9_-]{1,32}:[0-9a-f]{32}$'
    ),

    CHECK (
        state IN (
            'pending_store',
            'pending_verification',
            'active',
            'verification_failed',
            'destroyed'
        )
    ),

    CHECK (
        active_vault_version >= 0
    ),

    CHECK (
        vault_latest_version >=
        active_vault_version
    ),

    CHECK (
        pending_vault_version >= 0
    ),

    CHECK (
        credential_version > 0
    ),

    CHECK (
        char_length(secret_ref)
        BETWEEN 1 AND 512
    ),

    CHECK (
        char_length(fingerprint_key_id)
        BETWEEN 1 AND 32
    )
);

CREATE INDEX
    model_access_connection_credentials_state_idx
ON model_access_connection_credentials (
    tenant_id,
    state,
    connection_id
);

-- ---------------------------------------------------------------------------
-- Immutable credential-verification audit trail
--
-- Contains no secret material.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_connection_verification_events (
    tenant_id UUID NOT NULL,

    verification_event_id UUID NOT NULL,

    connection_id UUID NOT NULL,

    actor_user_id UUID NOT NULL,

    operation TEXT NOT NULL,
    outcome TEXT NOT NULL,
    reason_code TEXT NOT NULL,

    provider_http_status INTEGER,

    fingerprint_display TEXT NOT NULL,

    request_id TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',

    occurred_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        verification_event_id
    ),

    FOREIGN KEY (
        tenant_id,
        connection_id
    )
    REFERENCES model_access_connections (
        tenant_id,
        connection_id
    )
    ON DELETE RESTRICT,

    CHECK (
        operation IN (
            'create',
            'rotate'
        )
    ),

    CHECK (
        outcome IN (
            'succeeded',
            'rejected',
            'transient_failure'
        )
    ),

    CHECK (
        reason_code ~
        '^[a-z0-9._-]{1,64}$'
    ),

    CHECK (
        fingerprint_display ~
        '^fp-[a-z0-9_-]{1,32}:[0-9a-f]{32}$'
    ),

    CHECK (
        provider_http_status IS NULL
        OR provider_http_status
           BETWEEN 100 AND 599
    )
);

CREATE INDEX
    model_access_verification_events_connection_idx
ON model_access_connection_verification_events (
    tenant_id,
    connection_id,
    occurred_at DESC,
    verification_event_id DESC
);

-- ---------------------------------------------------------------------------
-- Credential rotation idempotency/progress.
--
-- This is separate from generic connection mutation idempotency because a
-- credential operation crosses PostgreSQL, Vault, and a provider API.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_credential_operations (
    tenant_id UUID NOT NULL,
    actor_user_id UUID NOT NULL,

    operation TEXT NOT NULL,

    idempotency_key UUID NOT NULL,

    request_hash BYTEA NOT NULL,

    connection_id UUID NOT NULL,

    state TEXT NOT NULL,

    response_connection_version BIGINT,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        actor_user_id,
        operation,
        idempotency_key
    ),

    FOREIGN KEY (
        tenant_id,
        connection_id
    )
    REFERENCES model_access_connections (
        tenant_id,
        connection_id
    )
    ON DELETE RESTRICT,

    CHECK (
        operation = 'rotate_byok_credential'
    ),

    CHECK (
        octet_length(request_hash) = 32
    ),

    CHECK (
        state IN (
            'prepared',
            'secret_stored',
            'succeeded',
            'rejected'
        )
    ),

    CHECK (
        response_connection_version IS NULL
        OR response_connection_version > 0
    ),

    CHECK (
        expires_at > created_at
    )
);

CREATE INDEX
    model_access_credential_operations_expiry_idx
ON model_access_credential_operations (
    expires_at
);

-- ---------------------------------------------------------------------------
-- Durable secret cleanup queue.
--
-- Operational table: no raw secret, only opaque Vault reference + version.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_secret_cleanup (
    cleanup_id BIGSERIAL PRIMARY KEY,

    tenant_id UUID NOT NULL,

    secret_ref TEXT NOT NULL,

    secret_version BIGINT,

    action TEXT NOT NULL,

    available_at TIMESTAMPTZ NOT NULL
        DEFAULT clock_timestamp(),

    claimed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    attempts INTEGER NOT NULL DEFAULT 0,

    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL,

    CHECK (
        action IN (
            'destroy_version',
            'purge_secret'
        )
    ),

    CHECK (
        (
            action = 'destroy_version'
            AND secret_version IS NOT NULL
            AND secret_version > 0
        )
        OR
        (
            action = 'purge_secret'
            AND secret_version IS NULL
        )
    ),

    CHECK (
        attempts >= 0
    ),

    CHECK (
        char_length(secret_ref)
        BETWEEN 1 AND 512
    )
);

CREATE INDEX
    model_access_secret_cleanup_pending_idx
ON model_access_secret_cleanup (
    available_at,
    cleanup_id
)
WHERE completed_at IS NULL;

-- ---------------------------------------------------------------------------
-- RLS on tenant business/security history tables.
-- ---------------------------------------------------------------------------

DO $block$
DECLARE
    table_name TEXT;

    protected_tables TEXT[] := ARRAY[
        'model_access_connection_credentials',
        'model_access_connection_verification_events',
        'model_access_credential_operations'
    ];
BEGIN
    FOREACH table_name IN ARRAY
        protected_tables
    LOOP
        EXECUTE format(
            'REVOKE ALL ON TABLE %I FROM PUBLIC',
            table_name
        );

        EXECUTE format(
            'ALTER TABLE %I ENABLE ROW LEVEL SECURITY',
            table_name
        );

        EXECUTE format(
            'ALTER TABLE %I FORCE ROW LEVEL SECURITY',
            table_name
        );

        EXECUTE format(
            'CREATE POLICY %I
             ON %I
             TO gereh_model_access_app
             USING (
                 app.scope_kind() = ''tenant''
                 AND tenant_id =
                     app.current_tenant_id()
             )
             WITH CHECK (
                 app.scope_kind() = ''tenant''
                 AND tenant_id =
                     app.current_tenant_id()
             )',
            table_name ||
                '_tenant_isolation',
            table_name
        );
    END LOOP;
END
$block$;

GRANT SELECT, INSERT, UPDATE
    ON TABLE model_access_connection_credentials
    TO gereh_model_access_app;

GRANT SELECT, INSERT
    ON TABLE model_access_connection_verification_events
    TO gereh_model_access_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE model_access_credential_operations
    TO gereh_model_access_app;

-- Operational cleanup queue is cross-tenant like the outbox.
REVOKE ALL
    ON TABLE model_access_secret_cleanup
    FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE
    ON TABLE model_access_secret_cleanup
    TO gereh_model_access_app;

GRANT USAGE, SELECT
    ON SEQUENCE
        model_access_secret_cleanup_cleanup_id_seq
    TO gereh_model_access_app;

-- Nous Portal currently exposes BYOK as product metadata in the existing
-- phase-16 provider seed, but the retrieved official API documentation does
-- not expose a stable verification contract that this service can safely
-- implement. Hide it until a supported first-party verifier is added.
UPDATE model_access_providers
SET
    enabled = FALSE,
    version = version + 1,
    updated_at = clock_timestamp()
WHERE provider_key = 'nous';

COMMIT;
