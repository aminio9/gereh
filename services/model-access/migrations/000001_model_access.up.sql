BEGIN;

CREATE SCHEMA IF NOT EXISTS app;

REVOKE ALL ON SCHEMA app FROM PUBLIC;

GRANT USAGE
    ON SCHEMA app
    TO gereh_model_access_app;

CREATE OR REPLACE FUNCTION app.scope_kind()
RETURNS TEXT
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.scope_kind', TRUE),
        ''
    )
$function$;

CREATE OR REPLACE FUNCTION app.current_tenant_id()
RETURNS UUID
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.tenant_id', TRUE),
        ''
    )::UUID
$function$;

CREATE OR REPLACE FUNCTION app.current_principal_id()
RETURNS UUID
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.principal_id', TRUE),
        ''
    )::UUID
$function$;

CREATE OR REPLACE FUNCTION app.current_principal_type()
RETURNS TEXT
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.principal_type', TRUE),
        ''
    )
$function$;

REVOKE ALL
    ON FUNCTION app.scope_kind()
    FROM PUBLIC;

REVOKE ALL
    ON FUNCTION app.current_tenant_id()
    FROM PUBLIC;

REVOKE ALL
    ON FUNCTION app.current_principal_id()
    FROM PUBLIC;

REVOKE ALL
    ON FUNCTION app.current_principal_type()
    FROM PUBLIC;

GRANT EXECUTE
    ON FUNCTION app.scope_kind()
    TO gereh_model_access_app;

GRANT EXECUTE
    ON FUNCTION app.current_tenant_id()
    TO gereh_model_access_app;

GRANT EXECUTE
    ON FUNCTION app.current_principal_id()
    TO gereh_model_access_app;

GRANT EXECUTE
    ON FUNCTION app.current_principal_type()
    TO gereh_model_access_app;

-- ---------------------------------------------------------------------------
-- Global provider metadata
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_providers (
    provider_key TEXT PRIMARY KEY,

    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    supported_connection_types TEXT[] NOT NULL,

    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    sort_order INTEGER NOT NULL DEFAULT 0,

    version BIGINT NOT NULL DEFAULT 1,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    CHECK (
        provider_key ~
        '^[a-z0-9][a-z0-9._-]{0,63}$'
    ),

    CHECK (
        char_length(display_name)
        BETWEEN 1 AND 120
    ),

    CHECK (
        char_length(description) <= 2000
    ),

    CHECK (
        cardinality(supported_connection_types)
        BETWEEN 1 AND 3
    ),

    CHECK (
        supported_connection_types
        <@
        ARRAY[
            'platform_managed',
            'byok',
            'private_endpoint'
        ]::TEXT[]
    ),

    CHECK (version > 0)
);

INSERT INTO model_access_providers (
    provider_key,
    display_name,
    description,
    supported_connection_types,
    enabled,
    sort_order,
    created_at,
    updated_at
)
VALUES
(
    'openai',
    'OpenAI',
    'OpenAI model provider.',
    ARRAY['platform_managed', 'byok']::TEXT[],
    TRUE,
    10,
    clock_timestamp(),
    clock_timestamp()
),
(
    'anthropic',
    'Anthropic',
    'Anthropic model provider.',
    ARRAY['platform_managed', 'byok']::TEXT[],
    TRUE,
    20,
    clock_timestamp(),
    clock_timestamp()
),
(
    'google',
    'Google',
    'Google model provider.',
    ARRAY['platform_managed', 'byok']::TEXT[],
    TRUE,
    30,
    clock_timestamp(),
    clock_timestamp()
),
(
    'openrouter',
    'OpenRouter',
    'OpenRouter model provider.',
    ARRAY['platform_managed', 'byok']::TEXT[],
    TRUE,
    40,
    clock_timestamp(),
    clock_timestamp()
),
(
    'nous',
    'Nous',
    'Nous model provider.',
    ARRAY['byok']::TEXT[],
    TRUE,
    50,
    clock_timestamp(),
    clock_timestamp()
),
(
    'custom',
    'Custom endpoint',
    'Customer-owned private model endpoint.',
    ARRAY['private_endpoint']::TEXT[],
    TRUE,
    100,
    clock_timestamp(),
    clock_timestamp()
);

-- Provider rows are platform metadata.
-- Runtime may read, but not mutate them.
REVOKE ALL
    ON TABLE model_access_providers
    FROM PUBLIC;

GRANT SELECT
    ON TABLE model_access_providers
    TO gereh_model_access_app;

