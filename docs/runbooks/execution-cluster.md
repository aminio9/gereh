# Execution Cluster Runbook

## Scope

This runbook manages the Gereh regional execution Kubernetes plane.

The execution cluster is separate from the shared Gereh control plane and is
intended to host OpenClaw, Hermes, runtime support services, and isolated
execution sandboxes in later phases.

## Prerequisites

- AWS CLI
- Terraform 1.15.x
- kubectl compatible with Kubernetes 1.36
- jq
- AWS identity permitted to assume the infrastructure role
- VPC connectivity for Kubernetes API operations

## Plan

```bash
export TF_VAR_cluster_admin_role_arn="arn:aws:iam::<account>:role/GerehEKSAdmin"

cd infrastructure/live/dev/execution-cluster

terraform init \
  -backend-config=backend.hcl

terraform fmt -check

terraform validate

terraform plan
```

## Apply

```bash
terraform apply
```

Production infrastructure should normally be applied through the protected  
GitHub Actions environment rather than directly from a workstation.

## Configure Kubernetes access

```bash
export AWS_REGION=eu-north-1
export EXECUTION_CLUSTER_NAME=gereh-dev-exec-eu-north-1

aws eks update-kubeconfig \
  --region "${AWS_REGION}" \
  --name "${EXECUTION_CLUSTER_NAME}"
```

The Kubernetes API endpoint is private, so this command requires VPC  
connectivity for subsequent kubectl operations.

## Bootstrap cluster policy

```bash
task infra:execution:bootstrap
```

## Verify

```bash
task infra:execution:verify
```

## Required invariants

- EKS status is ACTIVE.
- Kubernetes API is private.
- Public Kubernetes API is disabled.
- Kubernetes version is the approved version.
- system, runtime, and sandbox node groups exist.
- runtime and sandbox pools are tainted.
- EKS control-plane logging is enabled.
- VPC CNI NetworkPolicy support is enabled.
- Pod Identity Agent is ACTIVE.
- execution namespaces use restricted Pod Security.
- default-deny NetworkPolicies are installed.
- no tenant runtime workloads exist before Runtime Plane Phase 23.

## Failure handling

If Terraform apply fails:

1. Do not manually create replacement cloud resources.
2. Inspect the Terraform error and AWS service status.
3. Re-run `terraform plan`.
4. Ensure the proposed reconciliation is safe.
5. Re-apply through the same state backend.

If Kubernetes bootstrap fails:

1. Confirm VPC/private API connectivity.
2. Confirm the caller is mapped through an EKS Access Entry.
3. Run `aws eks describe-cluster`.
4. Run `kubectl get --raw=/readyz`.
5. Inspect add-on status.
6. Re-run bootstrap; all manifests are declarative and idempotent.

## Destruction

Production destruction must not be automated.

Deletion protection must be explicitly disabled in reviewed Terraform before  
the production cluster can be destroyed.
