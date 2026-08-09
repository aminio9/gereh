// Package authorization adapts Tenant Service authorization for the
// Projection Service.
package authorization

import (
	"context"
	"errors"
	"fmt"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/projection/internal/domain"
)

// TenantAuthorizer checks tenant read permissions against the Tenant
// Service.
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

// RequireTenantRead requires the tenant read permission and returns a
// domain error on denial.
func (authorizer *TenantAuthorizer) RequireTenantRead(
	ctx context.Context,
	actorUserID string,
	tenantID string,
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
			Permission:  tenantv1.Permission_PERMISSION_TENANT_READ,
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
