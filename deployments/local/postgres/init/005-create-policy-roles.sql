\set ON_ERROR_STOP on

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_policy_migrator'
    ) THEN
        CREATE ROLE gereh_policy_migrator LOGIN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_policy_app'
    ) THEN
        CREATE ROLE gereh_policy_app LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_policy_migrator WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-policy-migrator-local-only';

ALTER ROLE gereh_policy_app WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-policy-app-local-only';

ALTER DATABASE policy_db
    OWNER TO gereh_policy_migrator;

REVOKE ALL
    ON DATABASE policy_db
    FROM PUBLIC;

GRANT CONNECT, TEMPORARY
    ON DATABASE policy_db
    TO gereh_policy_migrator;

GRANT CONNECT
    ON DATABASE policy_db
    TO gereh_policy_app;

\connect policy_db

REVOKE CREATE
    ON SCHEMA public
    FROM PUBLIC;

GRANT USAGE, CREATE
    ON SCHEMA public
    TO gereh_policy_migrator;

GRANT USAGE
    ON SCHEMA public
    TO gereh_policy_app;
