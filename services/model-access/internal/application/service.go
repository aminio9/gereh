// Package application implements Model Access use cases.
package application

import (
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/aminio9/gereh/services/model-access/internal/security"
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

	secretStore ports.SecretStore
	verifier    ports.CredentialVerifier

	fingerprinter *security.Fingerprinter

	catalogClient  ports.ProviderCatalogClient
	staticCatalog  ports.StaticCatalogLoader
	agentDirectory ports.AgentDirectory

	config Config

	now func() time.Time
}

// Option allows configuring optional dependencies on Service.
type Option func(*Service)

func WithCatalogClient(client ports.ProviderCatalogClient) Option {
	return func(s *Service) {
		s.catalogClient = client
	}
}

func WithStaticCatalog(staticCat ports.StaticCatalogLoader) Option {
	return func(s *Service) {
		s.staticCatalog = staticCat
	}
}

func WithAgentDirectory(directory ports.AgentDirectory) Option {
	return func(s *Service) {
		s.agentDirectory = directory
	}
}

// New validates dependencies and constructs the application service.
func New(
	repository ports.Repository,
	authorizer ports.Authorizer,
	secretStore ports.SecretStore,
	verifier ports.CredentialVerifier,
	fingerprinter *security.Fingerprinter,
	config Config,
	opts ...Option,
) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("model access repository is required")
	}

	if authorizer == nil {
		return nil, fmt.Errorf("model access authorizer is required")
	}

	if secretStore == nil {
		return nil, fmt.Errorf("model access secret store is required")
	}

	if verifier == nil {
		return nil, fmt.Errorf("model access credential verifier is required")
	}

	if fingerprinter == nil {
		return nil, fmt.Errorf("model access credential fingerprinter is required")
	}

	if config.EventTopic == "" {
		return nil, fmt.Errorf("model access event topic is required")
	}

	if config.IdempotencyTTL <= 0 {
		return nil, fmt.Errorf("model access idempotency TTL must be positive")
	}

	svc := &Service{
		repository:    repository,
		authorizer:    authorizer,
		secretStore:   secretStore,
		verifier:      verifier,
		fingerprinter: fingerprinter,
		config:        config,
		now:           time.Now,
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc, nil
}
