BEGIN;

CREATE SCHEMA IF NOT EXISTS app;

REVOKE ALL ON SCHEMA app FROM PUBLIC;
GRANT USAGE ON SCHEMA app TO gereh_work_app;

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
    TO gereh_work_app;
GRANT EXECUTE ON FUNCTION app.current_tenant_id()
    TO gereh_work_app;
GRANT EXECUTE ON FUNCTION app.current_principal_id()
    TO gereh_work_app;
GRANT EXECUTE ON FUNCTION app.current_principal_type()
    TO gereh_work_app;

CREATE TABLE work_goals (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    goal_id UUID NOT NULL,

    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',

    version BIGINT NOT NULL DEFAULT 1,
    created_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, goal_id),

    CONSTRAINT work_goals_company_identity_unique
        UNIQUE (
            tenant_id,
            company_id,
            goal_id
        ),

    CONSTRAINT work_goals_title_check
        CHECK (char_length(title) BETWEEN 1 AND 200),

    CONSTRAINT work_goals_description_check
        CHECK (char_length(description) <= 8000),

    CONSTRAINT work_goals_status_check
        CHECK (
            status IN (
                'active',
                'completed',
                'canceled',
                'archived'
            )
        ),

    CONSTRAINT work_goals_version_check
        CHECK (version > 0),

    CONSTRAINT work_goals_lifecycle_check
        CHECK (
            (
                status = 'completed'
                AND completed_at IS NOT NULL
                AND archived_at IS NULL
            )
            OR
            (
                status = 'archived'
                AND archived_at IS NOT NULL
            )
            OR
            (
                status IN ('active', 'canceled')
                AND completed_at IS NULL
                AND archived_at IS NULL
            )
        )
);

CREATE INDEX work_goals_list_idx
    ON work_goals (
        tenant_id,
        company_id,
        goal_id
    );

CREATE TABLE work_projects (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    goal_id UUID NOT NULL,
    project_id UUID NOT NULL,

    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'planned',

    version BIGINT NOT NULL DEFAULT 1,
    created_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, project_id),

    CONSTRAINT work_projects_company_identity_unique
        UNIQUE (
            tenant_id,
            company_id,
            goal_id,
            project_id
        ),

    CONSTRAINT work_projects_project_identity_unique
        UNIQUE (
            tenant_id,
            company_id,
            project_id
        ),

    CONSTRAINT work_projects_goal_fk
        FOREIGN KEY (
            tenant_id,
            company_id,
            goal_id
        )
        REFERENCES work_goals (
            tenant_id,
            company_id,
            goal_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT work_projects_title_check
        CHECK (char_length(title) BETWEEN 1 AND 200),

    CONSTRAINT work_projects_description_check
        CHECK (char_length(description) <= 8000),

    CONSTRAINT work_projects_status_check
        CHECK (
            status IN (
                'planned',
                'active',
                'on_hold',
                'completed',
                'canceled',
                'archived'
            )
        ),

    CONSTRAINT work_projects_version_check
        CHECK (version > 0),

    CONSTRAINT work_projects_lifecycle_check
        CHECK (
            (
                status = 'completed'
                AND completed_at IS NOT NULL
                AND archived_at IS NULL
            )
            OR
            (
                status = 'archived'
                AND archived_at IS NOT NULL
            )
            OR
            (
                status IN (
                    'planned',
                    'active',
                    'on_hold',
                    'canceled'
                )
                AND completed_at IS NULL
                AND archived_at IS NULL
            )
        )
);

CREATE INDEX work_projects_list_idx
    ON work_projects (
        tenant_id,
        company_id,
        goal_id,
        project_id
    );

