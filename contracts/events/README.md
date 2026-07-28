# Domain event contracts

Each event contract is owned by its producer and versioned explicitly.

Rules:

- Never change the meaning of an existing field.
- Additive compatible changes remain in the same event version.
- Breaking semantic changes require a new event type or major version.
- Every event uses the common envelope.
