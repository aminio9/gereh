#!/usr/bin/env bash
set -Eeuo pipefail

required_commands=(
  go node pnpm docker task buf sqlc air golangci-lint grpcurl atlas trivy
  govulncheck psql redis-cli nc
)

failed=0

echo "Toolchain"
for command_name in "${required_commands[@]}"; do
  if command -v "${command_name}" >/dev/null 2>&1; then
    version="$("${command_name}" --version 2>/dev/null | head -n 1 || true)"
    printf "  ✓ %-18s %s\n" "${command_name}" "${version}"
  else
    printf "  ✗ %s is missing\n" "${command_name}"
    failed=1
  fi
done

echo
echo "Infrastructure"
checks=(
  "PostgreSQL:postgres:5432"
  "Redis:redis:6379"
  "Kafka:kafka:19092"
  "Temporal:temporal:7233"
  "MinIO:minio:9000"
)

for item in "${checks[@]}"; do
  IFS=: read -r label host port <<<"${item}"
  if nc -z "${host}" "${port}" >/dev/null 2>&1; then
    printf "  ✓ %-18s %s:%s\n" "${label}" "${host}" "${port}"
  else
    printf "  ✗ %-18s %s:%s unavailable\n" "${label}" "${host}" "${port}"
    failed=1
  fi
done

echo
if [[ "${failed}" -ne 0 ]]; then
  echo "Development environment has missing requirements."
  exit 1
fi

echo "Development environment is healthy."
