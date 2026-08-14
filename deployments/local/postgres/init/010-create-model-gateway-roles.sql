\set ON_ERROR_STOP on

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_model_gateway_migrator'
    ) THEN
        CREATE ROLE gereh_model_gateway_migrator LOGIN;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_model_gateway_app'
    ) THEN
        CREATE ROLE gereh_model_gateway_app LOGIN;
    END IF;
END
$block$;

ALTER ROLE gereh_model_gateway_migrator WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-model-gateway-migrator-local-only';

ALTER ROLE gereh_model_gateway_app WITH
    NOSUPERUSER
    NOCREATEDB
    NOCREATEROLE
    NOINHERIT
    NOBYPASSRLS
    PASSWORD 'gereh-model-gateway-app-local-only';

ALTER DATABASE model_gateway_db
    OWNER TO gereh_model_gateway_migrator;

REVOKE ALL
    ON DATABASE model_gateway_db
    FROM PUBLIC;

GRANT CONNECT, TEMPORARY
    ON DATABASE model_gateway_db
    TO gereh_model_gateway_migrator;

GRANT CONNECT
    ON DATABASE model_gateway_db
    TO gereh_model_gateway_app;

\connect model_gateway_db

REVOKE CREATE
    ON SCHEMA public
    FROM PUBLIC;

GRANT USAGE, CREATE
    ON SCHEMA public
    TO gereh_model_gateway_migrator;

GRANT USAGE
    ON SCHEMA public
    TO gereh_model_gateway_app;
