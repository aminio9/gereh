\set ON_ERROR_STOP on

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_organization_migrator'
    ) THEN
        CREATE ROLE gereh_organization_migrator LOGIN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_organization_app'
    ) THEN
        CREATE ROLE gereh_organization_app LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_organization_migrator WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-organization-migrator-local-only';

ALTER ROLE gereh_organization_app WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-organization-app-local-only';

ALTER DATABASE organization_db
    OWNER TO gereh_organization_migrator;

REVOKE ALL
    ON DATABASE organization_db
    FROM PUBLIC;

GRANT CONNECT, TEMPORARY
    ON DATABASE organization_db
    TO gereh_organization_migrator;

GRANT CONNECT
    ON DATABASE organization_db
    TO gereh_organization_app;

\connect organization_db

REVOKE CREATE
    ON SCHEMA public
    FROM PUBLIC;

GRANT USAGE, CREATE
    ON SCHEMA public
    TO gereh_organization_migrator;

GRANT USAGE
    ON SCHEMA public
    TO gereh_organization_app;
