# RuntimeCell API

## Purpose

`runtime.gereh.ai/v1 RuntimeCell` is the Kubernetes desired-state contract between the Gereh control plane and the execution plane.
It does not expose cloud-provider infrastructure.

## Ownership

Writer:
- Runtime Manager only.

Readers:
- Runtime Manager.
- Runtime connectors where explicitly required.
- Operational/debugging identities through read-only RBAC.

Tenant workloads must never receive create, update, patch, delete, status, or finalizer permissions for RuntimeCell resources.

## Namespace

RuntimeCell control objects are stored in:
`gereh-runtime-system`

The object is not stored in tenant workload namespaces.

## Desired state

`spec.tenantId`
Gereh tenant identity. Immutable.

`spec.operationId`
Idempotency identity corresponding to `EnsureTenantRuntime.operation_id`. Immutable.

`spec.runtime`
Supported values:
- `openclaw`
- `hermes`

Immutable.

`spec.version`
Desired approved runtime release. Mutable so Runtime Manager can perform controlled upgrades.

`spec.region`
Logical Gereh execution region. Examples:
- `ir-thr-1`
- `ir-mhd-1`

The value is a Gereh placement identity, not a cloud-provider resource ID. Immutable.

`spec.isolationTier`
Supported values:
- `standard`
- `dedicated`

Immutable.

`spec.storageClass`
Kubernetes StorageClass selected by trusted regional placement policy. It must never contain cloud API credentials or raw CSI credentials. Immutable.

## Observed state

`status.observedGeneration`
Latest metadata generation reconciled by Runtime Manager.

`status.phase`
Summary state:
- `Pending`
- `Provisioning`
- `Ready`
- `Degraded`
- `Failed`
- `Deleting`

`status.observedRuntimeVersion`
Runtime version actually running.

`status.failureCode`
Safe machine-readable failure code. Never place:
- credentials;
- complete provider error bodies;
- prompts;
- completions;
- tool arguments;
- customer content;
- secret-bearing URLs;
- stack traces containing secrets in this field.

`status.conditions`
Kubernetes-style detailed conditions. Initial controller condition types:
- `Ready`
- `InfrastructureReady`
- `RuntimeReady`

## gRPC mapping

`EnsureTenantRuntimeRequest.tenant_id` → `spec.tenantId`
`EnsureTenantRuntimeRequest.operation_id` → `spec.operationId`
`EnsureTenantRuntimeRequest.runtime` → `spec.runtime`
`EnsureTenantRuntimeRequest.runtime_version` → `spec.version`
`EnsureTenantRuntimeRequest.region` → `spec.region`
`EnsureTenantRuntimeRequest.isolation_tier` → `spec.isolationTier`
`EnsureTenantRuntimeRequest.storage_class` → `spec.storageClass`

`RuntimeCell.status.phase = Pending | Provisioning` → `RUNTIME_PROVISIONING_STATE_PENDING`
`RuntimeCell.status.phase = Ready` → `RUNTIME_PROVISIONING_STATE_READY`
`RuntimeCell.status.phase = Failed | Degraded` → `RUNTIME_PROVISIONING_STATE_FAILED`

## Provider neutrality

The RuntimeCell schema must not contain:
- Arvan IDs;
- AWS IDs;
- RKE2 node addresses;
- VM flavors;
- floating IPs;
- cloud credentials;
- provider-specific volume IDs;
- Terraform data;
- raw Pod templates.

Provider and cluster implementation is resolved by Runtime Manager and cluster/platform configuration.

## Phase ownership

Phase 22:
- API type.
- CRD.
- schema validation.
- status contract.
- generated artifacts.
- API contract tests.

Phase 23:
- Kubernetes client.
- reconciliation.
- leader election.
- finalizer handling.
- RBAC.
- status transitions.
- idempotent creation.
- recovery logic.

Phase 24:
- OpenClaw implementation.

Phase 25:
- Hermes implementation.

Phase 26:
- workspace and sandbox lifecycle.
