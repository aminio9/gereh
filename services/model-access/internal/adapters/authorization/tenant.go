// Package authorization adapts Tenant Service authorization.
package authorization

import (
	"context"
	"errors"
	"fmt"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

// TenantAuthorizer performs authoritative tenant permission checks.
type TenantAuthorizer struct {
	client tenantv1.TenantServiceClient

	timeout time.Duration
}

// NewTenantAuthorizer wraps a Tenant Service client with a bounded timeout.
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

// Require checks a tenant permission and maps denial to domain errors.
func (authorizer *TenantAuthorizer) Require(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permission tenantv1.Permission,
) error {
	callContext, cancel := context.WithTimeout(ctx, authorizer.timeout)
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
		return fmt.Errorf("check tenant authorization: %w", err)
	}

	decision := response.GetDecision()

	if decision == nil {
		return errors.New(
			"Tenant Service returned no authorization decision",
		)
	}

	if decision.GetAllowed() {
		return nil
	}

	switch decision.GetDenialReason() {
	case tenantv1.
		AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_TENANT_NOT_ACTIVE,
		tenantv1.
			AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_TENANT_ARCHIVED:
		return domain.ErrTenantNotActive

	default:
		return domain.ErrForbidden
	}
}
