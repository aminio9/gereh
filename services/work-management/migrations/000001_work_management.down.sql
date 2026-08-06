BEGIN;

DROP TABLE IF EXISTS work_outbox;
DROP TABLE IF EXISTS work_task_schedules;
DROP TABLE IF EXISTS work_checklist_items;
DROP TABLE IF EXISTS work_artifacts;
DROP TABLE IF EXISTS work_comments;
DROP TABLE IF EXISTS work_task_assignments;
DROP TABLE IF EXISTS work_task_dependencies;
DROP TABLE IF EXISTS work_tasks;
DROP TABLE IF EXISTS work_projects;
DROP TABLE IF EXISTS work_goals;

DROP FUNCTION IF EXISTS app.current_principal_type();
DROP FUNCTION IF EXISTS app.current_principal_id();
DROP FUNCTION IF EXISTS app.current_tenant_id();
DROP FUNCTION IF EXISTS app.scope_kind();

DROP SCHEMA IF EXISTS app;

COMMIT;
