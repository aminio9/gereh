package application

import (
	"context"
	"fmt"
	"slices"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/aminio9/gereh/services/work-management/internal/protoutil"
	"github.com/google/uuid"
)

// CreateProjectInput is the input to project creation.
type CreateProjectInput struct {
	ActorUserID string
	TenantID    string
	CompanyID   string
	GoalID      string
	Title       string
	Description string
}

// UpdateProjectInput is the input to a project update.
type UpdateProjectInput struct {
	ActorUserID     string
	TenantID        string
	ProjectID       string
	ExpectedVersion int64
	Title           *string
	Description     *string
}

// CreateProject validates, authorizes, and commits a new project.
func (service *Service) CreateProject(
	ctx context.Context,
	input CreateProjectInput,
) (domain.Project, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"company_id":    input.CompanyID,
		"goal_id":       input.GoalID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Project{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_PROJECT_CREATE,
	); err != nil {
		return domain.Project{}, err
	}

	if err := service.companyClient.EnsureCompanyActive(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.CompanyID,
	); err != nil {
		return domain.Project{}, err
	}

	goal, err := service.repository.GetGoal(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.GoalID,
	)
	if err != nil {
		return domain.Project{}, err
	}

	if goal.CompanyID != input.CompanyID {
		return domain.Project{}, domain.ErrNotFound
	}

	if goal.Status == domain.GoalStatusArchived {
		return domain.Project{}, domain.ErrInvalidTransition
	}

	title, err := boundedText(
		"title",
		input.Title,
		1,
		200,
	)
	if err != nil {
		return domain.Project{}, err
	}

	description, err := boundedText(
		"description",
		input.Description,
		0,
		8000,
	)
	if err != nil {
		return domain.Project{}, err
	}

	projectID, err := uuid.NewV7()
	if err != nil {
		return domain.Project{}, fmt.Errorf(
			"generate project ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	project := domain.Project{
		TenantID:        input.TenantID,
		CompanyID:       input.CompanyID,
		GoalID:          input.GoalID,
		ID:              projectID.String(),
		Title:           title,
		Description:     description,
		Status:          domain.ProjectStatusPlanned,
		Version:         1,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		project.ID,
		"project.created",
		project.TenantID,
		"project",
		project.ID,
		project.Version,
		&workv1.ProjectCreated{
			Project: protoutil.Project(project),
		},
		now,
	)
	if err != nil {
		return domain.Project{}, err
	}

	return service.repository.CreateProject(
		ctx,
		ports.CreateProjectParams{
			ActorUserID: input.ActorUserID,
			Project:     project,
			Event:       event,
		},
	)
}

// GetProject returns one project by identity.
func (service *Service) GetProject(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	projectID string,
) (domain.Project, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"project_id":    projectID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Project{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_WORK_READ,
	); err != nil {
		return domain.Project{}, err
	}

	return service.repository.GetProject(
		ctx,
		actorUserID,
		tenantID,
		projectID,
	)
}

// ListProjects returns a paginated project page for a company.
func (service *Service) ListProjects(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	goalID string,
	pageSize int32,
	pageToken string,
	includeArchived bool,
) ([]domain.Project, string, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"company_id":    companyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return nil, "", err
		}
	}

	if goalID != "" {
		if err := validateUUID("goal_id", goalID); err != nil {
			return nil, "", err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_WORK_READ,
	); err != nil {
		return nil, "", err
	}

	limit := normalizePageSize(pageSize)

	cursorValue, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	var cursor *ports.ProjectCursor

	if cursorValue != "" {
		if err := validateUUID(
			"page_token",
			cursorValue,
		); err != nil {
			return nil, "", err
		}

		cursor = &ports.ProjectCursor{
			ProjectID: cursorValue,
		}
	}

	projects, err := service.repository.ListProjects(
		ctx,
		actorUserID,
		tenantID,
		companyID,
		goalID,
		limit,
		cursor,
		includeArchived,
	)
	if err != nil {
		return nil, "", err
	}

	nextToken := ""

	if len(projects) == limit && len(projects) > 0 {
		nextToken = encodePageToken(
			projects[len(projects)-1].ID,
		)
	}

	return projects, nextToken, nil
}

