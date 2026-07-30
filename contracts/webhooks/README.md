# Webhook contracts

External webhook contracts will be stored here.

Each webhook contract must define:

- Event name and version
- Request body schema
- Signature algorithm
- Timestamp tolerance
- Replay protection
- Retry policy
- Idempotency key
- Expected response codes
- Deprecation policy

Webhook payloads must not expose internal database schemas or secrets.