CREATE TABLE work_tasks (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    project_id UUID NOT NULL,
    task_id UUID NOT NULL,
    parent_task_id UUID,

    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    status TEXT NOT NULL DEFAULT 'backlog',
    priority TEXT NOT NULL DEFAULT 'normal',

    version BIGINT NOT NULL DEFAULT 1,
    created_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, task_id),

    CONSTRAINT work_tasks_project_identity_unique
        UNIQUE (
            tenant_id,
            project_id,
            task_id
        ),

    CONSTRAINT work_tasks_company_identity_unique
        UNIQUE (
            tenant_id,
            company_id,
            project_id,
            task_id
        ),

    CONSTRAINT work_tasks_project_fk
        FOREIGN KEY (
            tenant_id,
            company_id,
            project_id
        )
        REFERENCES work_projects (
            tenant_id,
            company_id,
            project_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT work_tasks_parent_fk
        FOREIGN KEY (
            tenant_id,
            company_id,
            project_id,
            parent_task_id
        )
        REFERENCES work_tasks (
            tenant_id,
            company_id,
            project_id,
            task_id
        )
        ON DELETE RESTRICT
        DEFERRABLE INITIALLY IMMEDIATE,

    CONSTRAINT work_tasks_not_self_parent
        CHECK (
            parent_task_id IS NULL
            OR parent_task_id <> task_id
        ),

    CONSTRAINT work_tasks_title_check
        CHECK (char_length(title) BETWEEN 1 AND 300),

    CONSTRAINT work_tasks_description_check
        CHECK (char_length(description) <= 32000),

    CONSTRAINT work_tasks_status_check
        CHECK (
            status IN (
                'backlog',
                'ready',
                'in_progress',
                'waiting_approval',
                'completed',
                'canceled'
            )
        ),

    CONSTRAINT work_tasks_priority_check
        CHECK (
            priority IN (
                'low',
                'normal',
                'high',
                'urgent'
            )
        ),

    CONSTRAINT work_tasks_version_check
        CHECK (version > 0),

    CONSTRAINT work_tasks_lifecycle_check
        CHECK (
            (
                status = 'completed'
                AND completed_at IS NOT NULL
                AND canceled_at IS NULL
            )
            OR
            (
                status = 'canceled'
                AND canceled_at IS NOT NULL
                AND completed_at IS NULL
            )
            OR
            (
                status NOT IN ('completed', 'canceled')
                AND completed_at IS NULL
                AND canceled_at IS NULL
            )
        )
);

CREATE INDEX work_tasks_project_list_idx
    ON work_tasks (
        tenant_id,
        company_id,
        project_id,
        task_id
    );

CREATE INDEX work_tasks_parent_idx
    ON work_tasks (
        tenant_id,
        project_id,
        parent_task_id
    )
    WHERE parent_task_id IS NOT NULL;

CREATE INDEX work_tasks_status_idx
    ON work_tasks (
        tenant_id,
        company_id,
        status,
        priority,
        task_id
    );

CREATE TABLE work_task_dependencies (
    tenant_id UUID NOT NULL,
    project_id UUID NOT NULL,
    task_id UUID NOT NULL,
    depends_on_task_id UUID NOT NULL,

    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (
        tenant_id,
        task_id,
        depends_on_task_id
    ),

    CONSTRAINT work_task_dependencies_task_fk
        FOREIGN KEY (
            tenant_id,
            project_id,
            task_id
        )
        REFERENCES work_tasks (
            tenant_id,
            project_id,
            task_id
        )
        ON DELETE CASCADE,

    CONSTRAINT work_task_dependencies_prerequisite_fk
        FOREIGN KEY (
            tenant_id,
            project_id,
            depends_on_task_id
        )
        REFERENCES work_tasks (
            tenant_id,
            project_id,
            task_id
        )
        ON DELETE RESTRICT,

    CONSTRAINT work_task_dependencies_not_self
        CHECK (task_id <> depends_on_task_id)
);

CREATE INDEX work_dependencies_prerequisite_idx
    ON work_task_dependencies (
        tenant_id,
        project_id,
        depends_on_task_id
    );

