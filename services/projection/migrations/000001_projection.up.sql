BEGIN;

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SCHEMA IF NOT EXISTS app;

REVOKE ALL ON SCHEMA app FROM PUBLIC;
GRANT USAGE ON SCHEMA app TO gereh_projection_app;

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
    TO gereh_projection_app;
GRANT EXECUTE ON FUNCTION app.current_tenant_id()
    TO gereh_projection_app;
GRANT EXECUTE ON FUNCTION app.current_principal_id()
    TO gereh_projection_app;
GRANT EXECUTE ON FUNCTION app.current_principal_type()
    TO gereh_projection_app;

-- ---------------------------------------------------------------------------
-- Durable consumer state
-- ---------------------------------------------------------------------------

CREATE TABLE projection_consumed_events (
    event_id UUID PRIMARY KEY,

    tenant_id UUID NOT NULL,

    topic TEXT NOT NULL,
    partition INTEGER NOT NULL,
    offset_value BIGINT NOT NULL,

    event_type TEXT NOT NULL,
    event_version INTEGER NOT NULL,

    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL,

    event_hash BYTEA NOT NULL,

    occurred_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NOT NULL,

    UNIQUE (
        topic,
        partition,
        offset_value
    ),

    CHECK (partition >= 0),
    CHECK (offset_value >= 0),
    CHECK (event_version > 0),
    CHECK (aggregate_version >= 0),
    CHECK (octet_length(event_hash) = 32)
);

-- Cross-tenant operational state. Not exposed to query APIs.
CREATE TABLE projection_partition_checkpoints (
    topic TEXT NOT NULL,
    partition INTEGER NOT NULL,

    last_offset BIGINT NOT NULL,
    last_event_id UUID NOT NULL,

    last_event_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        topic,
        partition
    ),

    CHECK (partition >= 0),
    CHECK (last_offset >= 0)
);

CREATE TABLE projection_tenant_watermarks (
    tenant_id UUID PRIMARY KEY,

    projected_through_event_time TIMESTAMPTZ NOT NULL,
    last_processed_at TIMESTAMPTZ NOT NULL
);

-- ---------------------------------------------------------------------------
-- Tenant
-- ---------------------------------------------------------------------------

CREATE TABLE projection_tenants (
    tenant_id UUID PRIMARY KEY,

    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL,
    region TEXT NOT NULL,
    retention_days INTEGER NOT NULL,

    source_version BIGINT NOT NULL,
    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    CHECK (source_version > 0)
);

-- ---------------------------------------------------------------------------
-- Organization
-- ---------------------------------------------------------------------------

CREATE TABLE projection_companies (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,

    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,
    is_default BOOLEAN NOT NULL,

    source_version BIGINT NOT NULL,
    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        company_id
    ),

    CHECK (source_version > 0)
);

CREATE INDEX projection_companies_status_idx
    ON projection_companies (
        tenant_id,
        status,
        company_id
    );

CREATE TABLE projection_agents (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    agent_id UUID NOT NULL,

    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role_title TEXT NOT NULL,
    objective TEXT NOT NULL,

    manager_agent_id UUID,

    status TEXT NOT NULL,
    execution_profile TEXT NOT NULL,
    autonomy_level TEXT NOT NULL,

    source_version BIGINT NOT NULL,
    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        agent_id
    ),

    CHECK (source_version > 0)
);

CREATE INDEX projection_agents_company_idx
    ON projection_agents (
        tenant_id,
        company_id,
        status,
        agent_id
    );

CREATE INDEX projection_agents_manager_idx
    ON projection_agents (
        tenant_id,
        manager_agent_id
    )
    WHERE manager_agent_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Work
-- ---------------------------------------------------------------------------

CREATE TABLE projection_goals (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    goal_id UUID NOT NULL,

    title TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,

    source_version BIGINT NOT NULL,
    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        goal_id
    ),

    CHECK (source_version > 0)
);

CREATE INDEX projection_goals_company_idx
    ON projection_goals (
        tenant_id,
        company_id,
        status,
        goal_id
    );

CREATE TABLE projection_projects (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    goal_id UUID NOT NULL,
    project_id UUID NOT NULL,

    title TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,

    source_version BIGINT NOT NULL,
    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        project_id
    ),

    CHECK (source_version > 0)
);

CREATE INDEX projection_projects_company_idx
    ON projection_projects (
        tenant_id,
        company_id,
        status,
        project_id
    );

CREATE INDEX projection_projects_goal_idx
    ON projection_projects (
        tenant_id,
        goal_id,
        project_id
    );

CREATE TABLE projection_tasks (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    project_id UUID NOT NULL,
    task_id UUID NOT NULL,

    parent_task_id UUID,

    title TEXT NOT NULL,
    description TEXT NOT NULL,

    status TEXT NOT NULL,
    priority TEXT NOT NULL,

    created_by_user_id UUID NOT NULL,

    source_version BIGINT NOT NULL,
    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        task_id
    ),

    CHECK (source_version > 0)
);

CREATE INDEX projection_tasks_company_status_idx
    ON projection_tasks (
        tenant_id,
        company_id,
        status,
        priority,
        task_id
    );

CREATE INDEX projection_tasks_project_idx
    ON projection_tasks (
        tenant_id,
        project_id,
        status,
        task_id
    );

CREATE TABLE projection_task_dependencies (
    tenant_id UUID NOT NULL,
    project_id UUID NOT NULL,

    task_id UUID NOT NULL,
    depends_on_task_id UUID NOT NULL,

    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        task_id,
        depends_on_task_id
    )
);

