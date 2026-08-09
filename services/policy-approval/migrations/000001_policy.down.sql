BEGIN;

DROP TABLE IF EXISTS policy_outbox;
DROP TABLE IF EXISTS policy_bootstrap_requests;
DROP TABLE IF EXISTS policy_decisions;
DROP TABLE IF EXISTS policy_rules;

ALTER TABLE policy_sets
    DROP CONSTRAINT IF EXISTS
        policy_sets_active_version_fk;

DROP TABLE IF EXISTS policy_versions;
DROP TABLE IF EXISTS policy_sets;

DROP FUNCTION IF EXISTS app.current_principal_type();
DROP FUNCTION IF EXISTS app.current_principal_id();
DROP FUNCTION IF EXISTS app.current_tenant_id();
DROP FUNCTION IF EXISTS app.scope_kind();

DROP SCHEMA IF EXISTS app;

COMMIT;