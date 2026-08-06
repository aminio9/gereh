package ports

import "context"

// CompanyClient validates company state through the Organization Service.
type CompanyClient interface {
	// EnsureCompanyActive returns nil only when the company exists and is
	// active in the tenant. Errors map to Work Management domain errors.
	EnsureCompanyActive(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
	) error
}
