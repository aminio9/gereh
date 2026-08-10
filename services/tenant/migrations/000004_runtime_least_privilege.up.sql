BEGIN;

-- Tenant lifecycle is archival / durable.
DROP POLICY IF EXISTS
    tenant_tenants_delete
    ON tenant_tenants;

REVOKE DELETE
    ON TABLE tenant_tenants
    FROM gereh_tenant_app;

-- Entitlements are versioned durable state.
DROP POLICY IF EXISTS
    tenant_entitlements_delete
    ON tenant_entitlements;

REVOKE DELETE
    ON TABLE tenant_entitlements
    FROM gereh_tenant_app;

-- Onboarding operations are durable customer-visible history.
DROP POLICY IF EXISTS
    tenant_onboarding_operations_delete
    ON tenant_onboarding_operations;

REVOKE DELETE
    ON TABLE tenant_onboarding_operations
    FROM gereh_tenant_app;

-- Outbox relay claims/updates rows; production runtime
-- does not need to hard-delete events.
REVOKE DELETE
    ON TABLE tenant_outbox
    FROM gereh_tenant_app;

-- Intentionally retain DELETE on tenant_memberships:
-- RemoveMember is a legitimate domain command.

COMMIT;