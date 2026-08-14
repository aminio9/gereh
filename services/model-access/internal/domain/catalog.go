package domain

import "time"

// OfferingStatus represents the availability status of a model offering.
type OfferingStatus string

const (
	// OfferingStatusAvailable indicates the model offering is active and queryable.
	OfferingStatusAvailable OfferingStatus = "available"
	// OfferingStatusUnavailable indicates the model offering is currently disabled or removed upstream.
	OfferingStatusUnavailable OfferingStatus = "unavailable"
)

// OfferingSource indicates whether a model offering was discovered from a provider or platform catalog.
type OfferingSource string

const (
	// OfferingSourceProviderDiscovered indicates the model was discovered via provider API.
	OfferingSourceProviderDiscovered OfferingSource = "provider_discovered"
	// OfferingSourcePlatformCatalog indicates the model was configured in the static platform catalog.
	OfferingSourcePlatformCatalog OfferingSource = "platform_catalog"
)

// ModelOffering represents a tenant-accessible model offering.
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

// CatalogRefreshStatus represents the execution state of a catalog refresh job.
type CatalogRefreshStatus string

const (
	// CatalogRefreshPending indicates the refresh job is waiting to be processed.
	CatalogRefreshPending CatalogRefreshStatus = "pending"
	// CatalogRefreshRunning indicates the refresh job is currently executing.
	CatalogRefreshRunning CatalogRefreshStatus = "running"
	// CatalogRefreshSucceeded indicates the refresh job completed successfully.
	CatalogRefreshSucceeded CatalogRefreshStatus = "succeeded"
	// CatalogRefreshFailed indicates the refresh job encountered an unrecoverable failure.
	CatalogRefreshFailed CatalogRefreshStatus = "failed"
)

// CatalogRefresh tracks the status and metadata of a catalog refresh operation.
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

// CatalogRefreshJob represents an item claimed from the catalog refresh queue.
type CatalogRefreshJob struct {
	RefreshID string

	TenantID     string
	ActorUserID  string
	ConnectionID string

	Attempts int
}

// CatalogRefreshResult summarizes the changes applied during a catalog refresh.
type CatalogRefreshResult struct {
	Generation int64

	DiscoveredCount  int
	AvailableCount   int
	UnavailableCount int

	RefreshedAt time.Time
}
