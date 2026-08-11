// Package domain contains Model Access business entities.
package domain

import "time"

// ConnectionType identifies who owns the provider relationship.
type ConnectionType string

// Supported connection types.
const (
	// ConnectionTypePlatformManaged indicates a Gereh-managed provider pool.
	ConnectionTypePlatformManaged ConnectionType = "platform_managed"
	// ConnectionTypeBYOK indicates tenant-supplied credentials.
	ConnectionTypeBYOK ConnectionType = "byok"
	// ConnectionTypePrivateEndpoint indicates a tenant private endpoint.
	ConnectionTypePrivateEndpoint ConnectionType = "private_endpoint"
)

// Valid reports whether the connection type is recognized.
func (value ConnectionType) Valid() bool {
	switch value {
	case ConnectionTypePlatformManaged,
		ConnectionTypeBYOK,
		ConnectionTypePrivateEndpoint:
		return true

	default:
		return false
	}
}

// ConnectionStatus describes connection control-plane state.
type ConnectionStatus string

// Supported connection statuses.
const (
	// ConnectionStatusDraft indicates an incomplete connection awaiting verification.
	ConnectionStatusDraft               ConnectionStatus = "draft"
	ConnectionStatusPendingVerification ConnectionStatus = "pending_verification"
	ConnectionStatusActive              ConnectionStatus = "active"
	ConnectionStatusVerificationFailed  ConnectionStatus = "verification_failed"
	ConnectionStatusDisabled            ConnectionStatus = "disabled"
	ConnectionStatusArchived            ConnectionStatus = "archived"
)

// Provider contains non-secret platform provider metadata.
type Provider struct {
	Key         string
	DisplayName string
	Description string

	SupportedConnectionTypes []ConnectionType

	Enabled bool

	Version int64

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Connection is the business identity of a model connection.
//
// ProviderPoolKey is internal routing metadata used only for
// platform-managed connections.
//
// It MUST NOT be added to the public ModelConnection protobuf.
//
// Raw provider credentials must never be added to this structure.
type Connection struct {
	TenantID string
	ID       string

	ProviderKey string

	ConnectionType ConnectionType

	// ProviderPoolKey is populated only for a Gereh-managed connection.
	//
	// It is not secret material, but it is internal platform topology and
	// therefore intentionally absent from public APIs and Kafka payloads.
	ProviderPoolKey *string

	DisplayName string

	Status ConnectionStatus

	Version int64

	CreatedByUserID string

	CreatedAt  time.Time
	UpdatedAt  time.Time
	ArchivedAt *time.Time
}

// ProviderPool is an internal Gereh-managed routing pool.
//
// It describes eligibility/routing only. It contains no provider credential.
type ProviderPool struct {
	Key string

	ProviderKey string

	Regions []string

	Enabled bool

	Priority int

	Version int64

	CreatedAt time.Time
	UpdatedAt time.Time
}
