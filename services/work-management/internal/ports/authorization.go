package ports

import (
	"context"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
)

// Authorizer checks one tenant permission against the Tenant Service.
//
// The Work Management Service performs the authoritative authorization check
// on every request. BFF middleware may reject early, but it is never trusted.
type Authorizer interface {
	Require(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		permission tenantv1.Permission,
	) error
}
