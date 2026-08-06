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

// CreateTaskInput is the input to task creation.
type CreateTaskInput struct {
	ActorUserID  string
	TenantID     string
	CompanyID    string
	ProjectID    string
	ParentTaskID *string
	Title        string
	Description  string
	Priority     workv1.TaskPriority
}

// UpdateTaskInput is the input to a task update.
type UpdateTaskInput struct {
	ActorUserID     string
	TenantID        string
	TaskID          string
	ExpectedVersion int64
	Title           *string
	Description     *string
	Priority        *domain.TaskPriority
}

// CreateTask validates, authorizes, and commits a new task.
func (service *Service) CreateTask(
	ctx context.Context,
	input CreateTaskInput,
) (domain.TaskView, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"company_id":    input.CompanyID,
		"project_id":    input.ProjectID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.TaskView{}, err
		}
	}

	if input.ParentTaskID != nil {
		if err := validateUUID(
			"parent_task_id",
			*input.ParentTaskID,
		); err != nil {
			return domain.TaskView{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_CREATE,
	); err != nil {
		return domain.TaskView{}, err
	}

	if err := service.companyClient.EnsureCompanyActive(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.CompanyID,
	); err != nil {
		return domain.TaskView{}, err
	}

	project, err := service.repository.GetProject(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.ProjectID,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	if project.CompanyID != input.CompanyID {
		return domain.TaskView{}, domain.ErrNotFound
	}

	if project.Status == domain.ProjectStatusArchived {
		return domain.TaskView{}, domain.ErrInvalidTransition
	}

	priority, err := domainTaskPriority(input.Priority)
	if err != nil {
		return domain.TaskView{}, err
	}

	if input.ParentTaskID != nil {
		if err := service.validateTaskParent(
			ctx,
			input.ActorUserID,
			input.TenantID,
			input.CompanyID,
			input.ProjectID,
			*input.ParentTaskID,
		); err != nil {
			return domain.TaskView{}, err
		}
	}

	title, err := boundedText(
		"title",
		input.Title,
		1,
		300,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	description, err := boundedText(
		"description",
		input.Description,
		0,
		32000,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	taskID, err := uuid.NewV7()
	if err != nil {
		return domain.TaskView{}, fmt.Errorf(
			"generate task ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	task := domain.Task{
		TenantID:        input.TenantID,
		CompanyID:       input.CompanyID,
		ProjectID:       input.ProjectID,
		ID:              taskID.String(),
		ParentTaskID:    input.ParentTaskID,
		Title:           title,
		Description:     description,
		Status:          domain.TaskStatusBacklog,
		Priority:        priority,
		Version:         1,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		task.ID,
		"task.created",
		task.TenantID,
		"task",
		task.ID,
		task.Version,
		&workv1.TaskCreated{
			Task: protoutil.Task(taskViewFromDomain(task)),
		},
		now,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	return service.repository.CreateTask(
		ctx,
		ports.CreateTaskParams{
			ActorUserID: input.ActorUserID,
			Task:        task,
			Event:       event,
		},
	)
}

// GetTask returns one task with derived dependency state.
func (service *Service) GetTask(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) (domain.TaskView, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"task_id":       taskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.TaskView{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_WORK_READ,
	); err != nil {
		return domain.TaskView{}, err
	}

	return service.repository.GetTask(
		ctx,
		actorUserID,
		tenantID,
		taskID,
	)
}

// ListTasks returns a paginated task page for a project.
func (service *Service) ListTasks(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	projectID string,
	pageSize int32,
	pageToken string,
	includeCanceled bool,
) ([]domain.TaskView, string, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"company_id":    companyID,
		"project_id":    projectID,
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

	var cursor *ports.TaskCursor

	if cursorValue != "" {
		if err := validateUUID(
			"page_token",
			cursorValue,
		); err != nil {
			return nil, "", err
		}

		cursor = &ports.TaskCursor{
			TaskID: cursorValue,
		}
	}

	tasks, err := service.repository.ListTasks(
		ctx,
		actorUserID,
		tenantID,
		companyID,
		projectID,
		limit,
		cursor,
		includeCanceled,
	)
	if err != nil {
		return nil, "", err
	}

	nextToken := ""

	if len(tasks) == limit && len(tasks) > 0 {
		nextToken = encodePageToken(
			tasks[len(tasks)-1].ID,
		)
	}

	return tasks, nextToken, nil
}

// UpdateTask validates and commits a versioned task update.
func (service *Service) UpdateTask(
	ctx context.Context,
	input UpdateTaskInput,
) (domain.TaskView, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.TaskView{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_UPDATE,
	); err != nil {
		return domain.TaskView{}, err
	}

	task, err := service.repository.GetTask(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	if task.Status == domain.TaskStatusCompleted ||
		task.Status == domain.TaskStatusCanceled {
		return domain.TaskView{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	if input.Title != nil {
		title, err := boundedText(
			"title",
			*input.Title,
			1,
			300,
		)
		if err != nil {
			return domain.TaskView{}, err
		}

		task.Title = title
	}

	if input.Description != nil {
		description, err := boundedText(
			"description",
			*input.Description,
			0,
			32000,
		)
		if err != nil {
			return domain.TaskView{}, err
		}

		task.Description = description
	}

	if input.Priority != nil {
		task.Priority = *input.Priority
	}

	task.Version++
	task.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		task.ID,
		"task.updated",
		task.TenantID,
		"task",
		task.ID,
		task.Version,
		&workv1.TaskUpdated{
			Task:            protoutil.Task(task),
			UpdatedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	return service.repository.UpdateTask(
		ctx,
		ports.UpdateTaskParams{
			ActorUserID:     input.ActorUserID,
			Task:            task.Task,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)
}

// ChangeTaskStatus changes a task lifecycle state.
func (service *Service) ChangeTaskStatus(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	expectedVersion int64,
	status workv1.TaskStatus,
) (domain.TaskView, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"task_id":       taskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.TaskView{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_TASK_STATUS_UPDATE,
	); err != nil {
		return domain.TaskView{}, err
	}

	statusValue, err := domainTaskStatus(status)
	if err != nil {
		return domain.TaskView{}, err
	}

	task, err := service.repository.GetTask(
		ctx,
		actorUserID,
		tenantID,
		taskID,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	previousStatus := task.Status

	if !taskCanTransition(task.Status, statusValue) {
		return domain.TaskView{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	task.Status = statusValue
	task.Version++
	task.UpdatedAt = now

	switch statusValue {
	case domain.TaskStatusCompleted:
		task.CompletedAt = &now
		task.CanceledAt = nil

	case domain.TaskStatusCanceled:
		task.CanceledAt = &now
		task.CompletedAt = nil

	default:
		task.CompletedAt = nil
		task.CanceledAt = nil
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		task.ID,
		"task.status_changed",
		task.TenantID,
		"task",
		task.ID,
		task.Version,
		&workv1.TaskStatusChanged{
			Task:            protoutil.Task(task),
			PreviousStatus:  protoutil.TaskStatus(previousStatus),
			ChangedByUserId: actorUserID,
		},
		now,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	return service.repository.ChangeTaskStatus(
		ctx,
		ports.TaskChangeParams{
			ActorUserID:     actorUserID,
			Task:            task.Task,
			PreviousStatus:  previousStatus,
			ExpectedVersion: expectedVersion,
			Event:           event,
		},
	)
}

func (service *Service) validateTaskParent(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	projectID string,
	parentTaskID string,
) error {
	parent, err := service.repository.GetTask(
		ctx,
		actorUserID,
		tenantID,
		parentTaskID,
	)
	if err != nil {
		return err
	}

	if parent.CompanyID != companyID ||
		parent.ProjectID != projectID {
		return fmt.Errorf(
			"%w: parent task must belong to the same project",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

// taskCanTransition reports whether a task status transition is permitted.
func taskCanTransition(
	current domain.TaskStatus,
	next domain.TaskStatus,
) bool {
	if current == next {
		return true
	}

	allowed := map[domain.TaskStatus][]domain.TaskStatus{
		domain.TaskStatusBacklog: {
			domain.TaskStatusReady,
			domain.TaskStatusInProgress,
			domain.TaskStatusWaitingApproval,
			domain.TaskStatusCompleted,
			domain.TaskStatusCanceled,
		},
		domain.TaskStatusReady: {
			domain.TaskStatusBacklog,
			domain.TaskStatusInProgress,
			domain.TaskStatusWaitingApproval,
			domain.TaskStatusCompleted,
			domain.TaskStatusCanceled,
		},
		domain.TaskStatusInProgress: {
			domain.TaskStatusBacklog,
			domain.TaskStatusReady,
			domain.TaskStatusWaitingApproval,
			domain.TaskStatusCompleted,
			domain.TaskStatusCanceled,
		},
		domain.TaskStatusWaitingApproval: {
			domain.TaskStatusBacklog,
			domain.TaskStatusReady,
			domain.TaskStatusInProgress,
			domain.TaskStatusCompleted,
			domain.TaskStatusCanceled,
		},
		domain.TaskStatusCompleted: {
			domain.TaskStatusBacklog,
			domain.TaskStatusCanceled,
		},
		domain.TaskStatusCanceled: {
			domain.TaskStatusBacklog,
			domain.TaskStatusReady,
			domain.TaskStatusInProgress,
		},
	}

	return slices.Contains(allowed[current], next)
}

func taskViewFromDomain(task domain.Task) domain.TaskView {
	return domain.TaskView{Task: task}
}
