package domain

import "errors"

var (
	// ErrInvalidArgument indicates invalid client input.
	ErrInvalidArgument = errors.New(
		"invalid organization argument",
	)

	// ErrNotFound indicates a company or agent does not exist.
	ErrNotFound = errors.New(
		"organization resource not found",
	)

	// ErrForbidden indicates the actor lacks the required permission.
	ErrForbidden = errors.New(
		"organization operation forbidden",
	)

	// ErrTenantNotActive indicates the tenant is not active.
	ErrTenantNotActive = errors.New(
		"tenant is not active",
	)

	// ErrConflict indicates a uniqueness or current-state conflict.
	ErrConflict = errors.New(
		"organization resource conflict",
	)

	// ErrVersionConflict indicates a stale optimistic-lock version.
	ErrVersionConflict = errors.New(
		"organization version conflict",
	)

	// ErrHierarchyCycle indicates the reporting hierarchy would cycle.
	ErrHierarchyCycle = errors.New(
		"agent reporting hierarchy would contain a cycle",
	)

	// ErrAgentHasReports indicates an agent still has direct reports.
	ErrAgentHasReports = errors.New(
		"agent still has direct reports",
	)

	// ErrCompanyHasAgents indicates a company still has active agents.
	ErrCompanyHasAgents = errors.New(
		"company still has active agents",
	)

	// ErrInvalidTransition indicates an invalid lifecycle transition.
	ErrInvalidTransition = errors.New(
		"invalid agent lifecycle transition",
	)

	// ErrDefaultCompany indicates the default company cannot be archived.
	ErrDefaultCompany = errors.New(
		"default company cannot be archived",
	)
)
