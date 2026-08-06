// Package ports defines the execution-orchestrator hexagonal ports.
package ports

import "context"

// EnsureTenantRuntimeRequest asks the Runtime Manager to converge the
// desired runtime state for one tenant.
type EnsureTenantRuntimeRequest struct {
	TenantID      string
	OperationID   string
	Region        string
	IsolationTier string
}

// RuntimeProvisioner provisions tenant runtime infrastructure. Implementations
// must be idempotent by operation_id.
type RuntimeProvisioner interface {
	EnsureTenantRuntime(
		ctx context.Context,
		request EnsureTenantRuntimeRequest,
	) error
}