CREATE TABLE work_task_assignments (
    tenant_id UUID NOT NULL,
    task_id UUID NOT NULL,
    assignment_id UUID NOT NULL,

    assignee_type TEXT NOT NULL,
    user_id UUID,
    agent_id UUID,
    assignment_role TEXT NOT NULL,

    assigned_by_user_id UUID NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (tenant_id, assignment_id),

    CONSTRAINT work_task_assignments_task_fk
        FOREIGN KEY (tenant_id, task_id)
        REFERENCES work_tasks (tenant_id, task_id)
        ON DELETE CASCADE,

    CONSTRAINT work_task_assignments_assignee_type_check
        CHECK (
            assignee_type IN ('user', 'agent')
        ),

    CONSTRAINT work_task_assignments_role_check
        CHECK (
            assignment_role IN (
                'primary',
                'collaborator',
                'reviewer'
            )
        ),

    CONSTRAINT work_task_assignments_assignee_check
        CHECK (
            (
                assignee_type = 'user'
                AND user_id IS NOT NULL
                AND agent_id IS NULL
            )
            OR
            (
                assignee_type = 'agent'
                AND agent_id IS NOT NULL
                AND user_id IS NULL
            )
        )
);

CREATE UNIQUE INDEX work_assignment_user_unique
    ON work_task_assignments (
        tenant_id,
        task_id,
        user_id
    )
    WHERE user_id IS NOT NULL;

CREATE UNIQUE INDEX work_assignment_agent_unique
    ON work_task_assignments (
        tenant_id,
        task_id,
        agent_id
    )
    WHERE agent_id IS NOT NULL;

CREATE UNIQUE INDEX work_assignment_primary_unique
    ON work_task_assignments (
        tenant_id,
        task_id
    )
    WHERE assignment_role = 'primary';

CREATE TABLE work_comments (
    tenant_id UUID NOT NULL,
    task_id UUID NOT NULL,
    comment_id UUID NOT NULL,

    author_type TEXT NOT NULL,
    author_user_id UUID,
    author_agent_id UUID,

    body TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, comment_id),

    CONSTRAINT work_comments_task_fk
        FOREIGN KEY (tenant_id, task_id)
        REFERENCES work_tasks (tenant_id, task_id)
        ON DELETE CASCADE,

    CONSTRAINT work_comments_author_type_check
        CHECK (
            author_type IN ('user', 'agent')
        ),

    CONSTRAINT work_comments_author_check
        CHECK (
            (
                author_type = 'user'
                AND author_user_id IS NOT NULL
                AND author_agent_id IS NULL
            )
            OR
            (
                author_type = 'agent'
                AND author_agent_id IS NOT NULL
                AND author_user_id IS NULL
            )
        ),

    CONSTRAINT work_comments_body_check
        CHECK (char_length(body) BETWEEN 1 AND 16000),

    CONSTRAINT work_comments_version_check
        CHECK (version > 0)
);

CREATE INDEX work_comments_task_idx
    ON work_comments (
        tenant_id,
        task_id,
        created_at,
        comment_id
    );

CREATE TABLE work_artifacts (
    tenant_id UUID NOT NULL,
    company_id UUID NOT NULL,
    task_id UUID NOT NULL,
    artifact_id UUID NOT NULL,

    object_key TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, artifact_id),

    CONSTRAINT work_artifacts_object_key_unique
        UNIQUE (tenant_id, object_key),

    CONSTRAINT work_artifacts_task_fk
        FOREIGN KEY (tenant_id, task_id)
        REFERENCES work_tasks (tenant_id, task_id)
        ON DELETE RESTRICT,

    CONSTRAINT work_artifacts_file_name_check
        CHECK (char_length(file_name) BETWEEN 1 AND 512),

    CONSTRAINT work_artifacts_content_type_check
        CHECK (char_length(content_type) BETWEEN 1 AND 255),

    CONSTRAINT work_artifacts_size_check
        CHECK (size_bytes >= 0),

    CONSTRAINT work_artifacts_sha256_check
        CHECK (sha256 ~ '^[a-f0-9]{64}$'),

    CONSTRAINT work_artifacts_metadata_check
        CHECK (
            jsonb_typeof(metadata) = 'object'
            AND pg_column_size(metadata) <= 65536
        )
);

