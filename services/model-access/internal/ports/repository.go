package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

// ConnectionCursor is an opaque pagination cursor over connection IDs.
type ConnectionCursor struct {
	ConnectionID string
}

// EventFactory builds the outbox event for a committed aggregate state.
type EventFactory func(
	domain.Connection,
) (domain.OutboxEvent, error)

// CreateConnectionParams carries a create mutation.
type CreateConnectionParams struct {
	ActorUserID string

	Connection domain.Connection

	// PlatformManagedRegion is populated only for
	// ConnectionTypePlatformManaged.
	//
	// Repository pool selection occurs inside the same transaction as
	// connection creation so an operator cannot disable/change pool
	// eligibility between selection and persistence.
	PlatformManagedRegion string

	IdempotencyKey string
	RequestHash    []byte

	IdempotencyExpiresAt time.Time

	EventFactory EventFactory
}

// UpdateConnectionParams carries an update mutation.
type UpdateConnectionParams struct {
	ActorUserID string

	TenantID     string
	ConnectionID string

	ExpectedVersion int64

	DisplayName string

	UpdatedAt time.Time

	IdempotencyKey string
	RequestHash    []byte

	IdempotencyExpiresAt time.Time

	EventFactory EventFactory
}

// ArchiveConnectionParams carries an archive mutation.
type ArchiveConnectionParams struct {
	ActorUserID string

	TenantID     string
	ConnectionID string

	ExpectedVersion int64

	ArchivedAt time.Time

	IdempotencyKey string
	RequestHash    []byte

	IdempotencyExpiresAt time.Time

	EventFactory EventFactory
}

// Repository owns Model Access persistence.
type Repository interface {
	ListProviders(
		ctx context.Context,
		actorUserID string,
		tenantID string,
	) ([]domain.Provider, error)

	CreateConnection(
		context.Context,
		CreateConnectionParams,
	) (domain.Connection, error)

	GetConnection(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		connectionID string,
	) (domain.Connection, error)

	ListConnections(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		limit int,
		cursor *ConnectionCursor,
		includeArchived bool,
	) ([]domain.Connection, error)

	UpdateConnection(
		context.Context,
		UpdateConnectionParams,
	) (domain.Connection, error)

	ArchiveConnection(
		context.Context,
		ArchiveConnectionParams,
	) (domain.Connection, error)

	ClaimOutbox(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.OutboxRecord, error)

	MarkOutboxPublished(
		ctx context.Context,
		outboxID int64,
	) error

	ReleaseOutbox(
		ctx context.Context,
		outboxID int64,
		retryAt time.Time,
		publishError string,
	) error
}
