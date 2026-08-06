// Package ports defines the Work Management Service boundaries.
package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
)

// GoalCursor is the pagination position for goals.
type GoalCursor struct {
	GoalID string
}

// ProjectCursor is the pagination position for projects.
type ProjectCursor struct {
	ProjectID string
}

// TaskCursor is the pagination position for tasks.
type TaskCursor struct {
	TaskID string
}

// CreateGoalParams carries a goal creation and its committed event.
type CreateGoalParams struct {
	ActorUserID string
	Goal        domain.Goal
	Event       domain.OutboxEvent
}

// UpdateGoalParams carries an optimistic-concurrency goal update.
type UpdateGoalParams struct {
	ActorUserID     string
	Goal            domain.Goal
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// CreateProjectParams carries a project creation and its committed event.
type CreateProjectParams struct {
	ActorUserID string
	Project     domain.Project
	Event       domain.OutboxEvent
}

// UpdateProjectParams carries an optimistic-concurrency project update.
type UpdateProjectParams struct {
	ActorUserID     string
	Project         domain.Project
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// CreateTaskParams carries a task creation and its committed event.
type CreateTaskParams struct {
	ActorUserID string
	Task        domain.Task
	Event       domain.OutboxEvent
}

// UpdateTaskParams carries an optimistic-concurrency task update.
type UpdateTaskParams struct {
	ActorUserID     string
	Task            domain.Task
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// TaskChangeParams carries a task status change and its committed event.
type TaskChangeParams struct {
	ActorUserID     string
	Task            domain.Task
	PreviousStatus  domain.TaskStatus
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// AddDependencyParams carries a task dependency creation.
type AddDependencyParams struct {
	ActorUserID string
	Dependency  domain.TaskDependency
	Event       domain.OutboxEvent
}

// RemoveDependencyParams carries a task dependency removal.
type RemoveDependencyParams struct {
	ActorUserID string
	Dependency  domain.TaskDependency
	Event       domain.OutboxEvent
}

// AssignTaskParams carries a task assignment creation.
type AssignTaskParams struct {
	ActorUserID string
	Assignment  domain.TaskAssignment
	Event       domain.OutboxEvent
}

// UnassignTaskParams carries a task assignment removal.
type UnassignTaskParams struct {
	ActorUserID string
	Assignment  domain.TaskAssignment
	Event       domain.OutboxEvent
}

// AddCommentParams carries a comment creation.
type AddCommentParams struct {
	ActorUserID string
	Comment     domain.Comment
	Event       domain.OutboxEvent
}

// UpdateCommentParams carries an optimistic-concurrency comment update.
type UpdateCommentParams struct {
	ActorUserID     string
	Comment         domain.Comment
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// DeleteCommentParams carries a comment soft-delete.
type DeleteCommentParams struct {
	ActorUserID     string
	Comment         domain.Comment
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// AddArtifactParams carries an artifact metadata creation.
type AddArtifactParams struct {
	ActorUserID string
	Artifact    domain.Artifact
	Event       domain.OutboxEvent
}

// DeleteArtifactParams carries an artifact metadata soft-delete.
type DeleteArtifactParams struct {
	ActorUserID string
	Artifact    domain.Artifact
	Event       domain.OutboxEvent
}

// AddChecklistItemParams carries a checklist item creation.
type AddChecklistItemParams struct {
	ActorUserID string
	Item        domain.ChecklistItem
	Event       domain.OutboxEvent
}

// UpdateChecklistItemParams carries an optimistic-concurrency checklist item
// update.
type UpdateChecklistItemParams struct {
	ActorUserID     string
	Item            domain.ChecklistItem
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// DeleteChecklistItemParams carries a checklist item deletion.
type DeleteChecklistItemParams struct {
	ActorUserID string
	Item        domain.ChecklistItem
	Event       domain.OutboxEvent
}

// UpsertScheduleParams carries a task schedule upsert.
type UpsertScheduleParams struct {
	ActorUserID string
	Schedule    domain.TaskSchedule
	Event       domain.OutboxEvent
}

// DeleteScheduleParams carries a task schedule deletion.
type DeleteScheduleParams struct {
	ActorUserID string
	Schedule    domain.TaskSchedule
	Event       domain.OutboxEvent
}

// Repository is the Work Management Service persistence boundary.
type Repository interface {
	CreateGoal(context.Context, CreateGoalParams) (domain.Goal, error)
	GetGoal(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		goalID string,
	) (domain.Goal, error)
	ListGoals(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
		limit int,
		cursor *GoalCursor,
		includeArchived bool,
	) ([]domain.Goal, error)
	UpdateGoal(context.Context, UpdateGoalParams) (domain.Goal, error)
	ChangeGoalStatus(context.Context, UpdateGoalParams) (domain.Goal, error)

	CreateProject(context.Context, CreateProjectParams) (domain.Project, error)
	GetProject(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		projectID string,
	) (domain.Project, error)
	ListProjects(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
		goalID string,
		limit int,
		cursor *ProjectCursor,
		includeArchived bool,
	) ([]domain.Project, error)
	UpdateProject(context.Context, UpdateProjectParams) (domain.Project, error)
	ChangeProjectStatus(context.Context, UpdateProjectParams) (domain.Project, error)

	CreateTask(context.Context, CreateTaskParams) (domain.TaskView, error)
	GetTask(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
	) (domain.TaskView, error)
	ListTasks(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
		projectID string,
		limit int,
		cursor *TaskCursor,
		includeCanceled bool,
	) ([]domain.TaskView, error)
	UpdateTask(context.Context, UpdateTaskParams) (domain.TaskView, error)
	ChangeTaskStatus(context.Context, TaskChangeParams) (domain.TaskView, error)

	AddDependency(context.Context, AddDependencyParams) (domain.TaskDependency, error)
	RemoveDependency(context.Context, RemoveDependencyParams) error

	AssignTask(context.Context, AssignTaskParams) (domain.TaskAssignment, error)
	GetAssignment(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
		assignmentID string,
	) (domain.TaskAssignment, error)
	UnassignTask(context.Context, UnassignTaskParams) error

	AddComment(context.Context, AddCommentParams) (domain.Comment, error)
	GetComment(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
		commentID string,
	) (domain.Comment, error)
	UpdateComment(context.Context, UpdateCommentParams) (domain.Comment, error)
	DeleteComment(context.Context, DeleteCommentParams) (domain.Comment, error)
	ListComments(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
	) ([]domain.Comment, error)

	AddArtifact(context.Context, AddArtifactParams) (domain.Artifact, error)
	GetArtifact(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
		artifactID string,
	) (domain.Artifact, error)
	DeleteArtifact(context.Context, DeleteArtifactParams) (domain.Artifact, error)
	ListArtifacts(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
	) ([]domain.Artifact, error)

	AddChecklistItem(context.Context, AddChecklistItemParams) (domain.ChecklistItem, error)
	GetChecklistItem(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
		itemID string,
	) (domain.ChecklistItem, error)
	UpdateChecklistItem(context.Context, UpdateChecklistItemParams) (domain.ChecklistItem, error)
	DeleteChecklistItem(context.Context, DeleteChecklistItemParams) error
	ListChecklist(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
	) ([]domain.ChecklistItem, error)

	UpsertSchedule(context.Context, UpsertScheduleParams) (domain.TaskSchedule, error)
	DeleteSchedule(context.Context, DeleteScheduleParams) error
	GetSchedule(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		taskID string,
	) (domain.TaskSchedule, error)

	ClaimOutbox(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.OutboxRecord, error)

	MarkOutboxPublished(
		ctx context.Context,
		outboxID int64,
	) error

	ReleaseOutbox(
		ctx context.Context,
		outboxID int64,
		retryAt time.Time,
		publishError string,
	) error
}
