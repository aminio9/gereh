package domain

import "errors"

var (
	// ErrInvalidArgument indicates invalid client input.
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrNotFound indicates that a tenant or membership does not exist.
	ErrNotFound = errors.New("tenant resource not found")

	// ErrForbidden indicates that the actor lacks the required role.
	ErrForbidden = errors.New("tenant operation forbidden")

	// ErrConflict indicates a uniqueness or current-state conflict.
	ErrConflict = errors.New("tenant resource conflict")

	// ErrVersionConflict indicates a stale optimistic-lock version.
	ErrVersionConflict = errors.New("tenant version conflict")

	// ErrLastOwner indicates that an operation would remove the final owner.
	ErrLastOwner = errors.New("tenant must retain at least one owner")

	// ErrArchived indicates that the tenant no longer accepts mutations.
	ErrArchived = errors.New("tenant is archived")
)
