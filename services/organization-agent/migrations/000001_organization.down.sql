BEGIN;

DROP TABLE IF EXISTS organization_outbox;
DROP TABLE IF EXISTS organization_bootstrap_requests;
DROP TABLE IF EXISTS organization_agent_revisions;
DROP TABLE IF EXISTS organization_agents;
DROP TABLE IF EXISTS organization_companies;

DROP FUNCTION IF EXISTS app.current_principal_type();
DROP FUNCTION IF EXISTS app.current_principal_id();
DROP FUNCTION IF EXISTS app.current_tenant_id();
DROP FUNCTION IF EXISTS app.scope_kind();

DROP SCHEMA IF EXISTS app;

COMMIT;
