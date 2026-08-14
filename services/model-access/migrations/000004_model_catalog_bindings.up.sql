BEGIN;

-- ---------------------------------------------------------------------------
-- Connection-scoped tenant catalog state.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_catalog_states (
    tenant_id UUID NOT NULL,
    connection_id UUID NOT NULL,

    generation BIGINT NOT NULL DEFAULT 0,

    last_success_at TIMESTAMPTZ,

    available_count INTEGER NOT NULL DEFAULT 0,
    unavailable_count INTEGER NOT NULL DEFAULT 0,

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

    CHECK (generation >= 0),

    CHECK (available_count >= 0),
    CHECK (unavailable_count >= 0)
);

-- ---------------------------------------------------------------------------
-- Stable tenant + connection-scoped model offerings.
--
-- Offerings are never reused for a different provider model.
-- A disappeared model becomes unavailable rather than being deleted.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_model_offerings (
    tenant_id UUID NOT NULL,
    offering_id UUID NOT NULL,

    connection_id UUID NOT NULL,

    provider_key TEXT NOT NULL,
    provider_model_id TEXT NOT NULL,

    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    status TEXT NOT NULL,

    source TEXT NOT NULL,

    agent_usable BOOLEAN NOT NULL DEFAULT FALSE,

    capabilities TEXT[] NOT NULL
        DEFAULT ARRAY[]::TEXT[],

    input_modalities TEXT[] NOT NULL
        DEFAULT ARRAY[]::TEXT[],

    output_modalities TEXT[] NOT NULL
        DEFAULT ARRAY[]::TEXT[],

    context_window_tokens BIGINT NOT NULL DEFAULT 0,
    max_output_tokens BIGINT NOT NULL DEFAULT 0,

    version BIGINT NOT NULL DEFAULT 1,

    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    refreshed_at TIMESTAMPTZ NOT NULL,

    unavailable_at TIMESTAMPTZ,

    PRIMARY KEY (
        tenant_id,
        offering_id
    ),

    UNIQUE (
        tenant_id,
        connection_id,
        provider_model_id
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
        char_length(provider_model_id)
        BETWEEN 1 AND 512
    ),

    CHECK (
        char_length(display_name)
        BETWEEN 1 AND 256
    ),

    CHECK (
        char_length(description) <= 4000
    ),

    CHECK (
        status IN (
            'available',
            'unavailable'
        )
    ),

    CHECK (
        source IN (
            'provider_discovered',
            'platform_catalog'
        )
    ),

    CHECK (context_window_tokens >= 0),
    CHECK (max_output_tokens >= 0),

    CHECK (version > 0),

    CHECK (
        (
            status = 'available'
            AND unavailable_at IS NULL
        )
        OR
        (
            status = 'unavailable'
            AND unavailable_at IS NOT NULL
        )
    )
);

CREATE INDEX
    model_access_model_offerings_list_idx
ON model_access_model_offerings (
    tenant_id,
    status,
    offering_id
);

CREATE INDEX
    model_access_model_offerings_connection_idx
ON model_access_model_offerings (
    tenant_id,
    connection_id,
    status,
    offering_id
);

CREATE INDEX
    model_access_model_offerings_agent_usable_idx
ON model_access_model_offerings (
    tenant_id,
    agent_usable,
    status,
    offering_id
)
WHERE
    agent_usable
    AND status = 'available';

-- ---------------------------------------------------------------------------
-- User-visible catalog refresh operation.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_catalog_refreshes (
    tenant_id UUID NOT NULL,
    refresh_id UUID NOT NULL,

    actor_user_id UUID NOT NULL,

    connection_id UUID NOT NULL,

    idempotency_key UUID NOT NULL,

    reason TEXT NOT NULL,

    status TEXT NOT NULL,

    catalog_generation BIGINT NOT NULL DEFAULT 0,

    discovered_count INTEGER NOT NULL DEFAULT 0,
    available_count INTEGER NOT NULL DEFAULT 0,
    unavailable_count INTEGER NOT NULL DEFAULT 0,

    error_code TEXT NOT NULL DEFAULT '',

    requested_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    PRIMARY KEY (
        tenant_id,
        refresh_id
    ),

    UNIQUE (
        tenant_id,
        actor_user_id,
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
        reason IN (
            'manual',
            'connection_activated',
            'credential_rotated'
        )
    ),

    CHECK (
        status IN (
            'pending',
            'running',
            'succeeded',
            'failed'
        )
    ),

    CHECK (
        catalog_generation >= 0
    ),

    CHECK (
        discovered_count >= 0
        AND available_count >= 0
        AND unavailable_count >= 0
    ),

    CHECK (
        char_length(error_code) <= 64
    )
);

CREATE INDEX
    model_access_catalog_refreshes_connection_idx
ON model_access_catalog_refreshes (
    tenant_id,
    connection_id,
    requested_at DESC
);

-- Operational queue. Contains no credential or provider payload.
CREATE TABLE model_access_catalog_refresh_queue (
    refresh_id UUID PRIMARY KEY,

    tenant_id UUID NOT NULL,
    actor_user_id UUID NOT NULL,
    connection_id UUID NOT NULL,

    available_at TIMESTAMPTZ NOT NULL,

    claimed_at TIMESTAMPTZ,

    attempts INTEGER NOT NULL DEFAULT 0,

    last_error TEXT,

    CHECK (attempts >= 0)
);

CREATE INDEX
    model_access_catalog_refresh_queue_pending_idx
ON model_access_catalog_refresh_queue (
    available_at,
    refresh_id
);

