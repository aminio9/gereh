package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/aminio9/gereh/services/policy-approval/internal/engine"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/aminio9/gereh/services/policy-approval/internal/security"
)

// Config defines the Policy Service application settings.
type Config struct {
	EventTopic                   string
	EvaluationServicePrincipalID string
	BootstrapServicePrincipalID  string
	DecisionTTL                  time.Duration
}

// Service implements the Policy Service use cases.
type Service struct {
	repository         ports.Repository
	authorizer         ports.Authorizer
	organizationClient ports.PolicyContextClient
	evaluator          *engine.Evaluator
	signer             *security.Signer
	config             Config
	now                func() time.Time
}

// New creates the application service.
func New(
	repository ports.Repository,
	authorizer ports.Authorizer,
	organizationClient ports.PolicyContextClient,
	evaluator *engine.Evaluator,
	signer *security.Signer,
	config Config,
) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf(
			"policy repository is required",
		)
	}

	if authorizer == nil {
		return nil, fmt.Errorf(
			"tenant authorizer is required",
		)
	}

	if organizationClient == nil {
		return nil, fmt.Errorf(
			"organization policy-context client is required",
		)
	}

	if evaluator == nil {
		return nil, fmt.Errorf(
			"policy evaluator is required",
		)
	}

	if signer == nil {
		return nil, fmt.Errorf(
			"decision signer is required",
		)
	}

	if strings.TrimSpace(config.EventTopic) == "" {
		return nil, fmt.Errorf(
			"policy event topic is required",
		)
	}

	if err := validateUUID(
		"evaluation_service_principal_id",
		config.EvaluationServicePrincipalID,
	); err != nil {
		return nil, err
	}

	if err := validateUUID(
		"bootstrap_service_principal_id",
		config.BootstrapServicePrincipalID,
	); err != nil {
		return nil, err
	}

	if config.DecisionTTL <= 0 {
		return nil, fmt.Errorf(
			"policy decision TTL must be positive",
		)
	}

	return &Service{
		repository:         repository,
		authorizer:         authorizer,
		organizationClient: organizationClient,
		evaluator:          evaluator,
		signer:             signer,
		config:             config,
		now:                time.Now,
	}, nil
}
