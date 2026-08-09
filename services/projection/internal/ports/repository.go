// Package ports defines the Projection Service boundaries.
package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/projection/internal/domain"
)

// ProjectionTransaction is the tenant-scoped transaction used by one event
// projection.
type ProjectionTransaction interface {
	UpsertTenant(
		ctx context.Context,
		value domain.Tenant,
	) error

	UpsertCompany(
		ctx context.Context,
		value domain.Company,
	) error

	UpsertAgent(
		ctx context.Context,
		value domain.Agent,
	) error

	UpsertGoal(
		ctx context.Context,
		value domain.Goal,
	) error

	UpsertProject(
		ctx context.Context,
		value domain.Project,
	) error

	UpsertTask(
		ctx context.Context,
		value domain.Task,
	) error

	UpsertDependency(
		ctx context.Context,
		value domain.Dependency,
	) error

	DeleteDependency(
		ctx context.Context,
		tenantID string,
		taskID string,
		dependsOnTaskID string,
	) error

	UpsertAssignment(
		ctx context.Context,
		value domain.Assignment,
	) error

	DeleteAssignment(
		ctx context.Context,
		tenantID string,
		assignmentID string,
	) error

	AppendActivity(
		ctx context.Context,
		value domain.Activity,
	) error

	UpsertSearchDocument(
		ctx context.Context,
		value domain.SearchDocument,
	) error
}

// ApplyFunc mutates projection rows inside the event transaction.
type ApplyFunc func(
	ctx context.Context,
	transaction ProjectionTransaction,
) error

// AgentCursor is the opaque keyset cursor for agent overviews.
type AgentCursor struct {
	UpdatedAt time.Time
	AgentID   string
}

// ActivityCursor is the opaque keyset cursor for the activity feed.
type ActivityCursor struct {
	OccurredAt time.Time
	EventID    string
}

// SearchCursor is the opaque keyset cursor for search results.
type SearchCursor struct {
	Rank      float64
	UpdatedAt time.Time
	Type      string
	ID        string
}

// Repository persists and queries read models.
type Repository interface {
	ApplyEvent(
		ctx context.Context,
		servicePrincipalID string,
		event domain.EventMeta,
		apply ApplyFunc,
	) (bool, error)

	GetDashboardSummary(
		ctx context.Context,
		actorUserID string,
		tenantID string,
	) (
		domain.DashboardSummary,
		domain.ProjectionMetadata,
		error,
	)

	GetCompanyOverview(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
	) (
		domain.CompanyOverview,
		domain.ProjectionMetadata,
		error,
	)

	ListAgentOverviews(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID *string,
		pageSize int,
		cursor *AgentCursor,
	) (
		[]domain.AgentOverview,
		*AgentCursor,
		domain.ProjectionMetadata,
		error,
	)

	ListTaskActivity(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID *string,
		taskID *string,
		pageSize int,
		cursor *ActivityCursor,
	) (
		[]domain.Activity,
		*ActivityCursor,
		domain.ProjectionMetadata,
		error,
	)

	Search(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		query string,
		companyID *string,
		types []string,
		pageSize int,
		cursor *SearchCursor,
	) (
		[]domain.SearchResult,
		*SearchCursor,
		domain.ProjectionMetadata,
		error,
	)
}
