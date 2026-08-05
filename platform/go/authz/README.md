# Shared authorization runtime

`platform/go/authz` provides reusable tenant authorization checks for internal
gRPC services.

Each service defines a complete method policy map. Methods absent from the map
are denied by default.

The shared interceptor expects validated internal request metadata:

- `x-actor-user-id`
- `x-tenant-id`
- `x-request-id`
- `x-correlation-id`

The interceptor calls Tenant Service and attaches the trusted authorization
decision to the request context.

Tenant Service itself does not use this interceptor because it is the policy
decision point.

Metadata alone is not a trusted identity boundary. Production deployments must
carry `x-actor-user-id` and `x-tenant-id` over authenticated workload transport
such as mTLS, SPIFFE/SPIRE, service-mesh identity, or a cloud-native service
identity.
