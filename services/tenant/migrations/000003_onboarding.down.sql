BEGIN;

DROP TABLE IF EXISTS tenant_onboarding_operations;

ALTER TABLE tenant_tenants
    DROP CONSTRAINT tenant_tenants_status_check;

ALTER TABLE tenant_tenants
    ADD CONSTRAINT tenant_tenants_status_check
    CHECK (
        status IN ('active', 'archived')
    );

ALTER TABLE tenant_tenants
    DROP CONSTRAINT tenant_tenants_archived_check;

ALTER TABLE tenant_tenants
    ADD CONSTRAINT tenant_tenants_archived_check
    CHECK (
        (
            status = 'active'
            AND archived_at IS NULL
        )
        OR
        (
            status = 'archived'
            AND archived_at IS NOT NULL
        )
    );

ALTER TABLE tenant_tenants
    ALTER COLUMN status SET DEFAULT 'active';

COMMIT;
