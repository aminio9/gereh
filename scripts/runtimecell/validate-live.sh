#!/usr/bin/env bash

set -Eeuo pipefail

namespace="gereh-runtimecell-contract-$RANDOM"
cell_name="runtimecell-contract"
tenant_id="2a27c31f-9a73-4977-8e4f-b49135380cc7"
operation_id="749694fc-a57d-4622-b503-900c2353ec1e"

cleanup() {
  kubectl delete namespace \
    "${namespace}" \
    --ignore-not-found \
    --wait=false >/dev/null 2>&1 || true
}

trap cleanup EXIT

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

kubectl get --raw=/readyz >/dev/null

kubectl create namespace \
  "${namespace}"

cat <<EOF | kubectl apply -f -
apiVersion: runtime.gereh.ai/v1
kind: RuntimeCell
metadata:
  name: ${cell_name}
  namespace: ${namespace}
spec:
  tenantId: ${tenant_id}
  operationId: ${operation_id}
  runtime: openclaw
  version: "2026.07"
  isolationTier: standard
  storageClass: test-rwo
  region: ir-thr-1
EOF

echo "Checking selectable operationId field..."

selected="$(
  kubectl \
    -n "${namespace}" \
    get runtimecells \
    --field-selector "spec.operationId=${operation_id}" \
    -o name
)"

[[ "${selected}" == "runtimecell.runtime.gereh.ai/${cell_name}" ]] \
  || fail "operationId field selector did not return RuntimeCell"

echo "Checking invalid runtime rejection..."

if cat <<EOF | kubectl apply --dry-run=server -f - >/dev/null 2>&1
apiVersion: runtime.gereh.ai/v1
kind: RuntimeCell
metadata:
  name: invalid-runtime
  namespace: ${namespace}
spec:
  tenantId: ${tenant_id}
  operationId: invalid-runtime-test
  runtime: arbitrary-container
  version: "1"
  isolationTier: standard
  storageClass: test-rwo
  region: ir-thr-1
EOF
then
  fail "kube-apiserver accepted an invalid runtime"
fi

echo "Checking immutable region..."

if kubectl \
  -n "${namespace}" \
  patch runtimecell "${cell_name}" \
  --type merge \
  -p '{"spec":{"region":"ir-mhd-1"}}' \
  >/dev/null 2>&1
then
  fail "kube-apiserver accepted immutable region mutation"
fi

echo "Checking mutable runtime version..."

kubectl \
  -n "${namespace}" \
  patch runtimecell "${cell_name}" \
  --type merge \
  -p '{"spec":{"version":"2026.08"}}' \
  >/dev/null

version="$(
  kubectl \
    -n "${namespace}" \
    get runtimecell "${cell_name}" \
    -o jsonpath='{.spec.version}'
)"

[[ "${version}" == "2026.08" ]] \
  || fail "runtime version update did not persist"

echo "Checking status subresource..."

generation="$(
  kubectl \
    -n "${namespace}" \
    get runtimecell "${cell_name}" \
    -o jsonpath='{.metadata.generation}'
)"

kubectl \
  -n "${namespace}" \
  patch runtimecell "${cell_name}" \
  --subresource=status \
  --type merge \
  -p "{
    \"status\": {
      \"observedGeneration\": ${generation},
      \"phase\": \"Ready\",
      \"observedRuntimeVersion\": \"2026.08\",
      \"conditions\": [
        {
          \"type\": \"Ready\",
          \"status\": \"True\",
          \"reason\": \"ContractTestReady\",
          \"message\": \"RuntimeCell API contract test completed.\",
          \"lastTransitionTime\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
          \"observedGeneration\": ${generation}
        }
      ]
    }
  }" \
  >/dev/null

phase="$(
  kubectl \
    -n "${namespace}" \
    get runtimecell "${cell_name}" \
    -o jsonpath='{.status.phase}'
)"

[[ "${phase}" == "Ready" ]] \
  || fail "status subresource did not persist Ready phase"

echo
echo "RuntimeCell Kubernetes API contract validation succeeded."
