package authz

import (
	"context"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
)

type decisionContextKey struct{}

// Decision is a trusted authorization result attached to a request.
type Decision struct {
	ActorUserID       string
	TenantID          string
	Role              tenantv1.TenantRole
	Permission        tenantv1.Permission
	TenantVersion     int64
	MembershipVersion int64
}

// WithDecision stores a trusted authorization decision.
func WithDecision(
	ctx context.Context,
	decision Decision,
) context.Context {
	return context.WithValue(
		ctx,
		decisionContextKey{},
		decision,
	)
}

// DecisionFromContext returns the trusted authorization decision.
func DecisionFromContext(
	ctx context.Context,
) (Decision, bool) {
	decision, ok := ctx.Value(
		decisionContextKey{},
	).(Decision)

	return decision, ok
}
