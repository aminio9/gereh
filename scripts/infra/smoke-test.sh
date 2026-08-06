#!/usr/bin/env bash
set -Eeuo pipefail

compose_file="${1:-deployments/local/compose.yaml}"
env_file="${2:-deployments/local/.env}"

compose=(
  docker compose
  --env-file "${env_file}"
  -f "${compose_file}"
)

echo "Checking PostgreSQL..."

databases="$(
  "${compose[@]}" exec -T postgres \
    psql \
      -U "${POSTGRES_USER:-gereh}" \
      -d postgres \
      -Atc "SELECT datname FROM pg_database ORDER BY datname;"
)"

required_databases=(
  iam_db
  tenant_db
  organization_db
  work_db
  policy_db
  model_access_db
  execution_db
  billing_db
  projection_db
  audit_db
)

for database in "${required_databases[@]}"; do
  if ! grep -qx "${database}" <<<"${databases}"; then
    echo "Missing PostgreSQL database: ${database}" >&2
    exit 1
  fi
done

echo "Checking Redis..."

"${compose[@]}" exec -T redis \
  redis-cli \
  SET gereh:smoke-test ok \
  EX 60 \
  >/dev/null

redis_value="$(
  "${compose[@]}" exec -T redis \
    redis-cli \
    --raw \
    GET gereh:smoke-test
)"

if [[ "${redis_value}" != "ok" ]]; then
  echo "Redis smoke test failed." >&2
  exit 1
fi

"${compose[@]}" exec -T redis \
  redis-cli \
  DEL gereh:smoke-test \
  >/dev/null

echo "Checking Kafka..."

topics="$(
  "${compose[@]}" exec -T kafka \
    /opt/kafka/bin/kafka-topics.sh \
      --bootstrap-server localhost:19092 \
      --list
)"

required_topics=(
  gereh.tenant.events.v1
  gereh.organization-agent.events.v1
  gereh.work.events.v1
  gereh.policy-approval.events.v1
  gereh.execution.commands.v1
  gereh.execution.events.v1
  gereh.model.usage.v1
  gereh.audit.events.v1
  gereh.events.dlq.v1
)

for topic in "${required_topics[@]}"; do
  if ! grep -qx "${topic}" <<<"${topics}"; then
    echo "Missing Kafka topic: ${topic}" >&2
    exit 1
  fi
done

echo "Checking Temporal..."

"${compose[@]}" exec -T temporal \
  temporal operator cluster health \
    --address 127.0.0.1:7233

echo
echo "Standalone development infrastructure is healthy."
