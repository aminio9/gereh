// Package authorization adapts Tenant Service authorization for the
// Work Management Service.
package authorization

import (
	"context"
	"errors"
	"fmt"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
)

// TenantAuthorizer checks permissions against the Tenant Service.
type TenantAuthorizer struct {
	client  tenantv1.TenantServiceClient
	timeout time.Duration
}

// NewTenantAuthorizer creates a Tenant Service-backed authorizer.
func NewTenantAuthorizer(
	client tenantv1.TenantServiceClient,
	timeout time.Duration,
) *TenantAuthorizer {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &TenantAuthorizer{
		client:  client,
		timeout: timeout,
	}
}

// Require checks one tenant permission and returns a domain error on denial.
func (authorizer *TenantAuthorizer) Require(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permission tenantv1.Permission,
) error {
	callContext, cancel := context.WithTimeout(
		ctx,
		authorizer.timeout,
	)
	defer cancel()

	response, err := authorizer.client.CheckAuthorization(
		callContext,
		&tenantv1.CheckAuthorizationRequest{
			ActorUserId: actorUserID,
			TenantId:    tenantID,
			Permission:  permission,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"check tenant authorization: %w",
			err,
		)
	}

	decision := response.GetDecision()

	if decision == nil {
		return errors.New(
			"tenant service returned no authorization decision",
		)
	}

	if decision.GetAllowed() {
		return nil
	}

	switch decision.GetDenialReason() {
	case tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_TENANT_NOT_ACTIVE,
		tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_TENANT_ARCHIVED:
		return domain.ErrTenantNotActive

	default:
		return domain.ErrForbidden
	}
}
