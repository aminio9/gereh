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

// CreateGoalInput is the input to goal creation.
type CreateGoalInput struct {
	ActorUserID string
	TenantID    string
	CompanyID   string
	Title       string
	Description string
}

// UpdateGoalInput is the input to a goal update.
type UpdateGoalInput struct {
	ActorUserID     string
	TenantID        string
	GoalID          string
	ExpectedVersion int64
	Title           *string
	Description     *string
}

// CreateGoal validates, authorizes, and commits a new goal.
func (service *Service) CreateGoal(
	ctx context.Context,
	input CreateGoalInput,
) (domain.Goal, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"company_id":    input.CompanyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Goal{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_GOAL_CREATE,
	); err != nil {
		return domain.Goal{}, err
	}

	if err := service.companyClient.EnsureCompanyActive(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.CompanyID,
	); err != nil {
		return domain.Goal{}, err
	}

	title, err := boundedText(
		"title",
		input.Title,
		1,
		200,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	description, err := boundedText(
		"description",
		input.Description,
		0,
		8000,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	goalID, err := uuid.NewV7()
	if err != nil {
		return domain.Goal{}, fmt.Errorf(
			"generate goal ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	goal := domain.Goal{
		TenantID:        input.TenantID,
		CompanyID:       input.CompanyID,
		ID:              goalID.String(),
		Title:           title,
		Description:     description,
		Status:          domain.GoalStatusActive,
		Version:         1,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		goal.ID,
		"goal.created",
		goal.TenantID,
		"goal",
		goal.ID,
		goal.Version,
		&workv1.GoalCreated{
			Goal: protoutil.Goal(goal),
		},
		now,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	return service.repository.CreateGoal(
		ctx,
		ports.CreateGoalParams{
			ActorUserID: input.ActorUserID,
			Goal:        goal,
			Event:       event,
		},
	)
}

// GetGoal returns one goal by identity.
func (service *Service) GetGoal(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	goalID string,
) (domain.Goal, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"goal_id":       goalID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Goal{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_WORK_READ,
	); err != nil {
		return domain.Goal{}, err
	}

	return service.repository.GetGoal(
		ctx,
		actorUserID,
		tenantID,
		goalID,
	)
}

// ListGoals returns a paginated goal page for a company.
func (service *Service) ListGoals(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	pageSize int32,
	pageToken string,
	includeArchived bool,
) ([]domain.Goal, string, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"company_id":    companyID,
	} {
		if err := validateUUID(name, value); err != nil {
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

	var cursor *ports.GoalCursor

	if cursorValue != "" {
		if err := validateUUID(
			"page_token",
			cursorValue,
		); err != nil {
			return nil, "", err
		}

		cursor = &ports.GoalCursor{
			GoalID: cursorValue,
		}
	}

	goals, err := service.repository.ListGoals(
		ctx,
		actorUserID,
		tenantID,
		companyID,
		limit,
		cursor,
		includeArchived,
	)
	if err != nil {
		return nil, "", err
	}

	nextToken := ""

	if len(goals) == limit && len(goals) > 0 {
		nextToken = encodePageToken(
			goals[len(goals)-1].ID,
		)
	}

	return goals, nextToken, nil
}

// UpdateGoal validates and commits a versioned goal update.
func (service *Service) UpdateGoal(
	ctx context.Context,
	input UpdateGoalInput,
) (domain.Goal, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"goal_id":       input.GoalID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Goal{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_GOAL_UPDATE,
	); err != nil {
		return domain.Goal{}, err
	}

	goal, err := service.repository.GetGoal(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.GoalID,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	if goal.Status == domain.GoalStatusArchived {
		return domain.Goal{}, domain.ErrInvalidTransition
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
			return domain.Goal{}, err
		}

		goal.Title = title
	}

	if input.Description != nil {
		description, err := boundedText(
			"description",
			*input.Description,
			0,
			8000,
		)
		if err != nil {
			return domain.Goal{}, err
		}

		goal.Description = description
	}

	goal.Version++
	goal.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		goal.ID,
		"goal.updated",
		goal.TenantID,
		"goal",
		goal.ID,
		goal.Version,
		&workv1.GoalUpdated{
			Goal:            protoutil.Goal(goal),
			UpdatedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	return service.repository.UpdateGoal(
		ctx,
		ports.UpdateGoalParams{
			ActorUserID:     input.ActorUserID,
			Goal:            goal,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)
}

// ChangeGoalStatus changes a goal lifecycle state.
func (service *Service) ChangeGoalStatus(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	goalID string,
	expectedVersion int64,
	status workv1.GoalStatus,
) (domain.Goal, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"goal_id":       goalID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Goal{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_GOAL_UPDATE,
	); err != nil {
		return domain.Goal{}, err
	}

	statusValue, err := domainGoalStatus(status)
	if err != nil {
		return domain.Goal{}, err
	}

	goal, err := service.repository.GetGoal(
		ctx,
		actorUserID,
		tenantID,
		goalID,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	previousStatus := goal.Status

	if !goalCanTransition(goal.Status, statusValue) {
		return domain.Goal{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	goal.Status = statusValue
	goal.Version++
	goal.UpdatedAt = now

	switch statusValue {
	case domain.GoalStatusCompleted:
		goal.CompletedAt = &now
		goal.ArchivedAt = nil

	case domain.GoalStatusArchived:
		goal.ArchivedAt = &now
		goal.CompletedAt = nil

	default:
		goal.CompletedAt = nil
		goal.ArchivedAt = nil
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		goal.ID,
		"goal.status_changed",
		goal.TenantID,
		"goal",
		goal.ID,
		goal.Version,
		&workv1.GoalStatusChanged{
			Goal:            protoutil.Goal(goal),
			PreviousStatus:  protoutil.GoalStatus(previousStatus),
			ChangedByUserId: actorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	return service.repository.ChangeGoalStatus(
		ctx,
		ports.UpdateGoalParams{
			ActorUserID:     actorUserID,
			Goal:            goal,
			ExpectedVersion: expectedVersion,
			Event:           event,
		},
	)
}

// goalCanTransition reports whether a goal status transition is permitted.
func goalCanTransition(
	current domain.GoalStatus,
	next domain.GoalStatus,
) bool {
	if current == next {
		return true
	}

	switch current {
	case domain.GoalStatusActive:
		return slices.Contains(
			[]domain.GoalStatus{
				domain.GoalStatusCompleted,
				domain.GoalStatusCanceled,
				domain.GoalStatusArchived,
			},
			next,
		)

	case domain.GoalStatusCompleted:
		return slices.Contains(
			[]domain.GoalStatus{
				domain.GoalStatusActive,
				domain.GoalStatusArchived,
			},
			next,
		)

	case domain.GoalStatusCanceled:
		return slices.Contains(
			[]domain.GoalStatus{
				domain.GoalStatusActive,
				domain.GoalStatusArchived,
			},
			next,
		)

	default:
		return false
	}
}
