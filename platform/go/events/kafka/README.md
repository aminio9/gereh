# Gereh Kafka Transport

This package provides the shared Kafka producer and consumer implementation.

## Producer guarantees

- Idempotent producer behavior
- `acks=all`
- Deterministic Protobuf encoding
- Zstandard compression
- Bounded batch size
- Bounded delivery timeout
- Aggregate-based record keys
- OpenTelemetry trace propagation
- Synchronous acknowledgement before `Publish` returns

## Consumer guarantees

- Cooperative-sticky balancing
- Automatic commits disabled
- Records committed only after successful handler completion
- Rebalances blocked during record processing
- Sequential processing within each poll
- OpenTelemetry receive and process spans
- At-least-once delivery

## Required environment

```text
KAFKA_BROKERS=kafka:19092
KAFKA_CLIENT_ID=tenant
```

Consumers additionally require:

```text
KAFKA_GROUP_ID=tenant-projection-v1
KAFKA_TOPICS=gereh.tenant.events.v1
```

## Optional security

### TLS

```text
KAFKA_TLS_ENABLED=true
KAFKA_TLS_SERVER_NAME=kafka.internal
KAFKA_TLS_CA_FILE=/run/secrets/kafka-ca.pem
KAFKA_TLS_CERT_FILE=/run/secrets/kafka-client.pem
KAFKA_TLS_KEY_FILE=/run/secrets/kafka-client-key.pem
```

### SASL

```text
KAFKA_SASL_MECHANISM=scram-sha-512
KAFKA_SASL_USERNAME=service-account
KAFKA_SASL_PASSWORD=secret
```

## Processing rule

Every handler must be idempotent. A handler can successfully mutate its
database and then lose the Kafka commit response, causing the record to be
delivered again.

Use one of the following strategies:

- A processed-event inbox table with a unique event ID
- Aggregate version checks
- Idempotency-key constraints
- A transactional inbox combined with the business mutation
