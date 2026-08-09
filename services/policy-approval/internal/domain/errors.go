package domain

import "errors"

var (
	// ErrInvalidArgument indicates an invalid policy request argument.
	ErrInvalidArgument = errors.New("invalid policy argument")

	// ErrNotFound indicates a policy resource was not found.
	ErrNotFound = errors.New("policy resource not found")

	// ErrForbidden indicates the operation is not permitted.
	ErrForbidden = errors.New("policy operation forbidden")

	// ErrTenantNotActive indicates the tenant is not active.
	ErrTenantNotActive = errors.New("tenant is not active")

	// ErrConflict indicates a conflicting policy resource.
	ErrConflict = errors.New("policy resource conflict")

	// ErrVersionConflict indicates an optimistic-lock version mismatch.
	ErrVersionConflict = errors.New("policy resource version conflict")

	// ErrInvalidExpression indicates an invalid policy expression.
	ErrInvalidExpression = errors.New("invalid policy expression")

	// ErrExpressionCostExceeded indicates the expression exceeded cost bounds.
	ErrExpressionCostExceeded = errors.New("policy expression cost exceeded")

	// ErrInvalidConstraint indicates an invalid policy constraint.
	ErrInvalidConstraint = errors.New("invalid policy constraint")

	// ErrPolicyNotActive indicates the policy is not active.
	ErrPolicyNotActive = errors.New("policy is not active")

	// ErrInvalidTransition indicates an invalid policy status transition.
	ErrInvalidTransition = errors.New("invalid policy transition")

	// ErrDecisionMismatch indicates a request reused with different input.
	ErrDecisionMismatch = errors.New("evaluation request was reused with different input")
)
