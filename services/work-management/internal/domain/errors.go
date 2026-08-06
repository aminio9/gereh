package domain

import "errors"

// Domain errors for Work Management Service operations.
var (
	ErrInvalidArgument = errors.New("invalid work argument")
	ErrNotFound        = errors.New("work resource not found")
	ErrForbidden       = errors.New("work operation forbidden")
	ErrTenantNotActive = errors.New("tenant is not active")
	ErrConflict        = errors.New("work resource conflict")
	ErrVersionConflict = errors.New("work version conflict")

	ErrCompanyNotActive  = errors.New("company is not active")
	ErrAgentNotUsable    = errors.New("agent cannot receive work")
	ErrDependencyCycle   = errors.New("task dependency would create a cycle")
	ErrTaskBlocked       = errors.New("task has incomplete dependencies")
	ErrChecklistOpen     = errors.New("task has incomplete checklist items")
	ErrProjectOpenTasks  = errors.New("project has incomplete tasks")
	ErrGoalOpenProjects  = errors.New("goal has incomplete projects")
	ErrInvalidTransition = errors.New("invalid work status transition")
	ErrCommentOwnership  = errors.New("comment is owned by another principal")
)
