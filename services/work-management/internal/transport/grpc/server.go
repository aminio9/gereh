package grpc

import (
	"context"
	"errors"

	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/application"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/protoutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements WorkManagementService.
type Server struct {
	workv1.UnimplementedWorkManagementServiceServer

	service *application.Service
}

// New creates the Work Management Service gRPC transport.
func New(service *application.Service) *Server {
	return &Server{
		service: service,
	}
}

// CreateGoal creates a goal.
func (server *Server) CreateGoal(
	ctx context.Context,
	request *workv1.CreateGoalRequest,
) (*workv1.CreateGoalResponse, error) {
	result, err := server.service.CreateGoal(
		ctx,
		application.CreateGoalInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			CompanyID:   request.GetCompanyId(),
			Title:       request.GetTitle(),
			Description: request.GetDescription(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.CreateGoalResponse{
		Goal: protoutil.Goal(result),
	}, nil
}

// GetGoal returns a goal.
func (server *Server) GetGoal(
	ctx context.Context,
	request *workv1.GetGoalRequest,
) (*workv1.GetGoalResponse, error) {
	result, err := server.service.GetGoal(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetGoalId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.GetGoalResponse{
		Goal: protoutil.Goal(result),
	}, nil
}

// ListGoals lists a company's goals.
func (server *Server) ListGoals(
	ctx context.Context,
	request *workv1.ListGoalsRequest,
) (*workv1.ListGoalsResponse, error) {
	goals, nextToken, err :=
		server.service.ListGoals(
			ctx,
			request.GetActorUserId(),
			request.GetTenantId(),
			request.GetCompanyId(),
			request.GetPageSize(),
			request.GetPageToken(),
			request.GetIncludeArchived(),
		)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*workv1.Goal, 0, len(goals))

	for _, value := range goals {
		items = append(items, protoutil.Goal(value))
	}

	return &workv1.ListGoalsResponse{
		Goals:         items,
		NextPageToken: nextToken,
	}, nil
}

// UpdateGoal updates goal settings.
func (server *Server) UpdateGoal(
	ctx context.Context,
	request *workv1.UpdateGoalRequest,
) (*workv1.UpdateGoalResponse, error) {
	result, err := server.service.UpdateGoal(
		ctx,
		application.UpdateGoalInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			GoalID:          request.GetGoalId(),
			ExpectedVersion: request.GetExpectedVersion(),
			Title:           request.Title,
			Description:     request.Description,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.UpdateGoalResponse{
		Goal: protoutil.Goal(result),
	}, nil
}

// ChangeGoalStatus changes a goal lifecycle state.
func (server *Server) ChangeGoalStatus(
	ctx context.Context,
	request *workv1.ChangeGoalStatusRequest,
) (*workv1.ChangeGoalStatusResponse, error) {
	result, err := server.service.ChangeGoalStatus(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetGoalId(),
		request.GetExpectedVersion(),
		request.GetStatus(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.ChangeGoalStatusResponse{
		Goal: protoutil.Goal(result),
	}, nil
}

// CreateProject creates a project.
func (server *Server) CreateProject(
	ctx context.Context,
	request *workv1.CreateProjectRequest,
) (*workv1.CreateProjectResponse, error) {
	result, err := server.service.CreateProject(
		ctx,
		application.CreateProjectInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			CompanyID:   request.GetCompanyId(),
			GoalID:      request.GetGoalId(),
			Title:       request.GetTitle(),
			Description: request.GetDescription(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.CreateProjectResponse{
		Project: protoutil.Project(result),
	}, nil
}

// GetProject returns a project.
func (server *Server) GetProject(
	ctx context.Context,
	request *workv1.GetProjectRequest,
) (*workv1.GetProjectResponse, error) {
	result, err := server.service.GetProject(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetProjectId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.GetProjectResponse{
		Project: protoutil.Project(result),
	}, nil
}

// ListProjects lists a company's projects.
func (server *Server) ListProjects(
	ctx context.Context,
	request *workv1.ListProjectsRequest,
) (*workv1.ListProjectsResponse, error) {
	projects, nextToken, err :=
		server.service.ListProjects(
			ctx,
			request.GetActorUserId(),
			request.GetTenantId(),
			request.GetCompanyId(),
			request.GetGoalId(),
			request.GetPageSize(),
			request.GetPageToken(),
			request.GetIncludeArchived(),
		)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*workv1.Project, 0, len(projects))

	for _, value := range projects {
		items = append(items, protoutil.Project(value))
	}

	return &workv1.ListProjectsResponse{
		Projects:      items,
		NextPageToken: nextToken,
	}, nil
}

// UpdateProject updates project settings.
func (server *Server) UpdateProject(
	ctx context.Context,
	request *workv1.UpdateProjectRequest,
) (*workv1.UpdateProjectResponse, error) {
	result, err := server.service.UpdateProject(
		ctx,
		application.UpdateProjectInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			ProjectID:       request.GetProjectId(),
			ExpectedVersion: request.GetExpectedVersion(),
			Title:           request.Title,
			Description:     request.Description,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.UpdateProjectResponse{
		Project: protoutil.Project(result),
	}, nil
}

// ChangeProjectStatus changes a project lifecycle state.
func (server *Server) ChangeProjectStatus(
	ctx context.Context,
	request *workv1.ChangeProjectStatusRequest,
) (*workv1.ChangeProjectStatusResponse, error) {
	result, err := server.service.ChangeProjectStatus(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetProjectId(),
		request.GetExpectedVersion(),
		request.GetStatus(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.ChangeProjectStatusResponse{
		Project: protoutil.Project(result),
	}, nil
}

// CreateTask creates a task.
func (server *Server) CreateTask(
	ctx context.Context,
	request *workv1.CreateTaskRequest,
) (*workv1.CreateTaskResponse, error) {
	result, err := server.service.CreateTask(
		ctx,
		application.CreateTaskInput{
			ActorUserID:  request.GetActorUserId(),
			TenantID:     request.GetTenantId(),
			CompanyID:    request.GetCompanyId(),
			ProjectID:    request.GetProjectId(),
			ParentTaskID: request.ParentTaskId,
			Title:        request.GetTitle(),
			Description:  request.GetDescription(),
			Priority:     request.GetPriority(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.CreateTaskResponse{
		Task: protoutil.Task(result),
	}, nil
}

// GetTask returns a task.
func (server *Server) GetTask(
	ctx context.Context,
	request *workv1.GetTaskRequest,
) (*workv1.GetTaskResponse, error) {
	result, err := server.service.GetTask(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetTaskId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.GetTaskResponse{
		Task: protoutil.Task(result),
	}, nil
}

// ListTasks lists a project's tasks.
func (server *Server) ListTasks(
	ctx context.Context,
	request *workv1.ListTasksRequest,
) (*workv1.ListTasksResponse, error) {
	tasks, nextToken, err :=
		server.service.ListTasks(
			ctx,
			request.GetActorUserId(),
			request.GetTenantId(),
			request.GetCompanyId(),
			request.GetProjectId(),
			request.GetPageSize(),
			request.GetPageToken(),
			request.GetIncludeCanceled(),
		)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*workv1.Task, 0, len(tasks))

	for _, value := range tasks {
		items = append(items, protoutil.Task(value))
	}

	return &workv1.ListTasksResponse{
		Tasks:         items,
		NextPageToken: nextToken,
	}, nil
}

// UpdateTask updates task settings.
func (server *Server) UpdateTask(
	ctx context.Context,
	request *workv1.UpdateTaskRequest,
) (*workv1.UpdateTaskResponse, error) {
	var priority *domain.TaskPriority

	if request.Priority != nil {
		value, err := domainTaskPriority(request.GetPriority())
		if err != nil {
			return nil, mapError(err)
		}

		priority = &value
	}

	result, err := server.service.UpdateTask(
		ctx,
		application.UpdateTaskInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			TaskID:          request.GetTaskId(),
			ExpectedVersion: request.GetExpectedVersion(),
			Title:           request.Title,
			Description:     request.Description,
			Priority:        priority,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.UpdateTaskResponse{
		Task: protoutil.Task(result),
	}, nil
}

// ChangeTaskStatus changes a task lifecycle state.
func (server *Server) ChangeTaskStatus(
	ctx context.Context,
	request *workv1.ChangeTaskStatusRequest,
) (*workv1.ChangeTaskStatusResponse, error) {
	result, err := server.service.ChangeTaskStatus(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetTaskId(),
		request.GetExpectedVersion(),
		request.GetStatus(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.ChangeTaskStatusResponse{
		Task: protoutil.Task(result),
	}, nil
}

// AddTaskDependency links a task to a prerequisite.
func (server *Server) AddTaskDependency(
	ctx context.Context,
	request *workv1.AddTaskDependencyRequest,
) (*workv1.AddTaskDependencyResponse, error) {
	result, err := server.service.AddTaskDependency(
		ctx,
		application.AddDependencyInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			TaskID:          request.GetTaskId(),
			DependsOnTaskID: request.GetDependsOnTaskId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.AddTaskDependencyResponse{
		Dependency: protoutil.TaskDependency(result),
	}, nil
}

// RemoveTaskDependency removes a prerequisite link.
func (server *Server) RemoveTaskDependency(
	ctx context.Context,
	request *workv1.RemoveTaskDependencyRequest,
) (*workv1.RemoveTaskDependencyResponse, error) {
	err := server.service.RemoveTaskDependency(
		ctx,
		application.RemoveDependencyInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			TaskID:          request.GetTaskId(),
			DependsOnTaskID: request.GetDependsOnTaskId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.RemoveTaskDependencyResponse{}, nil
}

// AssignTask assigns a user or agent to a task.
func (server *Server) AssignTask(
	ctx context.Context,
	request *workv1.AssignTaskRequest,
) (*workv1.AssignTaskResponse, error) {
	result, err := server.service.AssignTask(
		ctx,
		application.AssignTaskInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			TaskID:      request.GetTaskId(),
			Assignee:    request.GetAssigneeType(),
			UserID:      request.UserId,
			AgentID:     request.AgentId,
			Role:        request.GetRole(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.AssignTaskResponse{
		Assignment: protoutil.TaskAssignment(result),
	}, nil
}

// UnassignTask removes a task assignment.
func (server *Server) UnassignTask(
	ctx context.Context,
	request *workv1.UnassignTaskRequest,
) (*workv1.UnassignTaskResponse, error) {
	err := server.service.UnassignTask(
		ctx,
		application.UnassignTaskInput{
			ActorUserID:  request.GetActorUserId(),
			TenantID:     request.GetTenantId(),
			TaskID:       request.GetTaskId(),
			AssignmentID: request.GetAssignmentId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.UnassignTaskResponse{}, nil
}

// AddComment posts a comment on a task.
func (server *Server) AddComment(
	ctx context.Context,
	request *workv1.AddCommentRequest,
) (*workv1.AddCommentResponse, error) {
	result, err := server.service.AddComment(
		ctx,
		application.AddCommentInput{
			ActorUserID:   request.GetActorUserId(),
			TenantID:      request.GetTenantId(),
			TaskID:        request.GetTaskId(),
			AuthorType:    request.GetAuthorType(),
			AuthorUserID:  request.AuthorUserId,
			AuthorAgentID: request.AuthorAgentId,
			Body:          request.GetBody(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.AddCommentResponse{
		Comment: protoutil.Comment(result),
	}, nil
}

// UpdateComment edits a comment.
func (server *Server) UpdateComment(
	ctx context.Context,
	request *workv1.UpdateCommentRequest,
) (*workv1.UpdateCommentResponse, error) {
	result, err := server.service.UpdateComment(
		ctx,
		application.UpdateCommentInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			TaskID:          request.GetTaskId(),
			CommentID:       request.GetCommentId(),
			ExpectedVersion: request.GetExpectedVersion(),
			Body:            request.GetBody(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.UpdateCommentResponse{
		Comment: protoutil.Comment(result),
	}, nil
}

// DeleteComment soft-deletes a comment.
func (server *Server) DeleteComment(
	ctx context.Context,
	request *workv1.DeleteCommentRequest,
) (*workv1.DeleteCommentResponse, error) {
	err := server.service.DeleteComment(
		ctx,
		application.DeleteCommentInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			TaskID:          request.GetTaskId(),
			CommentID:       request.GetCommentId(),
			ExpectedVersion: request.GetExpectedVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.DeleteCommentResponse{}, nil
}

// AddArtifact records artifact metadata for a task.
func (server *Server) AddArtifact(
	ctx context.Context,
	request *workv1.AddArtifactRequest,
) (*workv1.AddArtifactResponse, error) {
	var metadata map[string]any

	if value := request.GetMetadata(); value != nil {
		metadata = value.AsMap()
	}

	result, err := server.service.AddArtifact(
		ctx,
		application.AddArtifactInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			CompanyID:   request.GetCompanyId(),
			TaskID:      request.GetTaskId(),
			ObjectKey:   request.GetObjectKey(),
			FileName:    request.GetFileName(),
			ContentType: request.GetContentType(),
			SizeBytes:   request.GetSizeBytes(),
			SHA256:      request.GetSha256(),
			Metadata:    metadata,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.AddArtifactResponse{
		Artifact: protoutil.Artifact(result),
	}, nil
}

// DeleteArtifact soft-deletes artifact metadata.
func (server *Server) DeleteArtifact(
	ctx context.Context,
	request *workv1.DeleteArtifactRequest,
) (*workv1.DeleteArtifactResponse, error) {
	err := server.service.DeleteArtifact(
		ctx,
		application.DeleteArtifactInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			TaskID:      request.GetTaskId(),
			ArtifactID:  request.GetArtifactId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.DeleteArtifactResponse{}, nil
}

// AddChecklistItem appends a checklist item to a task.
func (server *Server) AddChecklistItem(
	ctx context.Context,
	request *workv1.AddChecklistItemRequest,
) (*workv1.AddChecklistItemResponse, error) {
	result, err := server.service.AddChecklistItem(
		ctx,
		application.AddChecklistItemInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			TaskID:      request.GetTaskId(),
			Title:       request.GetTitle(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.AddChecklistItemResponse{
		Item: protoutil.ChecklistItem(result),
	}, nil
}

// UpdateChecklistItem edits a checklist item.
func (server *Server) UpdateChecklistItem(
	ctx context.Context,
	request *workv1.UpdateChecklistItemRequest,
) (*workv1.UpdateChecklistItemResponse, error) {
	result, err := server.service.UpdateChecklistItem(
		ctx,
		application.UpdateChecklistItemInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			TaskID:          request.GetTaskId(),
			ItemID:          request.GetItemId(),
			ExpectedVersion: request.GetExpectedVersion(),
			Title:           request.Title,
			Completed:       request.Completed,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.UpdateChecklistItemResponse{
		Item: protoutil.ChecklistItem(result),
	}, nil
}

// DeleteChecklistItem removes a checklist item.
func (server *Server) DeleteChecklistItem(
	ctx context.Context,
	request *workv1.DeleteChecklistItemRequest,
) (*workv1.DeleteChecklistItemResponse, error) {
	err := server.service.DeleteChecklistItem(
		ctx,
		application.DeleteChecklistItemInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			TaskID:          request.GetTaskId(),
			ItemID:          request.GetItemId(),
			ExpectedVersion: request.GetExpectedVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.DeleteChecklistItemResponse{}, nil
}

// UpsertTaskSchedule sets a task's time window.
func (server *Server) UpsertTaskSchedule(
	ctx context.Context,
	request *workv1.UpsertTaskScheduleRequest,
) (*workv1.UpsertTaskScheduleResponse, error) {
	result, err := server.service.UpsertTaskSchedule(
		ctx,
		application.UpsertScheduleInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			TaskID:      request.GetTaskId(),
			NotBefore:   protoTimestampValue(request.GetNotBefore()),
			DueAt:       protoTimestampValue(request.GetDueAt()),
			Timezone:    request.GetTimezone(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.UpsertTaskScheduleResponse{
		Schedule: protoutil.TaskSchedule(result),
	}, nil
}

// DeleteTaskSchedule removes a task schedule.
func (server *Server) DeleteTaskSchedule(
	ctx context.Context,
	request *workv1.DeleteTaskScheduleRequest,
) (*workv1.DeleteTaskScheduleResponse, error) {
	err := server.service.DeleteTaskSchedule(
		ctx,
		application.DeleteScheduleInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			TaskID:      request.GetTaskId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &workv1.DeleteTaskScheduleResponse{}, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(
			codes.InvalidArgument,
			"invalid work request",
		)

	case errors.Is(err, domain.ErrNotFound):
		return status.Error(
			codes.NotFound,
			"work resource not found",
		)

	case errors.Is(err, domain.ErrForbidden):
		return status.Error(
			codes.PermissionDenied,
			"work operation forbidden",
		)

	case errors.Is(err, domain.ErrTenantNotActive):
		return status.Error(
			codes.PermissionDenied,
			"tenant is not active",
		)

	case errors.Is(err, domain.ErrVersionConflict):
		return status.Error(
			codes.Aborted,
			"work resource changed; reload and retry",
		)

	case errors.Is(err, domain.ErrConflict):
		return status.Error(
			codes.FailedPrecondition,
			"work resource conflict",
		)

	case errors.Is(err, domain.ErrDependencyCycle):
		return status.Error(
			codes.FailedPrecondition,
			"task dependency would create a cycle",
		)

	case errors.Is(err, domain.ErrTaskBlocked):
		return status.Error(
			codes.FailedPrecondition,
			"task has incomplete dependencies",
		)

	case errors.Is(err, domain.ErrChecklistOpen):
		return status.Error(
			codes.FailedPrecondition,
			"task has incomplete checklist items",
		)

	case errors.Is(err, domain.ErrProjectOpenTasks):
		return status.Error(
			codes.FailedPrecondition,
			"project still has incomplete tasks",
		)

	case errors.Is(err, domain.ErrGoalOpenProjects):
		return status.Error(
			codes.FailedPrecondition,
			"goal still has incomplete projects",
		)

	case errors.Is(err, domain.ErrCompanyNotActive):
		return status.Error(
			codes.FailedPrecondition,
			"company is not active",
		)

	case errors.Is(err, domain.ErrInvalidTransition):
		return status.Error(
			codes.FailedPrecondition,
			"invalid work status transition",
		)

	case errors.Is(err, domain.ErrCommentOwnership):
		return status.Error(
			codes.FailedPrecondition,
			"comment is owned by another principal",
		)

	default:
		return status.Error(
			codes.Internal,
			"work operation failed",
		)
	}
}