CREATE INDEX work_artifacts_task_idx
    ON work_artifacts (
        tenant_id,
        task_id,
        artifact_id
    )
    WHERE deleted_at IS NULL;

CREATE TABLE work_checklist_items (
    tenant_id UUID NOT NULL,
    task_id UUID NOT NULL,
    item_id UUID NOT NULL,

    title TEXT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    position INTEGER NOT NULL DEFAULT 0,

    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (tenant_id, item_id),

    CONSTRAINT work_checklist_items_task_fk
        FOREIGN KEY (tenant_id, task_id)
        REFERENCES work_tasks (tenant_id, task_id)
        ON DELETE CASCADE,

    CONSTRAINT work_checklist_items_title_check
        CHECK (char_length(title) BETWEEN 1 AND 500),

    CONSTRAINT work_checklist_items_position_check
        CHECK (position >= 0),

    CONSTRAINT work_checklist_items_version_check
        CHECK (version > 0)
);

CREATE INDEX work_checklist_task_idx
    ON work_checklist_items (
        tenant_id,
        task_id,
        position,
        item_id
    );

CREATE TABLE work_task_schedules (
    tenant_id UUID NOT NULL,
    task_id UUID NOT NULL,

    not_before TIMESTAMPTZ,
    due_at TIMESTAMPTZ,
    timezone TEXT NOT NULL DEFAULT 'UTC',

    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (tenant_id, task_id),

    CONSTRAINT work_task_schedules_task_fk
        FOREIGN KEY (tenant_id, task_id)
        REFERENCES work_tasks (tenant_id, task_id)
        ON DELETE CASCADE,

    CONSTRAINT work_task_schedules_window_check
        CHECK (
            not_before IS NULL
            OR due_at IS NULL
            OR not_before <= due_at
        ),

    CONSTRAINT work_task_schedules_timezone_check
        CHECK (char_length(timezone) BETWEEN 1 AND 64),

    CONSTRAINT work_task_schedules_version_check
        CHECK (version > 0)
);

CREATE TABLE work_outbox (
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

    CONSTRAINT work_outbox_attempts_check
        CHECK (attempts >= 0)
);

CREATE INDEX work_outbox_pending_idx
    ON work_outbox (
        available_at,
        outbox_id
    )
    WHERE published_at IS NULL;

CREATE INDEX work_outbox_tenant_idx
    ON work_outbox (
        tenant_id,
        occurred_at DESC,
        outbox_id DESC
    );

DO $block$
DECLARE
    table_name TEXT;
    protected_tables TEXT[] := ARRAY[
        'work_goals',
        'work_projects',
        'work_tasks',
        'work_task_dependencies',
        'work_task_assignments',
        'work_comments',
        'work_artifacts',
        'work_checklist_items',
        'work_task_schedules'
    ];
BEGIN
    FOREACH table_name IN ARRAY protected_tables
    LOOP
        EXECUTE format(
            'REVOKE ALL ON TABLE %I FROM PUBLIC',
            table_name
        );

        EXECUTE format(
            'GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE %I TO gereh_work_app',
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
            'CREATE POLICY %I ON %I
             TO gereh_work_app
             USING (
                app.scope_kind() = ''tenant''
                AND tenant_id = app.current_tenant_id()
             )
             WITH CHECK (
                app.scope_kind() = ''tenant''
                AND tenant_id = app.current_tenant_id()
             )',
            table_name || '_tenant_isolation',
            table_name
        );
    END LOOP;
END
$block$;

REVOKE ALL ON TABLE work_outbox FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE
    ON TABLE work_outbox
    TO gereh_work_app;

GRANT USAGE, SELECT
    ON SEQUENCE work_outbox_outbox_id_seq
    TO gereh_work_app;

COMMIT;
