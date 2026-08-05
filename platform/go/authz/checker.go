package authz

import (
	"context"
	"fmt"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"google.golang.org/grpc"
)

// TenantClient is implemented by the generated Tenant Service client.
type TenantClient interface {
	CheckAuthorization(
		context.Context,
		*tenantv1.CheckAuthorizationRequest,
		...grpc.CallOption,
	) (*tenantv1.CheckAuthorizationResponse, error)
}

// Checker evaluates tenant permissions.
type Checker interface {
	Check(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		permission tenantv1.Permission,
	) (Decision, bool, error)
}

// GRPCChecker evaluates permissions through Tenant Service.
type GRPCChecker struct {
	client  TenantClient
	timeout time.Duration
}

// NewGRPCChecker creates a fail-closed authorization checker.
func NewGRPCChecker(
	client TenantClient,
	timeout time.Duration,
) (*GRPCChecker, error) {
	if client == nil {
		return nil, fmt.Errorf(
			"tenant service client is required",
		)
	}

	if timeout <= 0 {
		return nil, fmt.Errorf(
			"authorization timeout must be greater than zero",
		)
	}

	return &GRPCChecker{
		client:  client,
		timeout: timeout,
	}, nil
}

// Check evaluates a permission.
func (checker *GRPCChecker) Check(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permission tenantv1.Permission,
) (Decision, bool, error) {
	checkContext := ctx
	cancel := func() {}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		checkContext, cancel =
			context.WithTimeout(
				ctx,
				checker.timeout,
			)
	}

	defer cancel()

	response, err :=
		checker.client.CheckAuthorization(
			checkContext,
			&tenantv1.CheckAuthorizationRequest{
				ActorUserId: actorUserID,
				TenantId:    tenantID,
				Permission:  permission,
			},
		)
	if err != nil {
		return Decision{}, false, fmt.Errorf(
			"check tenant authorization: %w",
			err,
		)
	}

	value := response.GetDecision()
	if value == nil {
		return Decision{}, false, fmt.Errorf(
			"tenant service returned no authorization decision",
		)
	}

	decision := Decision{
		ActorUserID:       value.GetActorUserId(),
		TenantID:          value.GetTenantId(),
		Role:              value.GetRole(),
		Permission:        value.GetPermission(),
		TenantVersion:     value.GetTenantVersion(),
		MembershipVersion: value.GetMembershipVersion(),
	}

	return decision, value.GetAllowed(), nil
}
