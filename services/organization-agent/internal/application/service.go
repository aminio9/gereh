package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/aminio9/gereh/services/organization-agent/internal/ports"
)

// Config defines the Company and Agent Service application settings.
type Config struct {
	CompanyEventTopic           string
	AgentEventTopic             string
	BootstrapServicePrincipalID string
}

// Service implements the Company and Agent Service use cases.
type Service struct {
	repository ports.Repository
	authorizer ports.Authorizer
	config     Config
	now        func() time.Time
}

// New creates the application service.
func New(
	repository ports.Repository,
	authorizer ports.Authorizer,
	config Config,
) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf(
			"organization repository is required",
		)
	}

	if authorizer == nil {
		return nil, fmt.Errorf(
			"tenant authorizer is required",
		)
	}

	if strings.TrimSpace(
		config.CompanyEventTopic,
	) == "" {
		return nil, fmt.Errorf(
			"company event topic is required",
		)
	}

	if strings.TrimSpace(
		config.AgentEventTopic,
	) == "" {
		return nil, fmt.Errorf(
			"agent event topic is required",
		)
	}

	if err := validateUUID(
		"bootstrap_service_principal_id",
		config.BootstrapServicePrincipalID,
	); err != nil {
		return nil, err
	}

	return &Service{
		repository: repository,
		authorizer: authorizer,
		config:     config,
		now:        time.Now,
	}, nil
}
