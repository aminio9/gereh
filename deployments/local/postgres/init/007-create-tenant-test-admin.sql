\set ON_ERROR_STOP on

-- Local-only test administration role.
--
-- Integration tests that exercise durable tenant tables need destructive
-- cleanup, but the runtime role (gereh_tenant_app) intentionally lacks
-- DELETE after the least-privilege migration. This role exists only for
-- tests: it bypasses RLS (FORCE ROW LEVEL SECURITY would otherwise hide
-- rows even from the table owner) and holds no runtime responsibilities.
-- It must never be used by a deployed service.

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_tenant_test_admin'
    ) THEN
        CREATE ROLE gereh_tenant_test_admin LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_tenant_test_admin WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    BYPASSRLS
    PASSWORD 'gereh-tenant-test-admin-local-only';

GRANT CONNECT, TEMPORARY
    ON DATABASE tenant_db
    TO gereh_tenant_test_admin;

\connect tenant_db

GRANT USAGE
    ON SCHEMA public
    TO gereh_tenant_test_admin;

-- Grant table permissions only if tables exist (migrations may not have run yet).
DO $block$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'tenant_tenants'
    ) THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON TABLE tenant_tenants
            TO gereh_tenant_test_admin;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'tenant_memberships'
    ) THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON TABLE tenant_memberships
            TO gereh_tenant_test_admin;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'tenant_entitlements'
    ) THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON TABLE tenant_entitlements
            TO gereh_tenant_test_admin;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'tenant_outbox'
    ) THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON TABLE tenant_outbox
            TO gereh_tenant_test_admin;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'tenant_onboarding_operations'
    ) THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
            ON TABLE tenant_onboarding_operations
            TO gereh_tenant_test_admin;
    END IF;
END
$block$;