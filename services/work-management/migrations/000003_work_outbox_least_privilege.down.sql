BEGIN;

GRANT DELETE
    ON TABLE work_outbox
    TO gereh_work_app;

COMMIT;
