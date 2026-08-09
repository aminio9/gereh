package application

import (
	"context"

	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/aminio9/gereh/services/projection/internal/ports"
)

// GetDashboardSummaryInput identifies the tenant dashboard.
type GetDashboardSummaryInput struct {
	ActorUserID string
	TenantID    string
}

// GetDashboardSummary returns the tenant dashboard read model.
func (service *Service) GetDashboardSummary(
	ctx context.Context,
	input GetDashboardSummaryInput,
) (
	domain.DashboardSummary,
	domain.ProjectionMetadata,
	error,
) {
	if err := validateUUID(
		"actor_user_id",
		input.ActorUserID,
	); err != nil {
		return domain.DashboardSummary{},
			domain.ProjectionMetadata{},
			err
	}

	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return domain.DashboardSummary{},
			domain.ProjectionMetadata{},
			err
	}

	if err := service.authorizer.RequireTenantRead(
		ctx,
		input.ActorUserID,
		input.TenantID,
	); err != nil {
		return domain.DashboardSummary{},
			domain.ProjectionMetadata{},
			err
	}

	return service.repository.GetDashboardSummary(
		ctx,
		input.ActorUserID,
		input.TenantID,
	)
}

// GetCompanyOverviewInput identifies one company read model.
type GetCompanyOverviewInput struct {
	ActorUserID string
	TenantID    string
	CompanyID   string
}

// GetCompanyOverview returns one company read model.
func (service *Service) GetCompanyOverview(
	ctx context.Context,
	input GetCompanyOverviewInput,
) (
	domain.CompanyOverview,
	domain.ProjectionMetadata,
	error,
) {
	if err := validateUUID(
		"actor_user_id",
		input.ActorUserID,
	); err != nil {
		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			err
	}

	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			err
	}

	if err := validateUUID(
		"company_id",
		input.CompanyID,
	); err != nil {
		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			err
	}

	if err := service.authorizer.RequireTenantRead(
		ctx,
		input.ActorUserID,
		input.TenantID,
	); err != nil {
		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			err
	}

	return service.repository.GetCompanyOverview(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.CompanyID,
	)
}

// ListAgentOverviewsInput paginates agent read models.
type ListAgentOverviewsInput struct {
	ActorUserID string
	TenantID    string
	CompanyID   string
	PageSize    int
	Cursor      *ports.AgentCursor
}

// ListAgentOverviews returns a page of agent read models.
func (service *Service) ListAgentOverviews(
	ctx context.Context,
	input ListAgentOverviewsInput,
) (
	[]domain.AgentOverview,
	*ports.AgentCursor,
	domain.ProjectionMetadata,
	error,
) {
	if err := validateUUID(
		"actor_user_id",
		input.ActorUserID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := optionalUUID(
		"company_id",
		input.CompanyID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	var companyID *string

	if value := trimToNil(
		input.CompanyID,
	); value != nil {
		companyID = value
	}

	if err := service.authorizer.RequireTenantRead(
		ctx,
		input.ActorUserID,
		input.TenantID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	return service.repository.ListAgentOverviews(
		ctx,
		input.ActorUserID,
		input.TenantID,
		companyID,
		input.PageSize,
		input.Cursor,
	)
}

// ListTaskActivityInput paginates the tenant task activity feed.
type ListTaskActivityInput struct {
	ActorUserID string
	TenantID    string
	CompanyID   string
	TaskID      string
	PageSize    int
	Cursor      *ports.ActivityCursor
}

// ListTaskActivity returns a page of task activity feed items.
func (service *Service) ListTaskActivity(
	ctx context.Context,
	input ListTaskActivityInput,
) (
	[]domain.Activity,
	*ports.ActivityCursor,
	domain.ProjectionMetadata,
	error,
) {
	if err := validateUUID(
		"actor_user_id",
		input.ActorUserID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := optionalUUID(
		"company_id",
		input.CompanyID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := optionalUUID(
		"task_id",
		input.TaskID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := service.authorizer.RequireTenantRead(
		ctx,
		input.ActorUserID,
		input.TenantID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	var companyID *string
	var taskID *string

	if value := trimToNil(
		input.CompanyID,
	); value != nil {
		companyID = value
	}

	if value := trimToNil(
		input.TaskID,
	); value != nil {
		taskID = value
	}

	return service.repository.ListTaskActivity(
		ctx,
		input.ActorUserID,
		input.TenantID,
		companyID,
		taskID,
		input.PageSize,
		input.Cursor,
	)
}

// SearchInput paginates tenant search results.
type SearchInput struct {
	ActorUserID string
	TenantID    string
	Query       string
	CompanyID   string
	Types       []string
	PageSize    int
	Cursor      *ports.SearchCursor
}

// Search returns a page of tenant search results.
func (service *Service) Search(
	ctx context.Context,
	input SearchInput,
) (
	[]domain.SearchResult,
	*ports.SearchCursor,
	domain.ProjectionMetadata,
	error,
) {
	if err := validateUUID(
		"actor_user_id",
		input.ActorUserID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := boundedText(
		"query",
		input.Query,
		256,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := optionalUUID(
		"company_id",
		input.CompanyID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	for _, documentType := range input.Types {
		if !validDocumentType(documentType) {
			return nil, nil,
				domain.ProjectionMetadata{},
				domain.ErrInvalidArgument
		}
	}

	if err := service.authorizer.RequireTenantRead(
		ctx,
		input.ActorUserID,
		input.TenantID,
	); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	var companyID *string

	if value := trimToNil(
		input.CompanyID,
	); value != nil {
		companyID = value
	}

	return service.repository.Search(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.Query,
		companyID,
		input.Types,
		input.PageSize,
		input.Cursor,
	)
}

func validDocumentType(
	value string,
) bool {
	switch value {
	case "company",
		"agent",
		"goal",
		"project",
		"task":
		return true

	default:
		return false
	}
}

func trimToNil(value string) *string {
	if value == "" {
		return nil
	}

	cloned := value

	return &cloned
}
