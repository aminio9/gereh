package domain

import (
	"errors"
	"time"
)

// OperationState is the durable state of an asynchronous command.
type OperationState string

// Operation states.
const (
	OperationStatePending   OperationState = "pending"
	OperationStateRunning   OperationState = "running"
	OperationStateSucceeded OperationState = "succeeded"
	OperationStateFailed    OperationState = "failed"
	OperationStateCanceled  OperationState = "canceled"
)

// OperationError is safe customer-visible failure information.
//
// Do not store stack traces, SQL, provider credentials, access tokens,
// Kubernetes objects, or unredacted upstream responses here.
type OperationError struct {
	Code      string
	Message   string
	Retryable bool
	Details   map[string]any
}

// Operation represents one accepted asynchronous tenant command.
type Operation struct {
	ID          string
	TenantID    string
	ActorUserID string
	RequestID   string

	State        OperationState
	ResourceName string

	WorkflowID    string
	WorkflowRunID string

	Error    *OperationError
	Metadata map[string]string

	Version int64

	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// Operation transition errors.
var (
	ErrInvalidOperationTransition = errors.New(
		"invalid operation state transition",
	)
	ErrOperationAlreadyCompleted = errors.New(
		"operation is already completed",
	)
)
