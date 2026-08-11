package ports

import "context"

// SecretStore is the encrypted external secret-of-record for BYOK.
type SecretStore interface {
	Reference(
		tenantID string,
		connectionID string,
	) string

	WriteCAS(
		ctx context.Context,
		secretRef string,
		credential []byte,
		expectedVersion int64,
	) (
		version int64,
		err error,
	)

	ReadVersion(
		ctx context.Context,
		secretRef string,
		version int64,
	) ([]byte, error)

	CurrentVersion(
		ctx context.Context,
		secretRef string,
	) (int64, error)

	DestroyVersion(
		ctx context.Context,
		secretRef string,
		version int64,
	) error

	Purge(
		ctx context.Context,
		secretRef string,
	) error
}
