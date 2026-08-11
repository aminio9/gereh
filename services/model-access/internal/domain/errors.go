package domain

import "errors"

var (
	ErrInvalidArgument = errors.New("invalid model access argument")

	ErrNotFound = errors.New("model connection not found")

	ErrProviderNotFound = errors.New("model provider not found")

	ErrProviderDisabled = errors.New("model provider is disabled")

	ErrUnsupportedConnectionType = errors.New(
		"provider does not support this connection type",
	)

	ErrForbidden = errors.New("model access operation forbidden")

	ErrTenantNotActive = errors.New("tenant is not active")

	ErrConflict = errors.New("model access resource conflict")

	ErrVersionConflict = errors.New("model connection version conflict")

	ErrConnectionArchived = errors.New("model connection is archived")

	ErrIdempotencyConflict = errors.New(
		"idempotency key was reused with different input",
	)
)
