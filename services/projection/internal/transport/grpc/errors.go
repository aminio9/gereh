// Package grpc exposes the Projection Service over gRPC.
package grpc

import (
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/projection/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mapError converts a domain error into a gRPC status error.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, domain.ErrTenantNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())

	default:
		return fmt.Errorf("projection service error: %w", err)
	}
}
