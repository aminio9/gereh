package ports

import "context"

// Authorizer requires one tenant read permission for a human query.
type Authorizer interface {
	RequireTenantRead(
		ctx context.Context,
		actorUserID string,
		tenantID string,
	) error
}
