package grpc

import (
	"errors"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mapError converts domain errors into gRPC status errors without exposing
// internal detail.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(
			codes.InvalidArgument,
			"invalid Model Access request",
		)

	case errors.Is(err, domain.ErrNotFound),
		errors.Is(err, domain.ErrProviderNotFound):
		return status.Error(
			codes.NotFound,
			"Model Access resource not found",
		)

	case errors.Is(err, domain.ErrPlatformManagedEntitlementRequired):
		return status.Error(
			codes.PermissionDenied,
			"platform-managed model access is not enabled for this tenant",
		)

	case errors.Is(err, domain.ErrForbidden):
		return status.Error(
			codes.PermissionDenied,
			"Model Access operation forbidden",
		)

	case errors.Is(err, domain.ErrTenantNotActive),
		errors.Is(err, domain.ErrProviderDisabled),
		errors.Is(err, domain.ErrUnsupportedConnectionType),
		errors.Is(err, domain.ErrConnectionArchived):
		return status.Error(
			codes.FailedPrecondition,
			err.Error(),
		)

	case errors.Is(err, domain.ErrVersionConflict):
		return status.Error(
			codes.Aborted,
			"Model connection changed; reload and retry",
		)

	case errors.Is(err, domain.ErrConflict):
		return status.Error(
			codes.AlreadyExists,
			"Model Access resource already exists",
		)

	case errors.Is(err, domain.ErrIdempotencyConflict):
		return status.Error(
			codes.AlreadyExists,
			"idempotency key was reused with different input",
		)

	case errors.Is(err, domain.ErrPlatformManagedPoolUnavailable):
		return status.Error(
			codes.Unavailable,
			"platform-managed model access is currently unavailable",
		)

	default:
		return status.Error(
			codes.Internal,
			"internal Model Access Service error",
		)
	}
}
