\set ON_ERROR_STOP on

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_model_access_migrator'
    ) THEN
        CREATE ROLE gereh_model_access_migrator LOGIN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_model_access_app'
    ) THEN
        CREATE ROLE gereh_model_access_app LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_model_access_migrator WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-model-access-migrator-local-only';

ALTER ROLE gereh_model_access_app WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-model-access-app-local-only';

ALTER DATABASE model_access_db
    OWNER TO gereh_model_access_migrator;

REVOKE ALL
    ON DATABASE model_access_db
    FROM PUBLIC;

GRANT CONNECT, TEMPORARY
    ON DATABASE model_access_db
    TO gereh_model_access_migrator;

GRANT CONNECT
    ON DATABASE model_access_db
    TO gereh_model_access_app;

\connect model_access_db

REVOKE CREATE
    ON SCHEMA public
    FROM PUBLIC;

GRANT USAGE, CREATE
    ON SCHEMA public
    TO gereh_model_access_migrator;

GRANT USAGE
    ON SCHEMA public
    TO gereh_model_access_app;
