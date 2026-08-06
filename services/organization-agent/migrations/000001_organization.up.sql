BEGIN;

CREATE SCHEMA IF NOT EXISTS app;

REVOKE ALL ON SCHEMA app FROM PUBLIC;
GRANT USAGE ON SCHEMA app TO gereh_organization_app;

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

REVOKE ALL ON FUNCTION app.scope_kind() FROM PUBLIC;
REVOKE ALL ON FUNCTION app.current_tenant_id() FROM PUBLIC;
REVOKE ALL ON FUNCTION app.current_principal_id() FROM PUBLIC;
REVOKE ALL ON FUNCTION app.current_principal_type() FROM PUBLIC;

GRANT EXECUTE ON FUNCTION app.scope_kind()
    TO gereh_organization_app;

GRANT EXECUTE ON FUNCTION app.current_tenant_id()
    TO gereh_organization_app;

GRANT EXECUTE ON FUNCTION app.current_principal_id()
    TO gereh_organization_app;

GRANT EXECUTE ON FUNCTION app.current_principal_type()
    TO gereh_organization_app;

CREATE TABLE organization_companies (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,

    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    status TEXT NOT NULL DEFAULT 'active',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,

    version BIGINT NOT NULL DEFAULT 1,

    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, company_id),

    CONSTRAINT organization_companies_slug_unique
        UNIQUE (tenant_id, slug),

    CONSTRAINT organization_companies_status_check
        CHECK (status IN ('active', 'archived')),

    CONSTRAINT organization_companies_slug_check
        CHECK (
            slug ~ '^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$'
        ),

    CONSTRAINT organization_companies_display_name_check
        CHECK (
            char_length(display_name)
            BETWEEN 1 AND 120
        ),

    CONSTRAINT organization_companies_description_check
        CHECK (
            char_length(description) <= 2000
        ),

    CONSTRAINT organization_companies_version_check
        CHECK (version > 0),

    CONSTRAINT organization_companies_archive_check
        CHECK (
            (
                status = 'active'
                AND archived_at IS NULL
            )
            OR
            (
                status = 'archived'
                AND archived_at IS NOT NULL
            )
        )
);

CREATE UNIQUE INDEX organization_companies_default_unique
    ON organization_companies (tenant_id)
    WHERE is_default
      AND status = 'active';

CREATE INDEX organization_companies_list_idx
    ON organization_companies (
        tenant_id,
        company_id
    );

CREATE TABLE organization_agents (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    agent_id UUID NOT NULL,

    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role_title TEXT NOT NULL,
    objective TEXT NOT NULL,

    manager_agent_id UUID,

    status TEXT NOT NULL DEFAULT 'draft',
    execution_profile TEXT NOT NULL,
    autonomy_level TEXT NOT NULL,

    capabilities TEXT[] NOT NULL DEFAULT '{}',
    configuration JSONB NOT NULL DEFAULT '{}'::jsonb,

    version BIGINT NOT NULL DEFAULT 1,

    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, agent_id),

    CONSTRAINT organization_agents_company_fk
        FOREIGN KEY (tenant_id, company_id)
        REFERENCES organization_companies (
            tenant_id,
            company_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT organization_agents_company_identity_unique
        UNIQUE (
            tenant_id,
            company_id,
            agent_id
        ),

    CONSTRAINT organization_agents_slug_unique
        UNIQUE (
            tenant_id,
            company_id,
            slug
        ),

    CONSTRAINT organization_agents_manager_fk
        FOREIGN KEY (
            tenant_id,
            company_id,
            manager_agent_id
        )
        REFERENCES organization_agents (
            tenant_id,
            company_id,
            agent_id
        )
        ON DELETE RESTRICT
        DEFERRABLE INITIALLY IMMEDIATE,

    CONSTRAINT organization_agents_not_self_managed
        CHECK (
            manager_agent_id IS NULL
            OR manager_agent_id <> agent_id
        ),

    CONSTRAINT organization_agents_slug_check
        CHECK (
            slug ~ '^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$'
        ),

    CONSTRAINT organization_agents_display_name_check
        CHECK (
            char_length(display_name)
            BETWEEN 1 AND 120
        ),

    CONSTRAINT organization_agents_role_title_check
        CHECK (
            char_length(role_title)
            BETWEEN 1 AND 120
        ),

    CONSTRAINT organization_agents_objective_check
        CHECK (
            char_length(objective)
            BETWEEN 1 AND 4000
        ),

    CONSTRAINT organization_agents_status_check
        CHECK (
            status IN (
                'draft',
                'provisioning',
                'configuring_runtime',
                'health_checking',
                'ready',
                'degraded',
                'paused',
                'failed',
                'deleting',
                'deleted'
            )
        ),

    CONSTRAINT organization_agents_execution_profile_check
        CHECK (
            execution_profile IN (
                'balanced',
                'persistent',
                'technical_worker'
            )
        ),

    CONSTRAINT organization_agents_autonomy_level_check
        CHECK (
            autonomy_level IN (
                'observe_only',
                'suggest',
                'approval_required',
                'policy_bounded'
            )
        ),

    CONSTRAINT organization_agents_capabilities_check
        CHECK (
            cardinality(capabilities) <= 64
        ),

    CONSTRAINT organization_agents_configuration_check
        CHECK (
            jsonb_typeof(configuration) = 'object'
            AND pg_column_size(configuration) <= 65536
        ),

    CONSTRAINT organization_agents_version_check
        CHECK (version > 0),

    CONSTRAINT organization_agents_delete_check
        CHECK (
            (
                status = 'deleted'
                AND deleted_at IS NOT NULL
            )
            OR
            (
                status <> 'deleted'
                AND deleted_at IS NULL
            )
        )
);

