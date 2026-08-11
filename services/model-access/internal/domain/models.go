// Package domain contains Model Access business entities.
package domain

import "time"

// ConnectionType identifies who owns the provider relationship.
type ConnectionType string

const (
	ConnectionTypePlatformManaged ConnectionType = "platform_managed"
	ConnectionTypeBYOK            ConnectionType = "byok"
	ConnectionTypePrivateEndpoint ConnectionType = "private_endpoint"
)

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

const (
	ConnectionStatusDraft              ConnectionStatus = "draft"
	ConnectionStatusPendingVerification ConnectionStatus = "pending_verification"
	ConnectionStatusActive             ConnectionStatus = "active"
	ConnectionStatusVerificationFailed ConnectionStatus = "verification_failed"
	ConnectionStatusDisabled           ConnectionStatus = "disabled"
	ConnectionStatusArchived           ConnectionStatus = "archived"
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

// Connection is the public/business identity of a model connection.
//
// Do not add raw provider credentials to this structure in later phases.
// Phase 18 secret references belong to a separate internal record.
type Connection struct {
	TenantID string
	ID       string

	ProviderKey string

	ConnectionType ConnectionType

	DisplayName string

	Status ConnectionStatus

	Version int64

	CreatedByUserID string

	CreatedAt  time.Time
	UpdatedAt  time.Time
	ArchivedAt *time.Time
}
