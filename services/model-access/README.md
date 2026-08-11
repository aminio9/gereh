# Model Access Service

The Model Access Service owns tenant model-connection control-plane state.

## Current responsibilities

- model provider metadata;
- tenant model connections;
- platform-managed connection enablement;
- tenant authorization and entitlement validation;
- Gereh provider-pool selection;
- optimistic concurrency;
- mutation idempotency;
- immutable connection revisions;
- tenant RLS;
- transactional outbox events.

## Platform-managed connection

The platform-managed lifecycle is:

Customer requests a Gereh-managed provider
→
Tenant Service validates permission and entitlement
→
Model Access selects an eligible Gereh provider pool
→
connection is persisted as active
→
ModelConnectionCreated is emitted transactionally

Provider-pool identities are internal routing metadata and are not returned
through public APIs.

No provider credentials are stored in model-connection business rows.

## Not implemented yet

The following belong to later Model Access phases:

- BYOK credential storage and verification;
- custom endpoint validation and SSRF protection;
- model offering discovery/catalog;
- agent model assignments;
- Model Gateway runtime token issuance;
- provider inference proxying;
- usage and cost normalization.

## Data ownership

Model Access owns `model_access_db`.

It must not query Tenant, Organization, Work, Policy or other service
databases directly.

Tenant information is obtained through Tenant Service gRPC.

## Events

Control-plane connection events are published to:

`gereh.model.events.v1`

using the transactional outbox.

## Security invariants

- never store provider API keys in `ModelConnection`;
- never return provider credentials;
- never place provider credentials in Kafka events;
- never place provider credentials in agent configuration;
- tenant business rows are protected by forced RLS;
- browser traffic reaches this service through the BFF;
- production gRPC uses workload mTLS.
