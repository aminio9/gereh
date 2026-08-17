#!/usr/bin/env bash

set -Eeuo pipefail

repo_root="$(
  cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
  pwd
)"

: "${EXECUTION_CLUSTER_NAME:?EXECUTION_CLUSTER_NAME is required}"
: "${AWS_REGION:?AWS_REGION is required}"

for command in aws kubectl jq; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "required command is missing: ${command}" >&2
    exit 1
  fi
done

echo "Configuring kubeconfig for ${EXECUTION_CLUSTER_NAME}..."

aws eks update-kubeconfig \
  --name "${EXECUTION_CLUSTER_NAME}" \
  --region "${AWS_REGION}"

echo "Checking private Kubernetes API reachability..."

if ! kubectl \
  --request-timeout=10s \
  get \
  --raw=/readyz >/dev/null
then
  cat >&2 <<'EOF'
Unable to reach the execution-cluster Kubernetes API.

The Gereh execution EKS endpoint is intentionally private.

Run this bootstrap command from one of:
  - a VPC-connected administrative host
  - a VPN-connected workstation
  - a trusted self-hosted GitHub runner
  - a future GitOps management component inside the trusted network
EOF
  exit 1
fi

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/priority-classes.yaml"

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/namespaces.yaml"

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/default-deny.yaml"

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/allow-dns.yaml"

"${repo_root}/scripts/infra/execution-cluster/verify.sh"