CREATE INDEX organization_agents_company_list_idx
    ON organization_agents (
        tenant_id,
        company_id,
        agent_id
    );

CREATE INDEX organization_agents_manager_idx
    ON organization_agents (
        tenant_id,
        company_id,
        manager_agent_id
    )
    WHERE manager_agent_id IS NOT NULL;

CREATE INDEX organization_agents_active_idx
    ON organization_agents (
        tenant_id,
        company_id,
        status
    )
    WHERE status <> 'deleted';

CREATE TABLE organization_agent_revisions (
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    version BIGINT NOT NULL,

    change_kind TEXT NOT NULL,
    snapshot JSONB NOT NULL,

    actor_user_id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        agent_id,
        version
    ),

    CONSTRAINT organization_agent_revisions_agent_fk
        FOREIGN KEY (tenant_id, agent_id)
        REFERENCES organization_agents (
            tenant_id,
            agent_id
        )
        ON DELETE CASCADE,

    CONSTRAINT organization_agent_revisions_change_kind_check
        CHECK (
            change_kind IN (
                'created',
                'updated',
                'manager_changed',
                'paused',
                'resumed',
                'deleted'
            )
        ),

    CONSTRAINT organization_agent_revisions_snapshot_check
        CHECK (
            jsonb_typeof(snapshot) = 'object'
        )
);

CREATE TABLE organization_bootstrap_requests (
    onboarding_operation_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL UNIQUE,
    company_id UUID NOT NULL,
    actor_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT organization_bootstrap_requests_company_fk
        FOREIGN KEY (tenant_id, company_id)
        REFERENCES organization_companies (
            tenant_id,
            company_id
        )
        ON DELETE RESTRICT
);

CREATE TABLE organization_outbox (
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

    CONSTRAINT organization_outbox_attempts_check
        CHECK (attempts >= 0)
);

CREATE INDEX organization_outbox_pending_idx
    ON organization_outbox (
        available_at,
        outbox_id
    )
    WHERE published_at IS NULL;

CREATE INDEX organization_outbox_tenant_idx
    ON organization_outbox (
        tenant_id,
        occurred_at DESC,
        outbox_id DESC
    );

REVOKE ALL ON TABLE organization_companies FROM PUBLIC;
REVOKE ALL ON TABLE organization_agents FROM PUBLIC;
REVOKE ALL ON TABLE organization_agent_revisions FROM PUBLIC;
REVOKE ALL ON TABLE organization_bootstrap_requests FROM PUBLIC;
REVOKE ALL ON TABLE organization_outbox FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE organization_companies
    TO gereh_organization_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE organization_agents
    TO gereh_organization_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE organization_agent_revisions
    TO gereh_organization_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE organization_bootstrap_requests
    TO gereh_organization_app;

GRANT SELECT, INSERT, UPDATE
    ON TABLE organization_outbox
    TO gereh_organization_app;

GRANT USAGE, SELECT
    ON SEQUENCE organization_outbox_outbox_id_seq
    TO gereh_organization_app;

ALTER TABLE organization_companies
    ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_companies
    FORCE ROW LEVEL SECURITY;

ALTER TABLE organization_agents
    ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_agents
    FORCE ROW LEVEL SECURITY;

ALTER TABLE organization_agent_revisions
    ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_agent_revisions
    FORCE ROW LEVEL SECURITY;

ALTER TABLE organization_bootstrap_requests
    ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_bootstrap_requests
    FORCE ROW LEVEL SECURITY;

CREATE POLICY organization_companies_tenant
ON organization_companies
TO gereh_organization_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY organization_agents_tenant
ON organization_agents
TO gereh_organization_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY organization_agent_revisions_tenant
ON organization_agent_revisions
TO gereh_organization_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY organization_bootstrap_requests_tenant
ON organization_bootstrap_requests
TO gereh_organization_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

COMMIT;
