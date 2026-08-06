package ports

import "context"

// EnsureDefaultCompanyRequest asks the Organization Service to idempotently
// create a tenant's default company.
type EnsureDefaultCompanyRequest struct {
	TenantID              string
	OnboardingOperationID string
	ActorUserID           string
	TenantDisplayName     string
}

// OrganizationBootstrapClient is the Organization Service internal bootstrap
// API used during tenant onboarding.
type OrganizationBootstrapClient interface {
	EnsureDefaultCompany(
		ctx context.Context,
		request EnsureDefaultCompanyRequest,
	) error
}