CREATE INDEX projection_dependencies_prerequisite_idx
    ON projection_task_dependencies (
        tenant_id,
        depends_on_task_id
    );

CREATE TABLE projection_task_assignments (
    tenant_id UUID NOT NULL,
    task_id UUID NOT NULL,
    assignment_id UUID NOT NULL,

    assignee_type TEXT NOT NULL,

    user_id UUID,
    agent_id UUID,

    assignment_role TEXT NOT NULL,

    assigned_at TIMESTAMPTZ NOT NULL,

    source_event_id UUID NOT NULL,
    source_event_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        assignment_id
    )
);

CREATE INDEX projection_assignments_task_idx
    ON projection_task_assignments (
        tenant_id,
        task_id,
        assignment_id
    );

CREATE INDEX projection_assignments_agent_idx
    ON projection_task_assignments (
        tenant_id,
        agent_id,
        task_id
    )
    WHERE agent_id IS NOT NULL;

CREATE INDEX projection_assignments_user_idx
    ON projection_task_assignments (
        tenant_id,
        user_id,
        task_id
    )
    WHERE user_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Activity feed
-- ---------------------------------------------------------------------------

CREATE TABLE projection_task_activity (
    tenant_id UUID NOT NULL,
    event_id UUID NOT NULL,

    event_type TEXT NOT NULL,

    company_id UUID,
    project_id UUID,
    task_id UUID,

    actor_type TEXT,
    actor_id UUID,

    summary TEXT NOT NULL,

    occurred_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        event_id
    ),

    CHECK (
        char_length(summary)
        BETWEEN 1 AND 500
    )
);

CREATE INDEX projection_activity_tenant_idx
    ON projection_task_activity (
        tenant_id,
        occurred_at DESC,
        event_id DESC
    );

CREATE INDEX projection_activity_company_idx
    ON projection_task_activity (
        tenant_id,
        company_id,
        occurred_at DESC,
        event_id DESC
    )
    WHERE company_id IS NOT NULL;

CREATE INDEX projection_activity_task_idx
    ON projection_task_activity (
        tenant_id,
        task_id,
        occurred_at DESC,
        event_id DESC
    )
    WHERE task_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Search
-- ---------------------------------------------------------------------------

CREATE TABLE projection_search_documents (
    tenant_id UUID NOT NULL,

    document_type TEXT NOT NULL,
    document_id UUID NOT NULL,

    company_id UUID,

    title TEXT NOT NULL,
    subtitle TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',

    deleted BOOLEAN NOT NULL DEFAULT FALSE,

    source_version BIGINT NOT NULL,
    source_event_id UUID NOT NULL,

    updated_at TIMESTAMPTZ NOT NULL,
    projected_at TIMESTAMPTZ NOT NULL,

    search_vector TSVECTOR
        GENERATED ALWAYS AS (
            setweight(
                to_tsvector(
                    'simple',
                    coalesce(title, '')
                ),
                'A'
            )
            ||
            setweight(
                to_tsvector(
                    'simple',
                    coalesce(subtitle, '')
                ),
                'B'
            )
            ||
            setweight(
                to_tsvector(
                    'simple',
                    coalesce(body, '')
                ),
                'C'
            )
        ) STORED,

    PRIMARY KEY (
        tenant_id,
        document_type,
        document_id
    ),

    CHECK (
        document_type IN (
            'company',
            'agent',
            'goal',
            'project',
            'task'
        )
    ),

    CHECK (source_version > 0)
);

CREATE INDEX projection_search_vector_idx
    ON projection_search_documents
    USING GIN (search_vector);

CREATE INDEX projection_search_title_trgm_idx
    ON projection_search_documents
    USING GIN (
        title gin_trgm_ops
    );

CREATE INDEX projection_search_company_idx
    ON projection_search_documents (
        tenant_id,
        company_id,
        document_type,
        updated_at DESC
    )
    WHERE NOT deleted;

-- ---------------------------------------------------------------------------
-- Privileges
-- ---------------------------------------------------------------------------

DO $block$
DECLARE
    table_name TEXT;

    protected_tables TEXT[] := ARRAY[
        'projection_consumed_events',
        'projection_tenant_watermarks',
        'projection_tenants',
        'projection_companies',
        'projection_agents',
        'projection_goals',
        'projection_projects',
        'projection_tasks',
        'projection_task_dependencies',
        'projection_task_assignments',
        'projection_task_activity',
        'projection_search_documents'
    ];
BEGIN
    FOREACH table_name IN ARRAY protected_tables
    LOOP
        EXECUTE format(
            'REVOKE ALL ON TABLE %I FROM PUBLIC',
            table_name
        );

        EXECUTE format(
            'GRANT SELECT, INSERT, UPDATE, DELETE
             ON TABLE %I
             TO gereh_projection_app',
            table_name
        );

        EXECUTE format(
            'ALTER TABLE %I
             ENABLE ROW LEVEL SECURITY',
            table_name
        );

        EXECUTE format(
            'ALTER TABLE %I
             FORCE ROW LEVEL SECURITY',
            table_name
        );

        EXECUTE format(
            'CREATE POLICY %I
             ON %I
             TO gereh_projection_app
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

-- Operational cross-tenant checkpoint state.
REVOKE ALL
    ON TABLE projection_partition_checkpoints
    FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE
    ON TABLE projection_partition_checkpoints
    TO gereh_projection_app;

COMMIT;