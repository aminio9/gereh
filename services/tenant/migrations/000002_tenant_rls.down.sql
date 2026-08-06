BEGIN;

DROP POLICY IF EXISTS
    tenant_entitlements_delete
    ON tenant_entitlements;

DROP POLICY IF EXISTS
    tenant_entitlements_update
    ON tenant_entitlements;

DROP POLICY IF EXISTS
    tenant_entitlements_insert
    ON tenant_entitlements;

DROP POLICY IF EXISTS
    tenant_entitlements_select
    ON tenant_entitlements;

DROP POLICY IF EXISTS
    tenant_memberships_delete
    ON tenant_memberships;

DROP POLICY IF EXISTS
    tenant_memberships_update
    ON tenant_memberships;

DROP POLICY IF EXISTS
    tenant_memberships_insert
    ON tenant_memberships;

DROP POLICY IF EXISTS
    tenant_memberships_select
    ON tenant_memberships;

DROP POLICY IF EXISTS
    tenant_tenants_delete
    ON tenant_tenants;

DROP POLICY IF EXISTS
    tenant_tenants_update
    ON tenant_tenants;

DROP POLICY IF EXISTS
    tenant_tenants_insert
    ON tenant_tenants;

DROP POLICY IF EXISTS
    tenant_tenants_select
    ON tenant_tenants;

ALTER TABLE tenant_entitlements
    NO FORCE ROW LEVEL SECURITY;

ALTER TABLE tenant_entitlements
    DISABLE ROW LEVEL SECURITY;

ALTER TABLE tenant_memberships
    NO FORCE ROW LEVEL SECURITY;

ALTER TABLE tenant_memberships
    DISABLE ROW LEVEL SECURITY;

ALTER TABLE tenant_tenants
    NO FORCE ROW LEVEL SECURITY;

ALTER TABLE tenant_tenants
    DISABLE ROW LEVEL SECURITY;

DROP INDEX IF EXISTS
    tenant_outbox_tenant_occurred_idx;

ALTER TABLE tenant_outbox
    DROP COLUMN IF EXISTS tenant_id;

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