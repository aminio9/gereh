// Package domain contains execution-orchestrator business types.
package domain

// ProvisionTenantInput is the input to the tenant provisioning workflow.
type ProvisionTenantInput struct {
	TenantID    string
	OperationID string
	Region      string
}

// OperationFailure is safe customer-visible failure information. It must not
// contain provider credentials, stack traces, or unredacted upstream data.
type OperationFailure struct {
	Code      string
	Message   string
	Retryable bool
	Details   map[string]any
}
