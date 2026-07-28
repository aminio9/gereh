#!/usr/bin/env bash
set -Eeuo pipefail

echo
echo "Gereh development container started."

if [[ -S /var/run/docker.sock ]]; then
  echo "Core dependencies:"
  sudo docker compose \
    -f .devcontainer/compose.yaml \
    ps \
    --format 'table {{.Service}}\t{{.Status}}\t{{.Ports}}' \
    || true
else
  echo "Docker socket is not mounted inside the workspace container."
  echo "Recreate the Dev Container after changing compose.yaml."
fi

echo
echo "Run task doctor after post-create setup completes."
