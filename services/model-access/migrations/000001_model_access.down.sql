BEGIN;

DROP TABLE IF EXISTS
    model_access_outbox;

DROP TABLE IF EXISTS
    model_access_idempotency;

DROP TABLE IF EXISTS
    model_access_connection_revisions;

DROP TABLE IF EXISTS
    model_access_connections;

DROP TABLE IF EXISTS
    model_access_providers;

DROP FUNCTION IF EXISTS
    app.current_principal_type();

DROP FUNCTION IF EXISTS
    app.current_principal_id();

DROP FUNCTION IF EXISTS
    app.current_tenant_id();

DROP FUNCTION IF EXISTS
    app.scope_kind();

DROP SCHEMA IF EXISTS app;

COMMIT;
