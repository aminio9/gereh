# Public OpenAPI contracts

`public-api.yaml` is the source of truth for the browser and external
customer API exposed by the Gereh API BFF.

Internal microservices must not expose this API directly.

Rules:

- Every operation has a stable `operationId`.
- Mutating operations support idempotency keys.
- Long-running operations return `202 Accepted`.
- Errors use the shared error envelope.
- Request IDs are propagated through `X-Request-ID`.
- Tenant identity is derived from authentication, not trusted from an
  arbitrary browser header.
