BEGIN;

-- ---------------------------------------------------------------------------
-- Tenant onboarding lifecycle
-- ---------------------------------------------------------------------------

ALTER TABLE tenant_tenants
    ALTER COLUMN status SET DEFAULT 'provisioning';

ALTER TABLE tenant_tenants
    DROP CONSTRAINT tenant_tenants_status_check;

ALTER TABLE tenant_tenants
    ADD CONSTRAINT tenant_tenants_status_check
    CHECK (
        status IN (
            'provisioning',
            'active',
            'provisioning_failed',
            'archived'
        )
    );

ALTER TABLE tenant_tenants
    DROP CONSTRAINT tenant_tenants_archived_check;

ALTER TABLE tenant_tenants
    ADD CONSTRAINT tenant_tenants_archived_check
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
    );

CREATE TABLE tenant_onboarding_operations (
    operation_id UUID PRIMARY KEY,

    tenant_id UUID NOT NULL
        REFERENCES tenant_tenants(tenant_id)
        ON DELETE CASCADE,

    actor_user_id UUID NOT NULL,
    request_id TEXT NOT NULL,

    state TEXT NOT NULL,
    resource_name TEXT NOT NULL,

    workflow_id TEXT,
    workflow_run_id TEXT,

    error_code TEXT,
    error_message TEXT,
    error_retryable BOOLEAN,
    error_details JSONB NOT NULL DEFAULT '{}'::jsonb,

    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    version BIGINT NOT NULL DEFAULT 1,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    CONSTRAINT tenant_onboarding_operations_tenant_unique
        UNIQUE (tenant_id),

    CONSTRAINT tenant_onboarding_operations_request_unique
        UNIQUE (actor_user_id, request_id),

    CONSTRAINT tenant_onboarding_operations_state_check
        CHECK (
            state IN (
                'pending',
                'running',
                'succeeded',
                'failed',
                'canceled'
            )
        ),

    CONSTRAINT tenant_onboarding_operations_resource_name_check
        CHECK (
            resource_name =
                'tenants/' || tenant_id::text
        ),

    CONSTRAINT tenant_onboarding_operations_version_check
        CHECK (version > 0),

    CONSTRAINT tenant_onboarding_operations_error_details_check
        CHECK (
            jsonb_typeof(error_details) = 'object'
        ),

    CONSTRAINT tenant_onboarding_operations_metadata_check
        CHECK (
            jsonb_typeof(metadata) = 'object'
        ),

    CONSTRAINT tenant_onboarding_operations_state_timestamps_check
        CHECK (
            (
                state = 'pending'
                AND started_at IS NULL
                AND completed_at IS NULL
            )
            OR
            (
                state = 'running'
                AND started_at IS NOT NULL
                AND completed_at IS NULL
            )
            OR
            (
                state IN ('succeeded', 'failed', 'canceled')
                AND completed_at IS NOT NULL
            )
        ),

    CONSTRAINT tenant_onboarding_operations_error_check
        CHECK (
            (
                state = 'failed'
                AND error_code IS NOT NULL
                AND error_message IS NOT NULL
                AND error_retryable IS NOT NULL
            )
            OR
            (
                state <> 'failed'
                AND error_code IS NULL
                AND error_message IS NULL
                AND error_retryable IS NULL
                AND error_details = '{}'::jsonb
            )
        )
);

CREATE INDEX tenant_onboarding_operations_actor_created_idx
    ON tenant_onboarding_operations (
        actor_user_id,
        created_at DESC,
        operation_id DESC
    );

CREATE INDEX tenant_onboarding_operations_running_idx
    ON tenant_onboarding_operations (
        updated_at,
        operation_id
    )
    WHERE state IN ('pending', 'running');

-- ---------------------------------------------------------------------------
-- Runtime-role privileges
-- ---------------------------------------------------------------------------

REVOKE ALL
    ON TABLE tenant_onboarding_operations
    FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE tenant_onboarding_operations
    TO gereh_tenant_app;

-- ---------------------------------------------------------------------------
-- Enable RLS
-- ---------------------------------------------------------------------------

ALTER TABLE tenant_onboarding_operations
    ENABLE ROW LEVEL SECURITY;

ALTER TABLE tenant_onboarding_operations
    FORCE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- Onboarding-operation policies
-- ---------------------------------------------------------------------------

CREATE POLICY tenant_onboarding_operations_select
ON tenant_onboarding_operations
FOR SELECT
TO gereh_tenant_app
USING (
    (
        app.scope_kind() = 'tenant'
        AND tenant_id = app.current_tenant_id()
    )
    OR
    (
        app.scope_kind() = 'principal'
        AND app.current_principal_type() = 'user'
        AND actor_user_id =
            app.current_principal_id()
    )
);

CREATE POLICY tenant_onboarding_operations_insert
ON tenant_onboarding_operations
FOR INSERT
TO gereh_tenant_app
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY tenant_onboarding_operations_update
ON tenant_onboarding_operations
FOR UPDATE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY tenant_onboarding_operations_delete
ON tenant_onboarding_operations
FOR DELETE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

COMMIT;
