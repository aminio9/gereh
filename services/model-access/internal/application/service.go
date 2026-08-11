// Package application implements Model Access use cases.
package application

import (
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/ports"
)

// Config configures the Model Access application service.
type Config struct {
	EventTopic string

	IdempotencyTTL time.Duration
}

// Service coordinates authorization, validation and persistence.
type Service struct {
	repository ports.Repository
	authorizer ports.Authorizer

	config Config

	now func() time.Time
}

// New validates dependencies and constructs the application service.
func New(
	repository ports.Repository,
	authorizer ports.Authorizer,
	config Config,
) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("model access repository is required")
	}

	if authorizer == nil {
		return nil, fmt.Errorf("model access authorizer is required")
	}

	if config.EventTopic == "" {
		return nil, fmt.Errorf("Model Access event topic is required")
	}

	if config.IdempotencyTTL <= 0 {
		return nil, fmt.Errorf("Model Access idempotency TTL must be positive")
	}

	return &Service{
		repository: repository,
		authorizer: authorizer,
		config:     config,
		now:        time.Now,
	}, nil
}
