# Projection Service

Kafka-backed tenant read models for dashboards, agent overviews, task
activity, and search.

The Projection Service is the **CQRS read side** of the platform. It consumes
tenant, company/agent, and work domain events from Kafka and builds
tenant-scoped read models in its own PostgreSQL database. Command services
remain the authoritative source of truth; this service answers "what does the
UI need to see quickly?" with eventually consistent data.

## Purpose

- CQRS read-side only. No write commands, no authoritative state.
- Single-database aggregate queries replace the BFF fan-out that would
  otherwise call Tenant, Organization, Work, and Policy services for every
  dashboard render.

## Authoritative sources

| Domain | Service | Kafka topic |
| --- | --- | --- |
| Tenant | Tenant Service | `gereh.tenant.events.v1` |
| Company | Company & Agent Service | `gereh.organization.company.events.v1` |
| Agent | Company & Agent Service | `gereh.organization.agent.events.v1` |
| Work | Work Management Service | `gereh.work.events.v1` |

Policy events are **not** projected into the ordinary read models: policy
definitions and decisions have materially narrower authorization semantics
than normal tenant data. Cost dashboard, inbox, execution overview, and model
usage remain extension points until their authoritative event families exist.

## Inputs

Kafka domain events (`EventEnvelope`) on the topics listed above.

The Projection consumer group starts at the **earliest retained offset** when
no committed group offset exists (`ConsumerStartOffsetEarliest`), so a fresh
deployment rebuilds the read model from retained history.

## Storage

`projection_db`, owned and migrated by `gereh_projection_migrator`, queried by
`gereh_projection_app` with **forced row-level security**.

Tables:

- `projection_consumed_events` — event-id inbox for idempotency
- `projection_partition_checkpoints` — durable Kafka-position checkpoints
- `projection_tenant_watermarks` — per-tenant projection freshness
- `projection_tenants`, `projection_companies`, `projection_agents`
- `projection_goals`, `projection_projects`, `projection_tasks`
- `projection_task_dependencies`, `projection_task_assignments`
- `projection_task_activity` — safe activity feed rows
- `projection_search_documents` — PostgreSQL full-text + trigram search

There are intentionally **no cross-domain foreign keys**: events may arrive out
of order across topics during rebalance, replay, or disaster recovery.

## Outputs

Internal gRPC query API (`PROJECTION_GRPC_ADDRESS`, default `:18086`):

- `GetDashboardSummary`
- `GetCompanyOverview`
- `ListAgentOverviews`
- `ListTaskActivity`
- `Search`

Every response carries `ProjectionMetadata`
(`projected_through_event_time`, `last_processed_at`) so clients can
distinguish "zero results" from "projection has not caught up".

The BFF exposes these as:

```text
GET /v1/tenants/{tenantId}/dashboard
GET /v1/tenants/{tenantId}/companies/{companyId}/overview
GET /v1/tenants/{tenantId}/agents/overview
GET /v1/tenants/{tenantId}/companies/{companyId}/agents/overview
GET /v1/tenants/{tenantId}/activity
GET /v1/tenants/{tenantId}/search?q=...
```

## Consistency

Eventual. The read model is rebuilt from Kafka events; responses are
eventually consistent with command services.

The durability rule is:

```text
Kafka record
    -> BEGIN PostgreSQL transaction
    -> insert/check consumed-event inbox
    -> update projection rows
    -> append safe activity item
    -> update search document
    -> update tenant watermark
    -> update partition checkpoint
    -> COMMIT PostgreSQL
    -> commit Kafka offset
```

## Idempotency

`projection_consumed_events.event_id` is the inbox. A redelivered event:

- has its content hash compared; identical content is skipped without
  mutation and the checkpoint advances;
- different content for the same `event_id` fails with
  `ErrEventIdentityConflict`, which stops the consumer (data-integrity
  incident).

Aggregate-version guards (`WHERE source_version < EXCLUDED.source_version`)
prevent an older aggregate event from overwriting a newer one across topics.

## Recovery

1. Restore a `projection_db` backup.
2. Inspect `projection_partition_checkpoints`.
3. Reset the Kafka consumer group to `checkpoint + 1` before resuming.

Kafka committed offsets can be newer than a restored backup; without the reset,
events between the backup and the committed offset would be permanently lost
from the restored read model.

Full replay works only as far back as retained source events. Production
therefore requires projection DB backups plus Kafka retention longer than the
backup/RPO recovery window.

## Security

- Every query authorizes the actor with the Tenant Service
  (`RequireTenantRead`) before touching the database.
- Queries run in explicit tenant-scoped read-only transactions.
- All tenant-owned tables have **forced row-level security**; the runtime role
  `gereh_projection_app` is `NOSUPERUSER`, `NOBYPASSRLS`, and owns no
  projection table.
- Kafka mutations use a fixed service principal inside the same RLS.
- Envelope `tenant_id` must equal payload `tenant_id`.
- Read models never contain credentials; activity rows never contain comment
  bodies, artifact object keys, or policy details.

## Forbidden dependencies

- No source-service database access.
- No cross-service `internal` imports.

## Development

```bash
task infra:up

export PROJECTION_MIGRATION_DATABASE_URL='postgres://gereh_projection_migrator:gereh-projection-migrator-local-only@127.0.0.1:5432/projection_db?sslmode=disable'
export PROJECTION_DATABASE_URL='postgres://gereh_projection_app:gereh-projection-app-local-only@127.0.0.1:5432/projection_db?sslmode=disable'

task db:projection:hash
task db:projection:validate
task db:projection:apply
task db:projection:status

task test:projection
```

## gRPC health

The process stays `NOT_SERVING` until the Kafka consumer receives its first
non-empty partition assignment, then flips to `SERVING`. Kubernetes liveness
and readiness probes therefore never route traffic to a projection that cannot
yet report caught-up state.