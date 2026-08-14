BEGIN;

CREATE SCHEMA IF NOT EXISTS app;

REVOKE ALL ON SCHEMA app FROM PUBLIC;

GRANT USAGE
    ON SCHEMA app
    TO gereh_model_gateway_app;

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

-- ---------------------------------------------------------------------------
-- Model Gateway Request Journal
--
-- Never contains raw prompt content, completions, tool arguments, or API keys.
-- ---------------------------------------------------------------------------

CREATE TABLE model_gateway_request_journal (
    tenant_id UUID NOT NULL,
    request_id TEXT NOT NULL,

    agent_id UUID NOT NULL,
    execution_id UUID NOT NULL,
    workflow_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    step_id TEXT NOT NULL,

    connection_id UUID NOT NULL,
    offering_id UUID NOT NULL,
    provider_key TEXT NOT NULL,
    provider_model_id TEXT NOT NULL,

    status TEXT NOT NULL,

    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    cached_prompt_tokens BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens BIGINT NOT NULL DEFAULT 0,

    estimated_cost_microusd BIGINT NOT NULL DEFAULT 0,

    error_code TEXT NOT NULL DEFAULT '',

    streamed BOOLEAN NOT NULL DEFAULT FALSE,
    retry_count INTEGER NOT NULL DEFAULT 0,
    fallback_from_offering_id UUID,

    duration_ms BIGINT NOT NULL DEFAULT 0,
    time_to_first_token_ms BIGINT NOT NULL DEFAULT 0,

    admitted_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,

    PRIMARY KEY (
        tenant_id,
        request_id
    ),

    CHECK (
        char_length(request_id) BETWEEN 1 AND 256
    ),

    CHECK (
        status IN (
            'admitted',
            'succeeded',
            'failed',
            'client_disconnected'
        )
    ),

    CHECK (prompt_tokens >= 0),
    CHECK (completion_tokens >= 0),
    CHECK (total_tokens >= 0),
    CHECK (cached_prompt_tokens >= 0),
    CHECK (reasoning_tokens >= 0),
    CHECK (estimated_cost_microusd >= 0),
    CHECK (duration_ms >= 0),
    CHECK (time_to_first_token_ms >= 0),
    CHECK (retry_count >= 0),
    CHECK (char_length(error_code) <= 64)
);

CREATE INDEX
    model_gateway_request_journal_agent_idx
ON model_gateway_request_journal (
    tenant_id,
    agent_id,
    admitted_at DESC
);

CREATE INDEX
    model_gateway_request_journal_execution_idx
ON model_gateway_request_journal (
    tenant_id,
    execution_id,
    admitted_at DESC
);

-- ---------------------------------------------------------------------------
-- Transactional Outbox
-- ---------------------------------------------------------------------------

CREATE TABLE model_gateway_outbox (
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
    model_gateway_outbox_pending_idx
ON model_gateway_outbox (
    available_at,
    outbox_id
)
WHERE published_at IS NULL;

CREATE INDEX
    model_gateway_outbox_tenant_idx
ON model_gateway_outbox (
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
        'model_gateway_request_journal'
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
             TO gereh_model_gateway_app
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
    ON TABLE model_gateway_request_journal
    TO gereh_model_gateway_app;

REVOKE ALL
    ON TABLE model_gateway_outbox
    FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE
    ON TABLE model_gateway_outbox
    TO gereh_model_gateway_app;

GRANT USAGE, SELECT
    ON SEQUENCE
        model_gateway_outbox_outbox_id_seq
    TO gereh_model_gateway_app;

COMMIT;
