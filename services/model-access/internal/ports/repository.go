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

// EnsureBYOKCredentialParams carries BYOK credential metadata creation.
type EnsureBYOKCredentialParams struct {
	ActorUserID string

	TenantID     string
	ConnectionID string

	SecretRef string

	Fingerprint []byte

	FingerprintDisplay string
	FingerprintKeyID   string

	Now time.Time
}

// ActivateBYOKParams activates a verified BYOK connection.
type ActivateBYOKParams struct {
	ActorUserID string

	TenantID     string
	ConnectionID string

	VaultVersion int64

	Fingerprint        []byte
	FingerprintDisplay string

	VerifiedAt time.Time

	Verification domain.CredentialVerification

	EventFactory EventFactory
}

// FailInitialBYOKParams marks initial BYOK verification as failed.
type FailInitialBYOKParams struct {
	ActorUserID string

	TenantID     string
	ConnectionID string

	VaultVersion int64

	Verification domain.CredentialVerification

	FailedAt time.Time

	EventFactory EventFactory
}

// PrepareRotationParams prepares a BYOK credential rotation.
type PrepareRotationParams struct {
	ActorUserID string

	TenantID     string
	ConnectionID string

	ExpectedVersion int64

	IdempotencyKey string
	RequestHash    []byte

	NewFingerprint        []byte
	NewFingerprintDisplay string

	Now       time.Time
	ExpiresAt time.Time
}

// RotationPreparation is the outcome of PrepareBYOKRotation.
type RotationPreparation struct {
	Connection domain.Connection
	Credential domain.BYOKCredential
	Operation  domain.CredentialOperation
}

// CompleteRotationParams completes a verified BYOK rotation.
type CompleteRotationParams struct {
	ActorUserID string

	TenantID     string
	ConnectionID string

	IdempotencyKey string

	NewVaultVersion int64

	NewFingerprint        []byte
	NewFingerprintDisplay string

	VerifiedAt time.Time

	Verification domain.CredentialVerification

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

	EnsureBYOKCredential(
		context.Context,
		EnsureBYOKCredentialParams,
	) (domain.BYOKCredential, error)

	GetBYOKCredential(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		connectionID string,
	) (domain.BYOKCredential, error)

	MarkBYOKSecretStored(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		connectionID string,
		vaultVersion int64,
		now time.Time,
	) (domain.BYOKCredential, error)

	ActivateBYOK(
		context.Context,
		ActivateBYOKParams,
	) (domain.Connection, error)

	FailInitialBYOKVerification(
		context.Context,
		FailInitialBYOKParams,
	) (domain.Connection, error)

	RecordTransientVerification(
		ctx context.Context,
		verification domain.CredentialVerification,
	) error

	PrepareBYOKRotation(
		context.Context,
		PrepareRotationParams,
	) (RotationPreparation, error)

	MarkBYOKRotationSecretStored(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		connectionID string,
		idempotencyKey string,
		vaultVersion int64,
		now time.Time,
	) error

	CompleteBYOKRotation(
		context.Context,
		CompleteRotationParams,
	) (domain.Connection, error)

	RejectBYOKRotation(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		connectionID string,
		idempotencyKey string,
		vaultVersion int64,
		verification domain.CredentialVerification,
		now time.Time,
	) error

	ClaimSecretCleanup(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.SecretCleanup, error)

	CompleteSecretCleanup(
		ctx context.Context,
		cleanupID int64,
	) error

	ReleaseSecretCleanup(
		ctx context.Context,
		cleanupID int64,
		retryAt time.Time,
		message string,
	) error

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
