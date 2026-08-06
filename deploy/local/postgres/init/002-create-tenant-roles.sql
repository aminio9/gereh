\set ON_ERROR_STOP on

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_tenant_migrator'
    ) THEN
        CREATE ROLE gereh_tenant_migrator LOGIN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_tenant_app'
    ) THEN
        CREATE ROLE gereh_tenant_app LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_tenant_migrator WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-tenant-migrator-local-only';

ALTER ROLE gereh_tenant_app WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-tenant-app-local-only';

ALTER DATABASE tenant_db
    OWNER TO gereh_tenant_migrator;

REVOKE ALL
    ON DATABASE tenant_db
    FROM PUBLIC;

GRANT CONNECT, TEMPORARY
    ON DATABASE tenant_db
    TO gereh_tenant_migrator;

GRANT CONNECT
    ON DATABASE tenant_db
    TO gereh_tenant_app;

\connect tenant_db

REVOKE CREATE
    ON SCHEMA public
    FROM PUBLIC;

GRANT USAGE, CREATE
    ON SCHEMA public
    TO gereh_tenant_migrator;

GRANT USAGE
    ON SCHEMA public
    TO gereh_tenant_app;