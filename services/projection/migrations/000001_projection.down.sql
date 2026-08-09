BEGIN;

DROP TABLE IF EXISTS projection_search_documents;
DROP TABLE IF EXISTS projection_task_activity;
DROP TABLE IF EXISTS projection_task_assignments;
DROP TABLE IF EXISTS projection_task_dependencies;
DROP TABLE IF EXISTS projection_tasks;
DROP TABLE IF EXISTS projection_projects;
DROP TABLE IF EXISTS projection_goals;
DROP TABLE IF EXISTS projection_agents;
DROP TABLE IF EXISTS projection_companies;
DROP TABLE IF EXISTS projection_tenants;
DROP TABLE IF EXISTS projection_tenant_watermarks;
DROP TABLE IF EXISTS projection_partition_checkpoints;
DROP TABLE IF EXISTS projection_consumed_events;

DROP FUNCTION IF EXISTS app.current_principal_type();
DROP FUNCTION IF EXISTS app.current_principal_id();
DROP FUNCTION IF EXISTS app.current_tenant_id();
DROP FUNCTION IF EXISTS app.scope_kind();

DROP SCHEMA IF EXISTS app;

COMMIT;