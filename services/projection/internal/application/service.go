// Package application implements the Projection Service use cases.
package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/aminio9/gereh/services/projection/internal/ports"
)

// Config defines Projection Service application settings.
type Config struct {
	// ServicePrincipalID is the service principal used to apply domain
	// events within a tenant scope.
	ServicePrincipalID string
}

// Service implements the Projection Service use cases.
type Service struct {
	repository ports.Repository
	authorizer ports.Authorizer
	config     Config
}

// New creates the application service.
func New(
	repository ports.Repository,
	authorizer ports.Authorizer,
	config Config,
) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf(
			"projection repository is required",
		)
	}

	if authorizer == nil {
		return nil, fmt.Errorf(
			"projection authorizer is required",
		)
	}

	if strings.TrimSpace(
		config.ServicePrincipalID,
	) == "" {
		return nil, fmt.Errorf(
			"projection service principal ID is required",
		)
	}

	if err := validateUUID(
		"service_principal_id",
		config.ServicePrincipalID,
	); err != nil {
		return nil, err
	}

	return &Service{
		repository: repository,
		authorizer: authorizer,
		config:     config,
	}, nil
}

// Project applies one consumed domain event inside a checkpointed
// tenant transaction.
//
// The event is applied with the configured service-scoped principal.
// The returned boolean reports whether the event was applied for the
// first time (true) or was a duplicate delivery (false).
func (service *Service) Project(
	ctx context.Context,
	domainEvent domain.EventMeta,
	apply ports.ApplyFunc,
) (bool, error) {
	if err := validateUUID(
		"tenant_id",
		domainEvent.TenantID,
	); err != nil {
		return false, err
	}

	applied, err := service.repository.ApplyEvent(
		ctx,
		service.config.ServicePrincipalID,
		domainEvent,
		apply,
	)
	if err != nil {
		return false, fmt.Errorf(
			"apply projection event: %w",
			err,
		)
	}

	return applied, nil
}
