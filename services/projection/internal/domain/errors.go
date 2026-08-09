package domain

import "errors"

var (
	// ErrInvalidArgument rejects malformed projection requests.
	ErrInvalidArgument = errors.New(
		"invalid projection argument",
	)

	// ErrNotFound means the projection resource does not exist.
	ErrNotFound = errors.New(
		"projection resource not found",
	)

	// ErrForbidden means the actor is not allowed to read the projection.
	ErrForbidden = errors.New(
		"projection access forbidden",
	)

	// ErrTenantNotActive means the tenant is not available for reads.
	ErrTenantNotActive = errors.New(
		"tenant is not active",
	)

	// ErrEventIdentityConflict means the same event ID was reused with
	// different content. This is a data-integrity incident and must stop
	// the consumer.
	ErrEventIdentityConflict = errors.New(
		"event ID was reused with different content",
	)
)
