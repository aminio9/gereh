// Package grpc exposes the Policy Approval Service over gRPC.
package grpc

import (
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mapError converts a domain error into a gRPC status error.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrInvalidArgument),
		errors.Is(err, domain.ErrInvalidExpression),
		errors.Is(err, domain.ErrInvalidConstraint):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, domain.ErrNotFound),
		errors.Is(err, domain.ErrPolicyNotActive):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, domain.ErrTenantNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, domain.ErrConflict),
		errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrDecisionMismatch):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, domain.ErrVersionConflict):
		return status.Error(codes.Aborted, err.Error())

	case errors.Is(err, domain.ErrExpressionCostExceeded):
		return status.Error(codes.ResourceExhausted, err.Error())

	default:
		return fmt.Errorf("policy service error: %w", err)
	}
}
