# Gereh Execution Kubernetes Cluster

Provider-neutral execution plane infrastructure for Gereh.

Supports:
- **Mode A**: Arvan Private Kubernetes (managed with native admin kubeconfig)
- **Mode B**: Hardened RKE2 on Arvan Cloud Servers, Derak, or other Iranian VPS providers via Ansible

## Structure

- `inventories/`: Environment host inventories
- `playbooks/`: Ansible orchestration for host preparation, RKE2 install, Kubernetes policy bootstrap, and verification
- `templates/`: CIS-hardened RKE2 configs, audit policy, registries, and host sysctl