-- ---------------------------------------------------------------------------
-- Agent-model binding aggregate.
--
-- agent_id belongs to Organization Service.
-- No cross-service FK is intentionally created.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_agent_bindings (
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,

    company_id UUID NOT NULL,

    status TEXT NOT NULL,

    primary_offering_id UUID NOT NULL,

    fast_offering_id UUID,

    fallback_policy TEXT NOT NULL,

    max_model_cost_microusd BIGINT,

    version BIGINT NOT NULL DEFAULT 1,

    created_by_user_id UUID NOT NULL,
    updated_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    removed_at TIMESTAMPTZ,

    PRIMARY KEY (
        tenant_id,
        agent_id
    ),

    FOREIGN KEY (
        tenant_id,
        primary_offering_id
    )
    REFERENCES model_access_model_offerings (
        tenant_id,
        offering_id
    )
    ON DELETE RESTRICT,

    FOREIGN KEY (
        tenant_id,
        fast_offering_id
    )
    REFERENCES model_access_model_offerings (
        tenant_id,
        offering_id
    )
    ON DELETE RESTRICT,

    CHECK (
        status IN (
            'active',
            'removed'
        )
    ),

    CHECK (
        fallback_policy IN (
            'none',
            'ordered'
        )
    ),

    CHECK (
        max_model_cost_microusd IS NULL
        OR max_model_cost_microusd > 0
    ),

    CHECK (version > 0),

    CHECK (
        (
            status = 'active'
            AND removed_at IS NULL
        )
        OR
        (
            status = 'removed'
            AND removed_at IS NOT NULL
        )
    )
);

CREATE INDEX
    model_access_agent_bindings_company_idx
ON model_access_agent_bindings (
    tenant_id,
    company_id,
    status,
    agent_id
);

-- Ordered fallback models.
CREATE TABLE model_access_agent_binding_fallbacks (
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,

    position INTEGER NOT NULL,

    offering_id UUID NOT NULL,

    PRIMARY KEY (
        tenant_id,
        agent_id,
        position
    ),

    UNIQUE (
        tenant_id,
        agent_id,
        offering_id
    ),

    FOREIGN KEY (
        tenant_id,
        agent_id
    )
    REFERENCES model_access_agent_bindings (
        tenant_id,
        agent_id
    )
    ON DELETE RESTRICT,

    FOREIGN KEY (
        tenant_id,
        offering_id
    )
    REFERENCES model_access_model_offerings (
        tenant_id,
        offering_id
    )
    ON DELETE RESTRICT,

    CHECK (
        position BETWEEN 0 AND 4
    )
);

-- Immutable binding history.
CREATE TABLE model_access_agent_binding_revisions (
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,

    revision BIGINT NOT NULL,

    company_id UUID NOT NULL,

    status TEXT NOT NULL,

    primary_offering_id UUID NOT NULL,

    fast_offering_id UUID,

    fallback_offering_ids UUID[] NOT NULL
        DEFAULT ARRAY[]::UUID[],

    fallback_policy TEXT NOT NULL,

    max_model_cost_microusd BIGINT,

    change_kind TEXT NOT NULL,

    changed_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        agent_id,
        revision
    ),

    CHECK (revision > 0),

    CHECK (
        status IN (
            'active',
            'removed'
        )
    ),

    CHECK (
        fallback_policy IN (
            'none',
            'ordered'
        )
    ),

    CHECK (
        change_kind IN (
            'created',
            'updated',
            'reactivated',
            'removed'
        )
    )
);

-- Mutation idempotency for the binding aggregate.
CREATE TABLE model_access_binding_idempotency (
    tenant_id UUID NOT NULL,
    actor_user_id UUID NOT NULL,

    operation TEXT NOT NULL,

    idempotency_key UUID NOT NULL,

    request_hash BYTEA NOT NULL,

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
            'set_agent_model_binding',
            'remove_agent_model_binding'
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
        pg_column_size(
            response_snapshot
        ) <= 32768
    ),

    CHECK (
        expires_at > created_at
    )
);

CREATE INDEX
    model_access_binding_idempotency_expiry_idx
ON model_access_binding_idempotency (
    expires_at
);

-- ---------------------------------------------------------------------------
-- Tenant RLS.
-- ---------------------------------------------------------------------------

DO $block$
DECLARE
    table_name TEXT;

    protected_tables TEXT[] := ARRAY[
        'model_access_catalog_states',
        'model_access_model_offerings',
        'model_access_catalog_refreshes',
        'model_access_agent_bindings',
        'model_access_agent_binding_fallbacks',
        'model_access_agent_binding_revisions',
        'model_access_binding_idempotency'
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
    ON model_access_catalog_states
    TO gereh_model_access_app;

GRANT SELECT, INSERT, UPDATE
    ON model_access_model_offerings
    TO gereh_model_access_app;

GRANT SELECT, INSERT, UPDATE
    ON model_access_catalog_refreshes
    TO gereh_model_access_app;

REVOKE ALL
    ON model_access_catalog_refresh_queue
    FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON model_access_catalog_refresh_queue
    TO gereh_model_access_app;

GRANT SELECT, INSERT, UPDATE
    ON model_access_agent_bindings
    TO gereh_model_access_app;

-- Replacement of fallback lists needs DELETE.
GRANT SELECT, INSERT, DELETE
    ON model_access_agent_binding_fallbacks
    TO gereh_model_access_app;

GRANT SELECT, INSERT
    ON model_access_agent_binding_revisions
    TO gereh_model_access_app;

GRANT SELECT, INSERT, DELETE
    ON model_access_binding_idempotency
    TO gereh_model_access_app;

COMMIT;
