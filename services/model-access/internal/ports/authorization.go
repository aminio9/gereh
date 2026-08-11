// Package ports defines Model Access service boundaries.
package ports

import (
	"context"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
)

// Authorizer performs authoritative tenant permission checks.
type Authorizer interface {
	Require(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		permission tenantv1.Permission,
	) error
}
