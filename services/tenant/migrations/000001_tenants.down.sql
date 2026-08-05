BEGIN;

DROP TABLE IF EXISTS tenant_outbox;
DROP TABLE IF EXISTS tenant_entitlements;
DROP TABLE IF EXISTS tenant_memberships;
DROP TABLE IF EXISTS tenant_tenants;

COMMIT;