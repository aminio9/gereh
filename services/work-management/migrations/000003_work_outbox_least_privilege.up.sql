BEGIN;

-- The runtime relay only needs SELECT, INSERT, and UPDATE on the operational
-- outbox. DELETE is revoked from the runtime role; test cleanup must use the
-- test migrator/admin connection instead.
REVOKE DELETE
    ON TABLE work_outbox
    FROM gereh_work_app;

COMMIT;
