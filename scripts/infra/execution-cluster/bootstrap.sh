#!/usr/bin/env bash

set -Eeuo pipefail

repo_root="$(
  cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
  pwd
)"

: "${KUBECONFIG:?KUBECONFIG is required}"

kubectl get --raw=/readyz >/dev/null

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/namespaces.yaml"

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/priority-classes.yaml"

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/default-service-accounts.yaml"

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/default-deny.yaml"

kubectl apply \
  -f "${repo_root}/deploy/policies/execution/allow-dns.yaml"

"${repo_root}/scripts/infra/execution-cluster/verify.sh"
