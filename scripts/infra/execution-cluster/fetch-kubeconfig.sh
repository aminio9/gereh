#!/usr/bin/env bash

set -Eeuo pipefail

: "${RKE2_ADMIN_HOST:?RKE2_ADMIN_HOST is required}"
: "${RKE2_REGISTRATION_ADDRESS:?RKE2_REGISTRATION_ADDRESS is required}"

SSH_USER="${SSH_USER:-ubuntu}"
OUTPUT="${KUBECONFIG_OUTPUT:-${HOME}/.kube/gereh-execution.yaml}"

mkdir -p "$(dirname "${OUTPUT}")"

tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT

ssh \
  "${SSH_USER}@${RKE2_ADMIN_HOST}" \
  'sudo cat /etc/rancher/rke2/rke2.yaml' \
  > "${tmp}"

sed \
  "s#https://127\.0\.0\.1:6443#https://${RKE2_REGISTRATION_ADDRESS}:6443#g" \
  "${tmp}" \
  > "${OUTPUT}"

chmod 0600 "${OUTPUT}"

echo "Kubeconfig written to:"
echo "${OUTPUT}"