// UpdateProject validates and commits a versioned project update.
func (service *Service) UpdateProject(
	ctx context.Context,
	input UpdateProjectInput,
) (domain.Project, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"project_id":    input.ProjectID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Project{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_PROJECT_UPDATE,
	); err != nil {
		return domain.Project{}, err
	}

	project, err := service.repository.GetProject(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.ProjectID,
	)
	if err != nil {
		return domain.Project{}, err
	}

	if project.Status == domain.ProjectStatusArchived {
		return domain.Project{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	if input.Title != nil {
		title, err := boundedText(
			"title",
			*input.Title,
			1,
			200,
		)
		if err != nil {
			return domain.Project{}, err
		}

		project.Title = title
	}

	if input.Description != nil {
		description, err := boundedText(
			"description",
			*input.Description,
			0,
			8000,
		)
		if err != nil {
			return domain.Project{}, err
		}

		project.Description = description
	}

	project.Version++
	project.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		project.ID,
		"project.updated",
		project.TenantID,
		"project",
		project.ID,
		project.Version,
		&workv1.ProjectUpdated{
			Project:         protoutil.Project(project),
			UpdatedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Project{}, err
	}

	return service.repository.UpdateProject(
		ctx,
		ports.UpdateProjectParams{
			ActorUserID:     input.ActorUserID,
			Project:         project,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)
}

// ChangeProjectStatus changes a project lifecycle state.
func (service *Service) ChangeProjectStatus(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	projectID string,
	expectedVersion int64,
	status workv1.ProjectStatus,
) (domain.Project, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"project_id":    projectID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Project{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_PROJECT_UPDATE,
	); err != nil {
		return domain.Project{}, err
	}

	statusValue, err := domainProjectStatus(status)
	if err != nil {
		return domain.Project{}, err
	}

	project, err := service.repository.GetProject(
		ctx,
		actorUserID,
		tenantID,
		projectID,
	)
	if err != nil {
		return domain.Project{}, err
	}

	previousStatus := project.Status

	if !projectCanTransition(project.Status, statusValue) {
		return domain.Project{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	project.Status = statusValue
	project.Version++
	project.UpdatedAt = now

	switch statusValue {
	case domain.ProjectStatusCompleted:
		project.CompletedAt = &now
		project.ArchivedAt = nil

	case domain.ProjectStatusArchived:
		project.ArchivedAt = &now
		project.CompletedAt = nil

	default:
		project.CompletedAt = nil
		project.ArchivedAt = nil
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		project.ID,
		"project.status_changed",
		project.TenantID,
		"project",
		project.ID,
		project.Version,
		&workv1.ProjectStatusChanged{
			Project:         protoutil.Project(project),
			PreviousStatus:  protoutil.ProjectStatus(previousStatus),
			ChangedByUserId: actorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Project{}, err
	}

	return service.repository.ChangeProjectStatus(
		ctx,
		ports.UpdateProjectParams{
			ActorUserID:     actorUserID,
			Project:         project,
			ExpectedVersion: expectedVersion,
			Event:           event,
		},
	)
}

// projectCanTransition reports whether a project status transition is
// permitted.
func projectCanTransition(
	current domain.ProjectStatus,
	next domain.ProjectStatus,
) bool {
	if current == next {
		return true
	}

	switch current {
	case domain.ProjectStatusPlanned:
		return slices.Contains(
			[]domain.ProjectStatus{
				domain.ProjectStatusActive,
				domain.ProjectStatusOnHold,
				domain.ProjectStatusCompleted,
				domain.ProjectStatusCanceled,
				domain.ProjectStatusArchived,
			},
			next,
		)

	case domain.ProjectStatusActive:
		return slices.Contains(
			[]domain.ProjectStatus{
				domain.ProjectStatusPlanned,
				domain.ProjectStatusOnHold,
				domain.ProjectStatusCompleted,
				domain.ProjectStatusCanceled,
				domain.ProjectStatusArchived,
			},
			next,
		)

	case domain.ProjectStatusOnHold:
		return slices.Contains(
			[]domain.ProjectStatus{
				domain.ProjectStatusPlanned,
				domain.ProjectStatusActive,
				domain.ProjectStatusCompleted,
				domain.ProjectStatusCanceled,
				domain.ProjectStatusArchived,
			},
			next,
		)

	case domain.ProjectStatusCompleted:
		return slices.Contains(
			[]domain.ProjectStatus{
				domain.ProjectStatusPlanned,
				domain.ProjectStatusActive,
				domain.ProjectStatusCanceled,
				domain.ProjectStatusArchived,
			},
			next,
		)

	case domain.ProjectStatusCanceled:
		return slices.Contains(
			[]domain.ProjectStatus{
				domain.ProjectStatusPlanned,
				domain.ProjectStatusActive,
				domain.ProjectStatusArchived,
			},
			next,
		)

	default:
		return false
	}
}
