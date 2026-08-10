BEGIN;

GRANT DELETE
    ON TABLE tenant_tenants
    TO gereh_tenant_app;

CREATE POLICY tenant_tenants_delete
ON tenant_tenants
FOR DELETE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

GRANT DELETE
    ON TABLE tenant_entitlements
    TO gereh_tenant_app;

CREATE POLICY tenant_entitlements_delete
ON tenant_entitlements
FOR DELETE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

GRANT DELETE
    ON TABLE tenant_onboarding_operations
    TO gereh_tenant_app;

CREATE POLICY tenant_onboarding_operations_delete
ON tenant_onboarding_operations
FOR DELETE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

GRANT DELETE
    ON TABLE tenant_outbox
    TO gereh_tenant_app;

COMMIT;