# Event contracts

Kafka topics carry immutable facts that have already occurred.

## Naming

Topic names use:

`gereh.<bounded-context>.<purpose>.v<major>`

Examples:

- `gereh.tenant.events.v1`
- `gereh.agent.events.v1`
- `gereh.execution.commands.v1`
- `gereh.execution.events.v1`
- `gereh.model.usage.v1`

## Delivery model

Consumers must assume at-least-once delivery.

Every consumer must therefore:

1. Validate the event version.
2. Deduplicate by `event_id`.
3. Process inside a local database transaction.
4. Record the consumed event ID.
5. Commit before advancing the Kafka offset.
6. Route poison events to an owned dead-letter topic.

## Compatibility

Additive compatible changes may remain in the same event version.
Semantic or structural breaking changes require a new event type or
major version.
