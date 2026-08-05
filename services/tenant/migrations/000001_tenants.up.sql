BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE tenant_tenants (
    tenant_id UUID PRIMARY KEY,
    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    region TEXT NOT NULL,
    retention_days INTEGER NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_by_user_id UUID NOT NULL,
    creation_request_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    archived_at TIMESTAMPTZ,

    CONSTRAINT tenant_tenants_slug_unique
        UNIQUE (slug),

    CONSTRAINT tenant_tenants_creation_request_unique
        UNIQUE (created_by_user_id, creation_request_id),

    CONSTRAINT tenant_tenants_slug_check
        CHECK (
            slug = lower(slug)
            AND slug ~ '^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])$'
        ),

    CONSTRAINT tenant_tenants_display_name_check
        CHECK (
            char_length(display_name) BETWEEN 1 AND 120
        ),

    CONSTRAINT tenant_tenants_status_check
        CHECK (
            status IN ('active', 'archived')
        ),

    CONSTRAINT tenant_tenants_region_check
        CHECK (
            char_length(region) BETWEEN 1 AND 64
        ),

    CONSTRAINT tenant_tenants_retention_check
        CHECK (
            retention_days BETWEEN 1 AND 3650
        ),

    CONSTRAINT tenant_tenants_version_check
        CHECK (
            version > 0
        ),

    CONSTRAINT tenant_tenants_archived_check
        CHECK (
            (status = 'active' AND archived_at IS NULL)
            OR
            (status = 'archived' AND archived_at IS NOT NULL)
        )
);

CREATE TABLE tenant_memberships (
    tenant_id UUID NOT NULL
        REFERENCES tenant_tenants(tenant_id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL,
    role TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    PRIMARY KEY (tenant_id, user_id),

    CONSTRAINT tenant_memberships_role_check
        CHECK (
            role IN ('owner', 'admin', 'member', 'viewer')
        ),

    CONSTRAINT tenant_memberships_version_check
        CHECK (
            version > 0
        )
);

CREATE INDEX tenant_memberships_user_tenant_idx
    ON tenant_memberships(user_id, tenant_id DESC);

CREATE INDEX tenant_memberships_tenant_role_idx
    ON tenant_memberships(tenant_id, role);

CREATE TABLE tenant_entitlements (
    tenant_id UUID PRIMARY KEY
        REFERENCES tenant_tenants(tenant_id)
        ON DELETE CASCADE,

    plan_key TEXT NOT NULL,
    features JSONB NOT NULL DEFAULT '{}'::jsonb,
    limits JSONB NOT NULL DEFAULT '{}'::jsonb,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT tenant_entitlements_plan_key_check
        CHECK (
            plan_key ~ '^[a-z][a-z0-9_-]{1,63}$'
        ),

    CONSTRAINT tenant_entitlements_features_object_check
        CHECK (
            jsonb_typeof(features) = 'object'
        ),

    CONSTRAINT tenant_entitlements_limits_object_check
        CHECK (
            jsonb_typeof(limits) = 'object'
        ),

    CONSTRAINT tenant_entitlements_version_check
        CHECK (
            version > 0
        )
);

CREATE TABLE tenant_outbox (
    outbox_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id UUID NOT NULL,
    topic TEXT NOT NULL,
    partition_key TEXT NOT NULL,
    envelope BYTEA NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    claimed_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,

    CONSTRAINT tenant_outbox_event_id_unique
        UNIQUE (event_id),

    CONSTRAINT tenant_outbox_attempts_check
        CHECK (attempts >= 0)
);

CREATE INDEX tenant_outbox_pending_idx
    ON tenant_outbox(available_at, outbox_id)
    WHERE published_at IS NULL;

COMMIT;