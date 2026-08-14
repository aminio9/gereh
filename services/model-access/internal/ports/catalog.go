package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

type ListOfferingsParams struct {
	ActorUserID string
	TenantID    string

	ConnectionID    string
	AgentUsableOnly bool

	Limit  int
	Cursor *OfferingCursor
}

type OfferingCursor struct {
	OfferingID string
}

type ApplyCatalogRefreshParams struct {
	ActorUserID  string
	TenantID     string
	ConnectionID string
	RefreshID    string

	Discovered []domain.DiscoveredModel

	Source domain.OfferingSource

	RefreshedAt time.Time
}

type CatalogRepository interface {
	ListOfferings(
		ctx context.Context,
		params ListOfferingsParams,
	) ([]domain.ModelOffering, error)

	GetOffering(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		offeringID string,
	) (domain.ModelOffering, error)

	EnqueueCatalogRefresh(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		connectionID string,
		requestedAt time.Time,
	) (domain.CatalogRefresh, error)

	GetCatalogRefresh(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		refreshID string,
	) (domain.CatalogRefresh, error)

	ClaimCatalogRefresh(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.CatalogRefreshJob, error)

	ApplyCatalogRefresh(
		ctx context.Context,
		params ApplyCatalogRefreshParams,
	) (domain.CatalogRefreshResult, error)

	FailCatalogRefresh(
		ctx context.Context,
		refreshID string,
		tenantID string,
		actorUserID string,
		connectionID string,
		errorCode string,
		failedAt time.Time,
	) error

	ReleaseCatalogRefresh(
		ctx context.Context,
		refreshID string,
		retryAt time.Time,
		message string,
	) error

	MarkConnectionOfferingsUnavailable(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		connectionID string,
		unavailableAt time.Time,
	) error
}

type ProviderCatalogClient interface {
	DiscoverModels(
		ctx context.Context,
		providerKey string,
		apiKey []byte,
	) ([]domain.DiscoveredModel, error)
}

type StaticCatalogLoader interface {
	LoadPlatformOfferings(
		providerKey string,
		poolKey string,
	) ([]domain.DiscoveredModel, error)

	FindCuratedMetadata(
		providerKey string,
		providerModelID string,
	) *domain.DiscoveredModel
}
