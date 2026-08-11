BEGIN;

-- ---------------------------------------------------------------------------
-- Gereh-managed provider pools
--
-- These rows contain routing/control-plane metadata only.
-- They MUST NOT contain provider credentials.
--
-- Actual provider secrets remain outside ordinary Model Access business rows
-- and are resolved by the Model Gateway in its later implementation phase.
-- ---------------------------------------------------------------------------

CREATE TABLE model_access_provider_pools (
    pool_key TEXT PRIMARY KEY,

    provider_key TEXT NOT NULL,

    regions TEXT[] NOT NULL
        DEFAULT ARRAY['*']::TEXT[],

    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Higher value wins among otherwise equivalent pools.
    priority INTEGER NOT NULL DEFAULT 0,

    version BIGINT NOT NULL DEFAULT 1,

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    FOREIGN KEY (provider_key)
    REFERENCES model_access_providers (
        provider_key
    )
    ON UPDATE RESTRICT
    ON DELETE RESTRICT,

    CHECK (
        pool_key ~
        '^[a-z0-9][a-z0-9._-]{0,127}$'
    ),

    CHECK (
        cardinality(regions) > 0
    ),

    CHECK (
        array_position(
            regions,
            ''
        ) IS NULL
    ),

    CHECK (
        priority >= 0
    ),

    CHECK (
        version > 0
    )
);

CREATE INDEX
    model_access_provider_pools_selection_idx
ON model_access_provider_pools (
    provider_key,
    enabled,
    priority DESC,
    pool_key
);

-- ---------------------------------------------------------------------------
-- Initial logical Gereh provider pools.
--
-- "*" means the pool is eligible for every tenant region.
--
-- These are logical pool identities, NOT provider credentials.
-- ---------------------------------------------------------------------------

INSERT INTO model_access_provider_pools (
    pool_key,
    provider_key,
    regions,
    enabled,
    priority,
    version,
    created_at,
    updated_at
)
VALUES
(
    'gereh-openai-global',
    'openai',
    ARRAY['*']::TEXT[],
    TRUE,
    100,
    1,
    clock_timestamp(),
    clock_timestamp()
),
(
    'gereh-anthropic-global',
    'anthropic',
    ARRAY['*']::TEXT[],
    TRUE,
    100,
    1,
    clock_timestamp(),
    clock_timestamp()
),
(
    'gereh-google-global',
    'google',
    ARRAY['*']::TEXT[],
    TRUE,
    100,
    1,
    clock_timestamp(),
    clock_timestamp()
),
(
    'gereh-openrouter-global',
    'openrouter',
    ARRAY['*']::TEXT[],
    TRUE,
    100,
    1,
    clock_timestamp(),
    clock_timestamp()
);

-- Provider-pool configuration is platform-owned metadata.
-- The Model Access runtime can resolve pools but cannot mutate them.

REVOKE ALL
    ON TABLE model_access_provider_pools
    FROM PUBLIC;

GRANT SELECT
    ON TABLE model_access_provider_pools
    TO gereh_model_access_app;

-- ---------------------------------------------------------------------------
-- Bind tenant connection → platform provider pool.
--
-- This field is deliberately internal and is NOT added to the public Protobuf
-- ModelConnection.
-- ---------------------------------------------------------------------------

ALTER TABLE model_access_connections
    ADD COLUMN provider_pool_key TEXT;

ALTER TABLE model_access_connections
    ADD CONSTRAINT
        model_access_connections_provider_pool_fk
    FOREIGN KEY (
        provider_pool_key
    )
    REFERENCES model_access_provider_pools (
        pool_key
    )
    ON UPDATE RESTRICT
    ON DELETE RESTRICT;

-- Phase-16 legacy platform-managed draft rows may have no pool.
--
-- New platform-managed connections created after this migration become ACTIVE
-- and must have a provider pool.
--
-- A legacy draft may also be archived without acquiring a pool first.

ALTER TABLE model_access_connections
    ADD CONSTRAINT
        model_access_connections_platform_pool_check
    CHECK (
        (
            connection_type = 'platform_managed'
            AND
            (
                (
                    provider_pool_key IS NULL
                    AND status IN (
                        'draft',
                        'archived'
                    )
                )
                OR
                (
                    provider_pool_key IS NOT NULL
                    AND status IN (
                        'active',
                        'disabled',
                        'archived'
                    )
                )
            )
        )
        OR
        (
            connection_type <> 'platform_managed'
            AND provider_pool_key IS NULL
        )
    );

CREATE INDEX
    model_access_connections_provider_pool_idx
ON model_access_connections (
    tenant_id,
    provider_pool_key,
    status,
    connection_id
)
WHERE provider_pool_key IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Immutable revision history also records the pool selection.
--
-- There is intentionally no FK here: historical revisions should remain
-- readable if a pool is later retired by a platform migration.
-- ---------------------------------------------------------------------------

ALTER TABLE model_access_connection_revisions
    ADD COLUMN provider_pool_key TEXT;

COMMIT;
