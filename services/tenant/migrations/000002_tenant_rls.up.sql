BEGIN;

DO $block$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = 'gereh_tenant_app'
    ) THEN
        RAISE EXCEPTION
            'required PostgreSQL role gereh_tenant_app does not exist';
    END IF;
END
$block$;

CREATE SCHEMA IF NOT EXISTS app;

REVOKE ALL
    ON SCHEMA app
    FROM PUBLIC;

GRANT USAGE
    ON SCHEMA app
    TO gereh_tenant_app;

CREATE OR REPLACE FUNCTION app.scope_kind()
RETURNS TEXT
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.scope_kind', TRUE),
        ''
    )
$function$;

CREATE OR REPLACE FUNCTION app.current_tenant_id()
RETURNS UUID
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.tenant_id', TRUE),
        ''
    )::UUID
$function$;

CREATE OR REPLACE FUNCTION app.current_principal_id()
RETURNS UUID
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.principal_id', TRUE),
        ''
    )::UUID
$function$;

CREATE OR REPLACE FUNCTION app.current_principal_type()
RETURNS TEXT
LANGUAGE SQL
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $function$
    SELECT NULLIF(
        current_setting('app.principal_type', TRUE),
        ''
    )
$function$;

REVOKE ALL
    ON FUNCTION app.scope_kind()
    FROM PUBLIC;

REVOKE ALL
    ON FUNCTION app.current_tenant_id()
    FROM PUBLIC;

REVOKE ALL
    ON FUNCTION app.current_principal_id()
    FROM PUBLIC;

REVOKE ALL
    ON FUNCTION app.current_principal_type()
    FROM PUBLIC;

GRANT EXECUTE
    ON FUNCTION app.scope_kind()
    TO gereh_tenant_app;

GRANT EXECUTE
    ON FUNCTION app.current_tenant_id()
    TO gereh_tenant_app;

GRANT EXECUTE
    ON FUNCTION app.current_principal_id()
    TO gereh_tenant_app;

GRANT EXECUTE
    ON FUNCTION app.current_principal_type()
    TO gereh_tenant_app;

-- Existing tenant-service events use the tenant UUID as partition_key.
-- Casting fails deliberately if historical data violates that invariant.
ALTER TABLE tenant_outbox
    ADD COLUMN tenant_id UUID;

UPDATE tenant_outbox
SET tenant_id = partition_key::UUID
WHERE tenant_id IS NULL;

ALTER TABLE tenant_outbox
    ALTER COLUMN tenant_id SET NOT NULL;

CREATE INDEX tenant_outbox_tenant_occurred_idx
    ON tenant_outbox(
        tenant_id,
        occurred_at DESC,
        outbox_id DESC
    );

REVOKE ALL
    ON TABLE tenant_tenants
    FROM PUBLIC;

REVOKE ALL
    ON TABLE tenant_memberships
    FROM PUBLIC;

REVOKE ALL
    ON TABLE tenant_entitlements
    FROM PUBLIC;

REVOKE ALL
    ON TABLE tenant_outbox
    FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE tenant_tenants
    TO gereh_tenant_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE tenant_memberships
    TO gereh_tenant_app;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE tenant_entitlements
    TO gereh_tenant_app;

-- The service relay processes all tenants, so the operational outbox is not
-- protected by per-tenant RLS. DELETE is needed for test cleanup and
-- service-internal outbox lifecycle management.
GRANT SELECT, INSERT, UPDATE, DELETE
    ON TABLE tenant_outbox
    TO gereh_tenant_app;

GRANT USAGE, SELECT
    ON SEQUENCE tenant_outbox_outbox_id_seq
    TO gereh_tenant_app;

ALTER TABLE tenant_tenants
    ENABLE ROW LEVEL SECURITY;

ALTER TABLE tenant_tenants
    FORCE ROW LEVEL SECURITY;

ALTER TABLE tenant_memberships
    ENABLE ROW LEVEL SECURITY;

ALTER TABLE tenant_memberships
    FORCE ROW LEVEL SECURITY;

ALTER TABLE tenant_entitlements
    ENABLE ROW LEVEL SECURITY;

ALTER TABLE tenant_entitlements
    FORCE ROW LEVEL SECURITY;

-- Tenant catalog -------------------------------------------------------------

CREATE POLICY tenant_tenants_select
ON tenant_tenants
FOR SELECT
TO gereh_tenant_app
USING (
    (
        app.scope_kind() = 'tenant'
        AND tenant_id = app.current_tenant_id()
    )
    OR
    (
        app.scope_kind() = 'principal'
        AND app.current_principal_type() = 'user'
        AND (
            created_by_user_id =
                app.current_principal_id()
            OR EXISTS (
                SELECT 1
                FROM tenant_memberships AS membership
                WHERE membership.tenant_id =
                    tenant_tenants.tenant_id
                  AND membership.user_id =
                    app.current_principal_id()
            )
        )
    )
);

CREATE POLICY tenant_tenants_insert
ON tenant_tenants
FOR INSERT
TO gereh_tenant_app
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
    AND created_by_user_id =
        app.current_principal_id()
);

CREATE POLICY tenant_tenants_update
ON tenant_tenants
FOR UPDATE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY tenant_tenants_delete
ON tenant_tenants
FOR DELETE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

-- Memberships ---------------------------------------------------------------

CREATE POLICY tenant_memberships_select
ON tenant_memberships
FOR SELECT
TO gereh_tenant_app
USING (
    (
        app.scope_kind() = 'tenant'
        AND tenant_id = app.current_tenant_id()
    )
    OR
    (
        app.scope_kind() = 'principal'
        AND app.current_principal_type() = 'user'
        AND user_id = app.current_principal_id()
    )
);

CREATE POLICY tenant_memberships_insert
ON tenant_memberships
FOR INSERT
TO gereh_tenant_app
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY tenant_memberships_update
ON tenant_memberships
FOR UPDATE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY tenant_memberships_delete
ON tenant_memberships
FOR DELETE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

-- Entitlements ---------------------------------------------------------------

CREATE POLICY tenant_entitlements_select
ON tenant_entitlements
FOR SELECT
TO gereh_tenant_app
USING (
    (
        app.scope_kind() = 'tenant'
        AND tenant_id = app.current_tenant_id()
    )
    OR
    (
        app.scope_kind() = 'principal'
        AND app.current_principal_type() = 'user'
        AND EXISTS (
            SELECT 1
            FROM tenant_memberships AS membership
            WHERE membership.tenant_id =
                tenant_entitlements.tenant_id
              AND membership.user_id =
                app.current_principal_id()
        )
    )
);

CREATE POLICY tenant_entitlements_insert
ON tenant_entitlements
FOR INSERT
TO gereh_tenant_app
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY tenant_entitlements_update
ON tenant_entitlements
FOR UPDATE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
)
WITH CHECK (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

CREATE POLICY tenant_entitlements_delete
ON tenant_entitlements
FOR DELETE
TO gereh_tenant_app
USING (
    app.scope_kind() = 'tenant'
    AND tenant_id = app.current_tenant_id()
);

COMMIT;