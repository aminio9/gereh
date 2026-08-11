package domain

import "errors"

// Domain sentinel errors for Model Access operations.
var (
	// ErrInvalidArgument indicates a malformed or invalid request parameter.
	ErrInvalidArgument = errors.New("invalid model access argument")

	ErrNotFound = errors.New("model connection not found")

	ErrProviderNotFound = errors.New("model provider not found")

	ErrProviderDisabled = errors.New("model provider is disabled")

	ErrUnsupportedConnectionType = errors.New(
		"provider does not support this connection type",
	)

	ErrForbidden = errors.New("model access operation forbidden")

	ErrTenantNotActive = errors.New("tenant is not active")

	ErrPlatformManagedEntitlementRequired = errors.New(
		"platform-managed model access is not enabled for this tenant",
	)

	ErrPlatformManagedPoolUnavailable = errors.New(
		"no platform-managed provider pool is available",
	)

	ErrConflict = errors.New("model access resource conflict")

	ErrVersionConflict = errors.New("model connection version conflict")

	ErrConnectionArchived = errors.New("model connection is archived")

	ErrIdempotencyConflict = errors.New(
		"idempotency key was reused with different input",
	)

	// ErrCredentialRequired indicates a BYOK creation without a credential.
	ErrCredentialRequired = errors.New("BYOK credential is required")

	// ErrCredentialRejected indicates the provider rejected the credential.
	ErrCredentialRejected = errors.New(
		"provider rejected the credential",
	)

	// ErrCredentialVerificationUnsupported indicates the provider has no
	// supported verification contract.
	ErrCredentialVerificationUnsupported = errors.New(
		"credential verification is not supported for this provider",
	)

	// ErrCredentialVerificationUnavailable indicates a transient verification
	// failure such as provider outage or rate limit.
	ErrCredentialVerificationUnavailable = errors.New(
		"provider credential verification is temporarily unavailable",
	)

	// ErrCredentialStateConflict indicates credential state changed.
	ErrCredentialStateConflict = errors.New("credential state changed")

	// ErrSecretStoreUnavailable indicates the secret store is unreachable.
	ErrSecretStoreUnavailable = errors.New("secret store is unavailable")

	// ErrSecretStoreConflict indicates a secret-store version conflict.
	ErrSecretStoreConflict = errors.New("secret store version conflict")

	// ErrSecretNotFound indicates a secret or version does not exist.
	ErrSecretNotFound = errors.New("secret not found")
)
