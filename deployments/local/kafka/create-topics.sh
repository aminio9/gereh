#!/usr/bin/env bash
set -Eeuo pipefail

bootstrap_server="${KAFKA_BOOTSTRAP_SERVER:-kafka:19092}"
kafka_topics="/opt/kafka/bin/kafka-topics.sh"

echo "Waiting for Kafka at ${bootstrap_server}..."

for attempt in $(seq 1 60); do
  if "${kafka_topics}" \
    --bootstrap-server "${bootstrap_server}" \
    --list \
    >/dev/null 2>&1
  then
    break
  fi

  if [[ "${attempt}" -eq 60 ]]; then
    echo "Kafka did not become ready." >&2
    exit 1
  fi

  sleep 2
done

create_topic() {
  local topic="$1"
  local retention_ms="$2"

  "${kafka_topics}" \
    --bootstrap-server "${bootstrap_server}" \
    --create \
    --if-not-exists \
    --topic "${topic}" \
    --partitions 3 \
    --replication-factor 1 \
    --config cleanup.policy=delete \
    --config min.insync.replicas=1 \
    --config retention.ms="${retention_ms}"
}

seven_days="604800000"
fourteen_days="1209600000"
thirty_days="2592000000"

create_topic "gereh.tenant.events.v1" "${seven_days}"
create_topic "gereh.organization.company.events.v1" "${seven_days}"
create_topic "gereh.organization.agent.events.v1" "${seven_days}"
create_topic "gereh.work.events.v1" "${seven_days}"
create_topic "gereh.policy.events.v1" "${seven_days}"
create_topic "gereh.execution.commands.v1" "${seven_days}"
create_topic "gereh.execution.events.v1" "${seven_days}"
create_topic "gereh.model.usage.v1" "${thirty_days}"
create_topic "gereh.audit.events.v1" "${thirty_days}"
create_topic "gereh.events.dlq.v1" "${fourteen_days}"
create_topic "gereh.execution-orchestrator.dlq.v1" "${fourteen_days}"

echo
echo "Kafka topics:"

"${kafka_topics}" \
  --bootstrap-server "${bootstrap_server}" \
  --list
