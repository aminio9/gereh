package ports

import "context"

// EnsureDefaultPoliciesRequest asks the Policy Service to idempotently create
// a tenant's default policies.
type EnsureDefaultPoliciesRequest struct {
	TenantID              string
	OnboardingOperationID string
	ActorUserID           string
}

// PolicyBootstrapClient is the Policy Service internal bootstrap API used
// during tenant onboarding.
type PolicyBootstrapClient interface {
	EnsureDefaultPolicies(
		ctx context.Context,
		request EnsureDefaultPoliciesRequest,
	) error
}
