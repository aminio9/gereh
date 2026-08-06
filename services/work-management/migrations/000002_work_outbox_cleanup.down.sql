BEGIN;

REVOKE DELETE
    ON TABLE work_outbox
    FROM gereh_work_app;

COMMIT;
