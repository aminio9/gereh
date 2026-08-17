#!/usr/bin/env bash

set -Eeuo pipefail

: "${EXECUTION_CLUSTER_NAME:?EXECUTION_CLUSTER_NAME is required}"
: "${AWS_REGION:?AWS_REGION is required}"

for command in aws kubectl jq; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "required command is missing: ${command}" >&2
    exit 1
  fi
done

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

echo "Checking EKS cluster status..."

cluster_status="$(
  aws eks describe-cluster \
    --name "${EXECUTION_CLUSTER_NAME}" \
    --region "${AWS_REGION}" \
    --query 'cluster.status' \
    --output text
)"

[[ "${cluster_status}" == "ACTIVE" ]] \
  || fail "cluster status is ${cluster_status}, expected ACTIVE"

echo "Checking private API configuration..."

endpoint_private="$(
  aws eks describe-cluster \
    --name "${EXECUTION_CLUSTER_NAME}" \
    --region "${AWS_REGION}" \
    --query 'cluster.resourcesVpcConfig.endpointPrivateAccess' \
    --output text
)"

endpoint_public="$(
  aws eks describe-cluster \
    --name "${EXECUTION_CLUSTER_NAME}" \
    --region "${AWS_REGION}" \
    --query 'cluster.resourcesVpcConfig.endpointPublicAccess' \
    --output text
)"

[[ "${endpoint_private}" == "True" ]] \
  || fail "private EKS API endpoint is not enabled"

[[ "${endpoint_public}" == "False" ]] \
  || fail "public EKS API endpoint must be disabled"

echo "Checking Kubernetes API..."

kubectl \
  --request-timeout=10s \
  get \
  --raw=/readyz >/dev/null \
  || fail "Kubernetes API is not ready"

echo "Waiting for nodes..."

kubectl wait \
  --for=condition=Ready \
  node \
  --all \
  --timeout=10m

echo "Checking node pools..."

for pool in system runtime sandbox; do
  count="$(
    kubectl get nodes \
      -l "gereh.ai/node-pool=${pool}" \
      -o json |
      jq '.items | length'
  )"

  [[ "${count}" -gt 0 ]] \
    || fail "no ready node exists in ${pool} node pool"
done

echo "Checking runtime taints..."

kubectl get nodes \
  -l gereh.ai/node-pool=runtime \
  -o json |
  jq -e '
    .items | length > 0 and
    all(
      .[];
      any(
        .spec.taints[]?;
        .key == "gereh.ai/workload"
        and .value == "runtime"
        and .effect == "NoSchedule"
      )
    )
  ' >/dev/null \
  || fail "runtime node taint missing"

echo "Checking sandbox taints..."

kubectl get nodes \
  -l gereh.ai/node-pool=sandbox \
  -o json |
  jq -e '
    .items | length > 0 and
    all(
      .[];
      any(
        .spec.taints[]?;
        .key == "gereh.ai/workload"
        and .value == "sandbox"
        and .effect == "NoSchedule"
      )
    )
  ' >/dev/null \
  || fail "sandbox node taint missing"

echo "Checking EKS add-ons..."

for addon in \
  coredns \
  kube-proxy \
  vpc-cni \
  eks-pod-identity-agent
do
  status="$(
    aws eks describe-addon \
      --cluster-name "${EXECUTION_CLUSTER_NAME}" \
      --addon-name "${addon}" \
      --region "${AWS_REGION}" \
      --query 'addon.status' \
      --output text
  )"

  [[ "${status}" == "ACTIVE" ]] \
    || fail "${addon} status is ${status}, expected ACTIVE"
done

echo "Checking VPC CNI NetworkPolicy agent..."

containers="$(
  kubectl \
    -n kube-system \
    get daemonset aws-node \
    -o json |
    jq -r '.spec.template.spec.containers[].name'
)"

grep -qx 'aws-network-policy-agent' <<<"${containers}" \
  || fail "aws-network-policy-agent container is missing"

echo "Checking namespace Pod Security..."

for namespace in \
  gereh-runtime-system \
  gereh-runtime-cells \
  gereh-sandboxes
do
  enforce="$(
    kubectl get namespace "${namespace}" \
      -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}'
  )"

  [[ "${enforce}" == "restricted" ]] \
    || fail "${namespace}: Pod Security enforce must be restricted"

  kubectl \
    -n "${namespace}" \
    get networkpolicy default-deny >/dev/null \
    || fail "${namespace}: default-deny policy missing"

  kubectl \
    -n "${namespace}" \
    get networkpolicy allow-cluster-dns >/dev/null \
    || fail "${namespace}: DNS policy missing"
done

echo
echo "Execution cluster validation successful."
