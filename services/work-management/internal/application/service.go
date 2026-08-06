package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/aminio9/gereh/services/work-management/internal/ports"
)

// Config defines the Work Management Service application settings.
type Config struct {
	EventTopic string
}

// Service implements the Work Management Service use cases.
type Service struct {
	repository    ports.Repository
	authorizer    ports.Authorizer
	companyClient ports.CompanyClient
	config        Config
	now           func() time.Time
}

// New creates the application service.
func New(
	repository ports.Repository,
	authorizer ports.Authorizer,
	companyClient ports.CompanyClient,
	config Config,
) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf(
			"work repository is required",
		)
	}

	if authorizer == nil {
		return nil, fmt.Errorf(
			"tenant authorizer is required",
		)
	}

	if companyClient == nil {
		return nil, fmt.Errorf(
			"organization company client is required",
		)
	}

	if strings.TrimSpace(config.EventTopic) == "" {
		return nil, fmt.Errorf(
			"work event topic is required",
		)
	}

	return &Service{
		repository:    repository,
		authorizer:    authorizer,
		companyClient: companyClient,
		config:        config,
		now:           time.Now,
	}, nil
}
