BEGIN;

ALTER TABLE model_access_connection_revisions
    DROP COLUMN IF EXISTS provider_pool_key;

DROP INDEX IF EXISTS
    model_access_connections_provider_pool_idx;

ALTER TABLE model_access_connections
    DROP CONSTRAINT IF EXISTS
        model_access_connections_platform_pool_check;

ALTER TABLE model_access_connections
    DROP CONSTRAINT IF EXISTS
        model_access_connections_provider_pool_fk;

ALTER TABLE model_access_connections
    DROP COLUMN IF EXISTS provider_pool_key;

DROP TABLE IF EXISTS
    model_access_provider_pools;

COMMIT;
