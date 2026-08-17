#!/usr/bin/env bash

set -Eeuo pipefail

: "${KUBECONFIG:?KUBECONFIG is required}"

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

echo "Checking Kubernetes API..."

kubectl \
  --request-timeout=10s \
  get \
  --raw=/readyz >/dev/null \
  || fail "Kubernetes API is not ready"

echo "Checking nodes..."

kubectl wait \
  --for=condition=Ready \
  node \
  --all \
  --timeout=10m

for pool in system runtime sandbox; do
  count="$(
    kubectl get nodes \
      -l "gereh.ai/node-pool=${pool}" \
      -o json |
      jq '.items | length'
  )"

  if [[ "${count}" -lt 1 ]]; then
    fail "No node exists in ${pool} node pool"
  fi
done

echo "Checking runtime taints..."

kubectl get nodes \
  -l gereh.ai/node-pool=runtime \
  -o json |
  jq -e '
    all(
      .items[];
      any(
        .spec.taints[]?;
        .key == "gereh.ai/workload"
        and .value == "runtime"
        and .effect == "NoSchedule"
      )
    )
  ' >/dev/null \
  || fail "runtime taint is missing"

echo "Checking sandbox taints..."

kubectl get nodes \
  -l gereh.ai/node-pool=sandbox \
  -o json |
  jq -e '
    all(
      .items[];
      any(
        .spec.taints[]?;
        .key == "gereh.ai/workload"
        and .value == "sandbox"
        and .effect == "NoSchedule"
      )
    )
  ' >/dev/null \
  || fail "sandbox taint is missing"

for namespace in \
  gereh-runtime-system \
  gereh-runtime-cells \
  gereh-sandboxes
do
  echo "Checking ${namespace}..."

  enforce="$(
    kubectl get namespace "${namespace}" \
      -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}'
  )"

  [[ "${enforce}" == "restricted" ]] \
    || fail "${namespace} does not enforce restricted Pod Security"

  kubectl \
    -n "${namespace}" \
    get networkpolicy default-deny >/dev/null \
    || fail "${namespace}: default-deny missing"

  kubectl \
    -n "${namespace}" \
    get networkpolicy allow-cluster-dns >/dev/null \
    || fail "${namespace}: DNS policy missing"

  automount="$(
    kubectl \
      -n "${namespace}" \
      get serviceaccount default \
      -o jsonpath='{.automountServiceAccountToken}'
  )"

  [[ "${automount}" == "false" ]] \
    || fail "${namespace}: default ServiceAccount token automount enabled"
done

echo
echo "Gereh execution cluster verification successful."