-- ---------------------------------------------------------------------------
-- Tenant connections
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_connections (
    tenant_id UUID NOT NULL,
    connection_id UUID NOT NULL,

    provider_key TEXT NOT NULL,

    connection_type TEXT NOT NULL,

    display_name TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'draft',

    version BIGINT NOT NULL DEFAULT 1,

    created_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ,

    PRIMARY KEY (
        tenant_id,
        connection_id
    ),

    FOREIGN KEY (provider_key)
    REFERENCES model_access_providers (
        provider_key
    )
    ON UPDATE RESTRICT
    ON DELETE RESTRICT,

    CHECK (
        connection_type IN (
            'platform_managed',
            'byok',
            'private_endpoint'
        )
    ),

    CHECK (
        status IN (
            'draft',
            'pending_verification',
            'active',
            'verification_failed',
            'disabled',
            'archived'
        )
    ),

    CHECK (
        char_length(display_name)
        BETWEEN 1 AND 120
    ),

    CHECK (version > 0),

    CHECK (
        (
            status = 'archived'
            AND archived_at IS NOT NULL
        )
        OR
        (
            status <> 'archived'
            AND archived_at IS NULL
        )
    )
);

CREATE UNIQUE INDEX
    model_access_connections_name_unique
ON model_access_connections (
    tenant_id,
    lower(display_name)
)
WHERE status <> 'archived';

CREATE INDEX
    model_access_connections_provider_idx
ON model_access_connections (
    tenant_id,
    provider_key,
    status,
    connection_id
);

CREATE INDEX
    model_access_connections_list_idx
ON model_access_connections (
    tenant_id,
    connection_id
);

-- ---------------------------------------------------------------------------
-- Immutable revisions
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_connection_revisions (
    tenant_id UUID NOT NULL,
    connection_id UUID NOT NULL,

    revision BIGINT NOT NULL,

    provider_key TEXT NOT NULL,
    connection_type TEXT NOT NULL,

    display_name TEXT NOT NULL,

    status TEXT NOT NULL,

    change_kind TEXT NOT NULL,

    changed_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        connection_id,
        revision
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

    CHECK (revision > 0),

    CHECK (
        change_kind IN (
            'created',
            'updated',
            'archived'
        )
    )
);

CREATE INDEX
    model_access_connection_revisions_list_idx
ON model_access_connection_revisions (
    tenant_id,
    connection_id,
    revision DESC
);

-- ---------------------------------------------------------------------------
-- Mutation idempotency
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_idempotency (
    tenant_id UUID NOT NULL,
    actor_user_id UUID NOT NULL,

    operation TEXT NOT NULL,

    idempotency_key UUID NOT NULL,

    request_hash BYTEA NOT NULL,

    -- Sanitized ModelConnection snapshot only.
    -- This table must never contain secret material.
    response_snapshot JSONB NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        actor_user_id,
        operation,
        idempotency_key
    ),

    CHECK (
        operation IN (
            'create_connection',
            'update_connection',
            'archive_connection'
        )
    ),

    CHECK (
        octet_length(request_hash) = 32
    ),

    CHECK (
        jsonb_typeof(response_snapshot) =
        'object'
    ),

    CHECK (
        pg_column_size(response_snapshot) <=
        16384
    ),

    CHECK (
        expires_at > created_at
    )
);

CREATE INDEX
    model_access_idempotency_expiry_idx
ON model_access_idempotency (
    expires_at
);

-- ---------------------------------------------------------------------------
-- Transactional outbox
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_outbox (
    outbox_id BIGSERIAL PRIMARY KEY,

    tenant_id UUID NOT NULL,

    event_id UUID NOT NULL UNIQUE,

    topic TEXT NOT NULL,
    partition_key TEXT NOT NULL,

    envelope BYTEA NOT NULL,

    occurred_at TIMESTAMPTZ NOT NULL,

    available_at TIMESTAMPTZ NOT NULL
        DEFAULT clock_timestamp(),

    claimed_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,

    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,

    CHECK (attempts >= 0)
);

CREATE INDEX
    model_access_outbox_pending_idx
ON model_access_outbox (
    available_at,
    outbox_id
)
WHERE published_at IS NULL;

CREATE INDEX
    model_access_outbox_tenant_idx
ON model_access_outbox (
    tenant_id,
    occurred_at DESC,
    outbox_id DESC
);

-- ---------------------------------------------------------------------------
-- RLS
-- ---------------------------------------------------------------------------

DO $block$
DECLARE
    table_name TEXT;

    protected_tables TEXT[] := ARRAY[
        'model_access_connections',
        'model_access_connection_revisions',
        'model_access_idempotency'
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
            table_name || '_tenant_isolation',
            table_name
        );
    END LOOP;
END
$block$;

GRANT SELECT, INSERT, UPDATE
    ON TABLE model_access_connections
    TO gereh_model_access_app;

-- Revision history is immutable.
GRANT SELECT, INSERT
    ON TABLE model_access_connection_revisions
    TO gereh_model_access_app;

-- DELETE is needed only to expire an idempotency key.
GRANT SELECT, INSERT, DELETE
    ON TABLE model_access_idempotency
    TO gereh_model_access_app;

-- Outbox is cross-tenant operational service state;
-- the relay uses it outside tenant transactions.
REVOKE ALL
    ON TABLE model_access_outbox
    FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE
    ON TABLE model_access_outbox
    TO gereh_model_access_app;

GRANT USAGE, SELECT
    ON SEQUENCE
        model_access_outbox_outbox_id_seq
    TO gereh_model_access_app;

COMMIT;
