package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
)

// CheckAuthorization evaluates one tenant permission.
func (service *Service) CheckAuthorization(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permission domain.Permission,
) (domain.AuthorizationDecision, error) {
	if err := validateAuthorizationInput(
		actorUserID,
		tenantID,
		permission,
	); err != nil {
		return domain.AuthorizationDecision{}, err
	}

	contextValue, err :=
		service.repository.GetTenantContext(
			ctx,
			tenantID,
			actorUserID,
		)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.AuthorizationDecision{
			Allowed:      false,
			TenantID:     tenantID,
			ActorUserID:  actorUserID,
			Permission:   permission,
			DenialReason: domain.DenialReasonNotMember,
		}, nil
	}

	if err != nil {
		return domain.AuthorizationDecision{}, err
	}

	return domain.EvaluateAuthorization(
		contextValue.Tenant,
		contextValue.Membership,
		permission,
	), nil
}

// BatchCheckAuthorization evaluates multiple permissions using one database
// lookup.
func (service *Service) BatchCheckAuthorization(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permissions []domain.Permission,
) ([]domain.AuthorizationDecision, error) {
	if len(permissions) == 0 {
		return nil, fmt.Errorf(
			"%w: at least one permission is required",
			domain.ErrInvalidArgument,
		)
	}

	if len(permissions) > 64 {
		return nil, fmt.Errorf(
			"%w: no more than 64 permissions may be checked",
			domain.ErrInvalidArgument,
		)
	}

	if err := validateUUID(
		"actor_user_id",
		actorUserID,
	); err != nil {
		return nil, err
	}

	if err := validateUUID(
		"tenant_id",
		tenantID,
	); err != nil {
		return nil, err
	}

	for _, permission := range permissions {
		if !domain.IsKnownPermission(permission) {
			return nil, fmt.Errorf(
				"%w: unsupported permission %q",
				domain.ErrInvalidArgument,
				permission,
			)
		}
	}

	contextValue, err :=
		service.repository.GetTenantContext(
			ctx,
			tenantID,
			actorUserID,
		)
	if errors.Is(err, domain.ErrNotFound) {
		decisions := make(
			[]domain.AuthorizationDecision,
			0,
			len(permissions),
		)

		for _, permission := range permissions {
			decisions = append(
				decisions,
				domain.AuthorizationDecision{
					Allowed:      false,
					TenantID:     tenantID,
					ActorUserID:  actorUserID,
					Permission:   permission,
					DenialReason: domain.DenialReasonNotMember,
				},
			)
		}

		return decisions, nil
	}

	if err != nil {
		return nil, err
	}

	decisions := make(
		[]domain.AuthorizationDecision,
		0,
		len(permissions),
	)

	for _, permission := range permissions {
		decisions = append(
			decisions,
			domain.EvaluateAuthorization(
				contextValue.Tenant,
				contextValue.Membership,
				permission,
			),
		)
	}

	return decisions, nil
}

func validateAuthorizationInput(
	actorUserID string,
	tenantID string,
	permission domain.Permission,
) error {
	if err := validateUUID(
		"actor_user_id",
		actorUserID,
	); err != nil {
		return err
	}

	if err := validateUUID(
		"tenant_id",
		tenantID,
	); err != nil {
		return err
	}

	if !domain.IsKnownPermission(permission) {
		return fmt.Errorf(
			"%w: unsupported permission %q",
			domain.ErrInvalidArgument,
			permission,
		)
	}

	return nil
}

func decorateTenantContext(
	contextValue domain.TenantContext,
) domain.TenantContext {
	contextValue.Permissions =
		domain.EffectivePermissions(
			contextValue.Tenant.Status,
			contextValue.Membership.Role,
		)

	return contextValue
}

func requirePermission(
	contextValue domain.TenantContext,
	permission domain.Permission,
) error {
	decision := domain.EvaluateAuthorization(
		contextValue.Tenant,
		contextValue.Membership,
		permission,
	)

	if decision.Allowed {
		return nil
	}

	if decision.DenialReason ==
		domain.DenialReasonTenantArchived {
		return domain.ErrArchived
	}

	return domain.ErrForbidden
}
