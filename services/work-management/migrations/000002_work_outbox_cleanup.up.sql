BEGIN;

-- The service relay processes all tenants, so the operational outbox is not
-- protected by per-tenant RLS. DELETE is needed for test cleanup and
-- service-internal outbox lifecycle management.
GRANT DELETE
    ON TABLE work_outbox
    TO gereh_work_app;

COMMIT;
