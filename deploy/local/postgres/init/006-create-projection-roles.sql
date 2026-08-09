\set ON_ERROR_STOP on

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_projection_migrator'
    ) THEN
        CREATE ROLE gereh_projection_migrator LOGIN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_projection_app'
    ) THEN
        CREATE ROLE gereh_projection_app LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_projection_migrator WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-projection-migrator-local-only';

ALTER ROLE gereh_projection_app WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-projection-app-local-only';

ALTER DATABASE projection_db
    OWNER TO gereh_projection_migrator;

REVOKE ALL
    ON DATABASE projection_db
    FROM PUBLIC;

GRANT CONNECT, TEMPORARY
    ON DATABASE projection_db
    TO gereh_projection_migrator;

GRANT CONNECT
    ON DATABASE projection_db
    TO gereh_projection_app;

\connect projection_db

REVOKE CREATE
    ON SCHEMA public
    FROM PUBLIC;

GRANT USAGE, CREATE
    ON SCHEMA public
    TO gereh_projection_migrator;

GRANT USAGE
    ON SCHEMA public
    TO gereh_projection_app;