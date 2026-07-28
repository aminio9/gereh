# Local environment runbook

## Core startup

```bash
task infra:up
task doctor
```

## Complete reset

```bash
task infra:reset
```

This deletes all local development data.

## Kafka topics

```bash
docker compose -f .devcontainer/compose.yaml exec kafka \
  /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server kafka:19092 \
  --list
```

## PostgreSQL

```bash
psql postgres://gereh:gereh@postgres:5432/gereh
```

## Redis

```bash
redis-cli -h redis ping
```
