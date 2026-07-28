#!/usr/bin/env bash
set -Eeuo pipefail

image="${1:?usage: smoke-test.sh IMAGE [PATH]}"
health_path="${2:-/health/live}"

container_name="gereh-smoke-$RANDOM-$RANDOM"

cleanup() {
  docker rm -f "${container_name}" >/dev/null 2>&1 || true
}

trap cleanup EXIT

docker run \
  --detach \
  --name "${container_name}" \
  --env HTTP_ADDRESS=:8080 \
  --publish 127.0.0.1::8080 \
  "${image}" \
  >/dev/null

host_port="$(
  docker port "${container_name}" 8080/tcp \
    | awk -F: 'NR == 1 { print $NF }'
)"

if [[ -z "${host_port}" ]]; then
  echo "Could not determine container port." >&2
  docker logs "${container_name}" >&2
  exit 1
fi

for attempt in $(seq 1 60); do
  if curl \
    --fail \
    --silent \
    --show-error \
    "http://127.0.0.1:${host_port}${health_path}" \
    >/dev/null
  then
    echo "Container smoke test passed: ${image}"
    exit 0
  fi

  sleep 1
done

echo "Container did not become healthy: ${image}" >&2

docker logs "${container_name}" >&2

exit 1
