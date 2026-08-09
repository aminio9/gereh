BEGIN;

CREATE SCHEMA IF NOT EXISTS app;

REVOKE ALL ON SCHEMA app FROM PUBLIC;
GRANT USAGE ON SCHEMA app TO gereh_policy_app;

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
    TO gereh_policy_app;

GRANT EXECUTE ON FUNCTION app.current_tenant_id()
    TO gereh_policy_app;

GRANT EXECUTE ON FUNCTION app.current_principal_id()
    TO gereh_policy_app;

GRANT EXECUTE ON FUNCTION app.current_principal_type()
    TO gereh_policy_app;

CREATE TABLE policy_sets (
    tenant_id UUID NOT NULL,
    policy_id UUID NOT NULL,

    scope_type TEXT NOT NULL,
    scope_id UUID,

    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    status TEXT NOT NULL DEFAULT 'draft',

    active_policy_version BIGINT,
    resource_version BIGINT NOT NULL DEFAULT 1,

    created_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, policy_id),

    CHECK (
        scope_type IN (
            'tenant',
            'company',
            'agent'
        )
    ),

    CHECK (
        (
            scope_type = 'tenant'
            AND scope_id IS NULL
        )
        OR
        (
            scope_type IN ('company', 'agent')
            AND scope_id IS NOT NULL
        )
    ),

    CHECK (
        status IN (
            'draft',
            'active',
            'archived'
        )
    ),

    CHECK (
        char_length(name) BETWEEN 1 AND 120
    ),

    CHECK (
        char_length(description) <= 4000
    ),

    CHECK (resource_version > 0),

    CHECK (
        (
            status = 'active'
            AND active_policy_version IS NOT NULL
            AND archived_at IS NULL
        )
        OR
        (
            status = 'draft'
            AND archived_at IS NULL
        )
        OR
        (
            status = 'archived'
            AND archived_at IS NOT NULL
        )
    )
);

CREATE INDEX policy_sets_active_scope_idx
    ON policy_sets (
        tenant_id,
        scope_type,
        scope_id,
        policy_id
    )
    WHERE status = 'active';

CREATE TABLE policy_versions (
    tenant_id UUID NOT NULL,
    policy_id UUID NOT NULL,
    policy_version BIGINT NOT NULL,

    default_effect TEXT NOT NULL,

    notes TEXT NOT NULL DEFAULT '',
    created_by_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL,
    activated_at TIMESTAMPTZ,

    PRIMARY KEY (
        tenant_id,
        policy_id,
        policy_version
    ),

    FOREIGN KEY (
        tenant_id,
        policy_id
    )
    REFERENCES policy_sets (
        tenant_id,
        policy_id
    )
    ON DELETE RESTRICT,

    CHECK (policy_version > 0),

    CHECK (
        default_effect IN (
            'deny',
            'require_approval'
        )
    ),

    CHECK (char_length(notes) <= 4000)
);

ALTER TABLE policy_sets
    ADD CONSTRAINT policy_sets_active_version_fk
    FOREIGN KEY (
        tenant_id,
        policy_id,
        active_policy_version
    )
    REFERENCES policy_versions (
        tenant_id,
        policy_id,
        policy_version
    )
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE policy_rules (
    tenant_id UUID NOT NULL,
    policy_id UUID NOT NULL,
    policy_version BIGINT NOT NULL,
    rule_id UUID NOT NULL,

    priority INTEGER NOT NULL,

    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    effect TEXT NOT NULL,

    action_patterns TEXT[] NOT NULL,
    resource_types TEXT[] NOT NULL DEFAULT '{}',
    risk_levels TEXT[] NOT NULL DEFAULT '{}',

    maximum_estimated_cost_micro_usd BIGINT,

    condition TEXT NOT NULL DEFAULT 'true',
    constraints JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason TEXT NOT NULL,

    PRIMARY KEY (
        tenant_id,
        policy_id,
        policy_version,
        rule_id
    ),

    FOREIGN KEY (
        tenant_id,
        policy_id,
        policy_version
    )
    REFERENCES policy_versions (
        tenant_id,
        policy_id,
        policy_version
    )
    ON DELETE CASCADE,

    CHECK (priority BETWEEN 0 AND 100000),

    CHECK (char_length(name) BETWEEN 1 AND 120),

    CHECK (
        effect IN (
            'allow',
            'deny',
            'require_approval',
            'allow_with_constraints'
        )
    ),

    CHECK (
        cardinality(action_patterns)
        BETWEEN 1 AND 64
    ),

    CHECK (cardinality(resource_types) <= 64),
    CHECK (cardinality(risk_levels) <= 5),

    CHECK (
        maximum_estimated_cost_micro_usd IS NULL
        OR maximum_estimated_cost_micro_usd >= 0
    ),

    CHECK (
        char_length(condition)
        BETWEEN 1 AND 4096
    ),

    CHECK (
        jsonb_typeof(constraints) = 'object'
        AND pg_column_size(constraints) <= 16384
    ),

    CHECK (char_length(reason) BETWEEN 1 AND 1000)
);

