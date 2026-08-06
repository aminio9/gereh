# PostgreSQL platform package

Technical helpers only:

- pgx pool construction with production-safe runtime parameters
- explicit transaction helpers with tenant/`principal` RLS scope binding
- transaction-local `app.*` security settings
- startup runtime-role verification (NOSUPERUSER / NOBYPASSRLS / owns no tables)
- health checks

Tenant-owned business queries must run through `Database.Begin` or
`ApplyScope` under an explicit `Scope`. Service-internal operational tables
(such as the transaction outbox) may use `Database.Pool`, but business-table
access must never bypass the scope.

No service-owned queries or domain entities belong here. Only `Context`,
pool, transaction, and RLS-scope plumbing lives in this package.