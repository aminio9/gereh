\set ON_ERROR_STOP on

-- Local-only test administration role.
--
-- Integration tests that exercise operator-owned provider pools need
-- destructive changes, but the runtime role (gereh_model_access_app)
-- intentionally holds only SELECT on model_access_provider_pools
-- (least privilege). This role exists only for tests and must never be
-- used by a deployed service.

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_model_access_test_admin'
    ) THEN
        CREATE ROLE gereh_model_access_test_admin LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_model_access_test_admin WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-model-access-test-admin-local-only';

GRANT CONNECT, TEMPORARY
    ON DATABASE model_access_db
    TO gereh_model_access_test_admin;

\connect model_access_db

GRANT USAGE
    ON SCHEMA public
    TO gereh_model_access_test_admin;

-- Grant pool mutation only if tables exist (migrations may not have run yet).
DO $block$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'model_access_provider_pools'
    ) THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON TABLE model_access_provider_pools
            TO gereh_model_access_test_admin;
    END IF;
END
$block$;
