# Execution Cluster Runbook

## Scope

This runbook manages the Gereh regional execution Kubernetes plane.

The execution cluster is separate from the shared Gereh control plane and is
intended to host OpenClaw, Hermes, runtime support services, and isolated
execution sandboxes in later phases.

## Modes

- **Mode A**: Arvan Private Kubernetes (managed with native admin kubeconfig)
- **Mode B**: Hardened RKE2 on Arvan Cloud Servers, Derak, or other Iranian VPS providers via Ansible

## Prerequisites

- SSH access to cluster nodes (Mode B)
- Python 3.13+ with `ansible-core`
- `kubectl` compatible with Kubernetes 1.36
- `jq`
- Valid private network connectivity / VPN

## Mode B: Provisioning RKE2 Cluster

1. Prepare environment inventory:
```bash
cd infrastructure/execution-cluster
cp inventories/dev/hosts.yml.example inventories/dev/hosts.yml
```

2. Set required environment variables:
```bash
export RKE2_TOKEN="$(openssl rand -hex 48)"
export RKE2_REGISTRATION_ADDRESS="10.80.0.11"
```

3. Run node preparation:
```bash
ansible-playbook -i inventories/dev/hosts.yml playbooks/prepare.yml
```

4. Install RKE2:
```bash
ansible-playbook -i inventories/dev/hosts.yml playbooks/install.yml
```

5. Bootstrap policies:
```bash
ansible-playbook -i inventories/dev/hosts.yml playbooks/bootstrap.yml
```

6. Verify cluster health:
```bash
ansible-playbook -i inventories/dev/hosts.yml playbooks/verify.yml
```

## Mode A: Arvan Private Kubernetes Bootstrap

If using Arvan Private Kubernetes with admin kubeconfig:

```bash
export KUBECONFIG="$HOME/.kube/gereh-arvan-execution.yaml"
bash scripts/infra/execution-cluster/bootstrap.sh
```

## Required Invariants

- Kubernetes version is pinned (v1.36.3).
- CIS profile is active (RKE2 Mode B).
- Secrets at rest are encrypted.
- System, runtime, and sandbox node pools exist.
- Runtime and sandbox node pools are tainted with `gereh.ai/workload=<pool>:NoSchedule`.
- `gereh-runtime-system`, `gereh-runtime-cells`, and `gereh-sandboxes` namespaces enforce Restricted Pod Security.
- Default ServiceAccounts have `automountServiceAccountToken: false`.
- Default-deny NetworkPolicies are installed in all execution namespaces.
- DNS is the only baseline allowed egress.