CREATE INDEX policy_rules_evaluation_idx
    ON policy_rules (
        tenant_id,
        policy_id,
        policy_version,
        priority,
        rule_id
    )
    WHERE enabled;

CREATE TABLE policy_decisions (
    tenant_id UUID NOT NULL,
    decision_id UUID NOT NULL,
    evaluation_request_id UUID NOT NULL,

    caller_service TEXT NOT NULL,

    subject_type TEXT NOT NULL,
    subject_id UUID NOT NULL,
    company_id UUID,

    action TEXT NOT NULL,

    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    resource_attributes JSONB NOT NULL DEFAULT '{}'::jsonb,

    risk TEXT NOT NULL,
    estimated_cost_micro_usd BIGINT NOT NULL,

    effect TEXT NOT NULL,
    constraints JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason TEXT NOT NULL,

    matched_policy_id UUID,
    matched_policy_version BIGINT,
    matched_rule_id UUID,

    input_hash BYTEA NOT NULL,

    decided_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,

    signing_key_id TEXT NOT NULL,
    signature BYTEA NOT NULL,

    PRIMARY KEY (tenant_id, decision_id),

    UNIQUE (
        tenant_id,
        evaluation_request_id
    ),

    CHECK (
        subject_type IN (
            'user',
            'agent',
            'service'
        )
    ),

    CHECK (
        risk IN (
            'low',
            'medium',
            'high',
            'critical'
        )
    ),

    CHECK (
        effect IN (
            'allow',
            'deny',
            'require_approval',
            'allow_with_constraints'
        )
    ),

    CHECK (estimated_cost_micro_usd >= 0),

    CHECK (
        jsonb_typeof(resource_attributes) = 'object'
        AND pg_column_size(resource_attributes) <= 65536
    ),

    CHECK (
        jsonb_typeof(constraints) = 'object'
        AND pg_column_size(constraints) <= 16384
    ),

    CHECK (expires_at > decided_at),

    CHECK (octet_length(input_hash) = 32),
    CHECK (octet_length(signature) = 32)
);

CREATE INDEX policy_decisions_subject_idx
    ON policy_decisions (
        tenant_id,
        subject_id,
        decided_at DESC,
        decision_id DESC
    );

CREATE INDEX policy_decisions_expiry_idx
    ON policy_decisions (
        expires_at
    );

CREATE TABLE policy_bootstrap_requests (
    onboarding_operation_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL UNIQUE,

    created_policy_ids UUID[] NOT NULL,

    actor_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,

    CHECK (
        cardinality(created_policy_ids) > 0
    )
);

CREATE TABLE policy_outbox (
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

CREATE INDEX policy_outbox_pending_idx
    ON policy_outbox (
        available_at,
        outbox_id
    )
    WHERE published_at IS NULL;

CREATE INDEX policy_outbox_tenant_idx
    ON policy_outbox (
        tenant_id,
        occurred_at DESC,
        outbox_id DESC
    );

DO $block$
DECLARE
    table_name TEXT;
    protected_tables TEXT[] := ARRAY[
        'policy_sets',
        'policy_versions',
        'policy_rules',
        'policy_decisions',
        'policy_bootstrap_requests'
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
             ON TABLE %I TO gereh_policy_app',
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
             TO gereh_policy_app
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

-- Runtime code does not need hard-delete permission on business history.
REVOKE DELETE
    ON TABLE policy_versions,
             policy_rules,
             policy_decisions,
             policy_bootstrap_requests
    FROM gereh_policy_app;

REVOKE ALL ON TABLE policy_outbox FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE
    ON TABLE policy_outbox
    TO gereh_policy_app;

GRANT USAGE, SELECT
    ON SEQUENCE policy_outbox_outbox_id_seq
    TO gereh_policy_app;

COMMIT;