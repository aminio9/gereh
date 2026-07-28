#!/usr/bin/env bash
set -Eeuo pipefail

mkdir -p bin

for service_dir in services/*; do
  [[ -d "${service_dir}/cmd" ]] || continue

  service_name="$(basename "${service_dir}")"
  package="./${service_dir}/cmd/${service_name}"

  echo "Building ${service_name}..."
  CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o "bin/${service_name}" \
    "${package}"
done
