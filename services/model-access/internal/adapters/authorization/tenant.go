// Package authorization adapts Tenant Service authorization.
package authorization

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
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

// Require performs the lightweight authorization-only path.
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

// RequireWithContext performs one Tenant Service call and obtains the trusted
// tenant status, region, entitlements and effective permissions together.
func (authorizer *TenantAuthorizer) RequireWithContext(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permission tenantv1.Permission,
) (ports.TenantAccessContext, error) {
	callContext, cancel := context.WithTimeout(ctx, authorizer.timeout)
	defer cancel()

	response, err := authorizer.client.GetTenantContext(
		callContext,
		&tenantv1.GetTenantContextRequest{
			ActorUserId: actorUserID,
			TenantId:    tenantID,
		},
	)
	if err != nil {
		return ports.TenantAccessContext{}, fmt.Errorf(
			"get tenant context: %w",
			err,
		)
	}

	tenantContext := response.GetContext()

	if tenantContext == nil ||
		tenantContext.GetTenant() == nil ||
		tenantContext.GetEntitlements() == nil {
		return ports.TenantAccessContext{}, errors.New(
			"Tenant Service returned incomplete tenant context",
		)
	}

	tenant := tenantContext.GetTenant()

	if !containsPermission(
		tenantContext.GetPermissions(),
		permission,
	) {
		if tenant.GetStatus() !=
			tenantv1.TenantStatus_TENANT_STATUS_ACTIVE {
			return ports.TenantAccessContext{}, domain.ErrTenantNotActive
		}

		return ports.TenantAccessContext{}, domain.ErrForbidden
	}

	entitlements := tenantContext.GetEntitlements()

	return ports.TenantAccessContext{
		Region: strings.ToLower(
			strings.TrimSpace(
				tenant.GetRegion(),
			),
		),

		PlanKey: strings.TrimSpace(
			entitlements.GetPlanKey(),
		),

		Features: maps.Clone(
			entitlements.GetFeatures(),
		),

		Limits: maps.Clone(
			entitlements.GetLimits(),
		),
	}, nil
}

func containsPermission(
	values []tenantv1.Permission,
	target tenantv1.Permission,
) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
