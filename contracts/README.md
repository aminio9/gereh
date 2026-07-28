# Gereh contracts

This directory contains all machine-readable contracts shared across
process, service, or organizational boundaries.

## Contract types

- `proto/`: internal gRPC messages and Kafka event payloads
- `openapi/`: public HTTP API contracts
- `events/`: Kafka topic governance and compatibility rules
- `webhooks/`: external webhook contracts

## Rules

1. Contracts are defined before implementations.
2. Breaking changes require a new versioned package or endpoint.
3. Database entities are never exported directly as contracts.
4. Public REST APIs use OpenAPI.
5. Internal synchronous APIs use Protobuf and gRPC.
6. Kafka payloads use versioned Protobuf messages.
7. Generated files are never edited manually.
8. Secrets and provider credentials are never represented in response
   contracts.
