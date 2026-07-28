\set ON_ERROR_STOP on

SELECT format(
  'CREATE DATABASE %I OWNER %I',
  database_name,
  current_user
)
FROM (
  VALUES
    ('iam_db'),
    ('tenant_db'),
    ('organization_db'),
    ('work_db'),
    ('policy_db'),
    ('model_access_db'),
    ('execution_db'),
    ('billing_db'),
    ('projection_db'),
    ('audit_db')
) AS databases(database_name)
WHERE NOT EXISTS (
  SELECT 1
  FROM pg_database
  WHERE datname = database_name
)
\gexec
