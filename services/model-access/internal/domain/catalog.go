package domain

import "time"

type OfferingStatus string

const (
	OfferingStatusAvailable   OfferingStatus = "available"
	OfferingStatusUnavailable OfferingStatus = "unavailable"
)

type OfferingSource string

const (
	OfferingSourceProviderDiscovered OfferingSource = "provider_discovered"
	OfferingSourcePlatformCatalog    OfferingSource = "platform_catalog"
)

type ModelOffering struct {
	TenantID string
	ID       string

	ConnectionID string

	ProviderKey     string
	ProviderModelID string

	DisplayName string
	Description string

	Status OfferingStatus
	Source OfferingSource

	AgentUsable bool

	Capabilities []string

	InputModalities  []string
	OutputModalities []string

	ContextWindowTokens int64
	MaxOutputTokens     int64

	Version int64

	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	RefreshedAt   time.Time
	UnavailableAt *time.Time
}

// DiscoveredModel is provider/platform-catalog input before a stable
// tenant offering ID is assigned.
type DiscoveredModel struct {
	ProviderKey     string
	ProviderModelID string

	DisplayName string
	Description string

	AgentUsable bool

	Capabilities []string

	InputModalities  []string
	OutputModalities []string

	ContextWindowTokens int64
	MaxOutputTokens     int64

	ProviderCreatedAt *time.Time
}

type CatalogRefreshStatus string

const (
	CatalogRefreshPending   CatalogRefreshStatus = "pending"
	CatalogRefreshRunning   CatalogRefreshStatus = "running"
	CatalogRefreshSucceeded CatalogRefreshStatus = "succeeded"
	CatalogRefreshFailed    CatalogRefreshStatus = "failed"
)

type CatalogRefresh struct {
	TenantID string
	ID       string

	ActorUserID string

	ConnectionID string

	Status CatalogRefreshStatus

	Generation int64

	DiscoveredCount  int
	AvailableCount   int
	UnavailableCount int

	ErrorCode string

	RequestedAt time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type CatalogRefreshJob struct {
	RefreshID string

	TenantID     string
	ActorUserID  string
	ConnectionID string

	Attempts int
}

type CatalogRefreshResult struct {
	Generation int64

	DiscoveredCount  int
	AvailableCount   int
	UnavailableCount int

	RefreshedAt time.Time
}
