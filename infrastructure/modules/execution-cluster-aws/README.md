# Gereh AWS Execution Cluster

Creates the regional Gereh execution plane on Amazon EKS.

## Responsibilities

- Dedicated execution VPC
- Three availability zones
- Private worker subnets
- Isolated control-plane subnets
- NAT egress
- VPC Flow Logs
- Private EKS API endpoint
- EKS Access Entries
- Kubernetes secret KMS encryption
- EKS control-plane logs
- VPC CNI NetworkPolicy support
- EKS Pod Identity Agent
- System, runtime, and sandbox node pools
- Encrypted node root volumes
- IMDSv2 enforcement

## Non-responsibilities

This module intentionally does not create:

- RuntimeCell CRDs
- Runtime Cell Manager deployments
- OpenClaw cells
- Hermes cells
- Sandbox workloads
- Workspace PVCs
- tenant namespaces
- model/tool gateway routes

Those belong to later Runtime Plane phases.

## Security boundary

No untrusted tenant workloads may run on the execution cluster until the
RuntimeCell controller, policy allowlists, strict CNI enforcement, runtime
security contexts, and admission controls are installed.
