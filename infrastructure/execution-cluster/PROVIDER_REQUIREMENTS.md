# Gereh Execution Cloud Requirements

The cloud provider is intentionally outside the Kubernetes implementation.
A provider is acceptable for the Gereh execution plane only when it provides
the following infrastructure primitives.

## Required

- Linux virtual machines.
- Private layer-3 networking between all cluster nodes.
- Stable private IPv4 addresses.
- SSD-backed storage for RKE2 server nodes.
- Cloud firewall/security-group equivalent.
- Ability to prevent public access to Kubernetes control-plane ports.
- Outbound HTTPS or access to an internal package/image mirror.
- Snapshots or backup capability for VM/block storage.

## Production

Production additionally requires:

- At least three control-plane VMs.
- Failure-domain separation where the provider supports it.
- Stable private registration endpoint implemented by one of:
  - private L4 load balancer;
  - floating/virtual IP;
  - stable private DNS over healthy server endpoints.
- At least two runtime workers.
- At least two sandbox workers.
- Ability to replace a VM while preserving the private-network architecture.

## Kubernetes networking

The provider firewall must permit only private cluster traffic required by RKE2 and Cilium.

### RKE2 server ports

- TCP 6443: Kubernetes API.
- TCP 9345: RKE2 supervisor / node registration.
- TCP 2379, 2380, 2381: etcd traffic between control-plane nodes.
- TCP 10250: kubelet communication/metrics between cluster nodes.

### Cilium VXLAN

- UDP 8472: Cilium VXLAN between cluster nodes.
- TCP 4240: Cilium health monitoring.
- ICMP type 0/8: Optional Cilium node health checks.

These ports must never be exposed indiscriminately to the public Internet.

## Public exposure

Do not publicly expose:
- 2379
- 2380
- 2381
- 8472
- 9345
- 10250

The Kubernetes API on 6443 should normally be reachable only through the
administrative VPN/private network. SSH should be restricted to a bastion/VPN/administrative source.

## NodePort

The execution plane does not rely on publicly exposed NodePort services.

## Cloud-specific functionality

Provider-specific block-storage CSI, snapshot controllers, load balancers and
object storage are adapters. They must not leak into Gereh domain or runtime contracts.
