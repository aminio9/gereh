# tenant

Tenant lifecycle, memberships, entitlements, region and retention policy.

Default local HTTP port: `8082`.

This directory is an independently deployable service boundary. Its `internal/`
packages must not be imported by another service.

## Authorization model

Tenant Service is the policy decision point for tenant-scoped authorization.

Authentication establishes an internal Gereh `user_id`. Tenant Service then maps:

`user_id + tenant_id -> membership role -> effective permissions`

Authorization is deny-by-default. Every tenant-scoped request must check an
explicit permission.

### Roles

- Owner: full tenant and membership administration
- Admin: tenant settings and non-owner membership administration
- Member: tenant and member visibility
- Viewer: minimal tenant and entitlement visibility

### Additional invariants

Role permissions do not override membership hierarchy invariants:

- admins cannot assign, modify, or remove owners;
- a tenant must always retain at least one owner;
- archived tenants are read-only;
- optimistic versions must match for administrative mutations.

### Internal identity propagation

The BFF derives `x-actor-user-id` from the validated opaque browser session.
Tenant Service rejects RPC requests whose `actor_user_id` field differs from
the authenticated actor metadata.

Production deployments must protect internal metadata with authenticated
workload transport such as mTLS, SPIFFE/SPIRE, service-mesh identity, or a
cloud-native service identity. In production, gRPC metadata is authorization
context and not a substitute for authenticated service-to-service transport.
