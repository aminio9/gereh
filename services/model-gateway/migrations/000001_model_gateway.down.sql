BEGIN;

DROP TABLE IF EXISTS model_gateway_outbox;
DROP TABLE IF EXISTS model_gateway_request_journal;

DROP FUNCTION IF EXISTS app.current_principal_type();
DROP FUNCTION IF EXISTS app.current_principal_id();
DROP FUNCTION IF EXISTS app.current_tenant_id();
DROP FUNCTION IF EXISTS app.scope_kind();

COMMIT;
