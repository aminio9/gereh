// Package ports defines Model Access service boundaries.
package ports

import (
	"context"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
)

// TenantAccessContext contains the tenant-owned business context that
// Model Access is permitted to depend on synchronously.
//
// Maps are copies of Tenant Service data; callers must treat them as
// point-in-time authorization/entitlement state.
type TenantAccessContext struct {
	Region string

	PlanKey string

	Features map[string]bool
	Limits   map[string]int64
}

// Authorizer performs authoritative Tenant Service checks.
type Authorizer interface {
	Require(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		permission tenantv1.Permission,
	) error

	RequireWithContext(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		permission tenantv1.Permission,
	) (TenantAccessContext, error)
}
