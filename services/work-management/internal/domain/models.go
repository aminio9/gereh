// Package domain contains Work Management Service business types.
package domain

import "time"

// GoalStatus is the lifecycle state of a goal.
type GoalStatus string

// GoalStatus values.
const (
	GoalStatusActive    GoalStatus = "active"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusCanceled  GoalStatus = "canceled"
	GoalStatusArchived  GoalStatus = "archived"
)

// ProjectStatus is the lifecycle state of a project.
type ProjectStatus string

// ProjectStatus values.
const (
	ProjectStatusPlanned   ProjectStatus = "planned"
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusOnHold    ProjectStatus = "on_hold"
	ProjectStatusCompleted ProjectStatus = "completed"
	ProjectStatusCanceled  ProjectStatus = "canceled"
	ProjectStatusArchived  ProjectStatus = "archived"
)

// TaskStatus is the lifecycle state of a task.
//
// The blocked state is derived from incomplete prerequisites and is never
// persisted as a status.
type TaskStatus string

// TaskStatus values.
const (
	TaskStatusBacklog         TaskStatus = "backlog"
	TaskStatusReady           TaskStatus = "ready"
	TaskStatusInProgress      TaskStatus = "in_progress"
	TaskStatusWaitingApproval TaskStatus = "waiting_approval"
	TaskStatusCompleted       TaskStatus = "completed"
	TaskStatusCanceled        TaskStatus = "canceled"
)

// TaskPriority is the priority of a task.
type TaskPriority string

// TaskPriority values.
const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityNormal TaskPriority = "normal"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityUrgent TaskPriority = "urgent"
)

// AssigneeType identifies the kind of task assignee.
type AssigneeType string

// AssigneeType values.
const (
	AssigneeTypeUser  AssigneeType = "user"
	AssigneeTypeAgent AssigneeType = "agent"
)

// AssignmentRole is the role of a task assignment.
type AssignmentRole string

// AssignmentRole values.
const (
	AssignmentRolePrimary      AssignmentRole = "primary"
	AssignmentRoleCollaborator AssignmentRole = "collaborator"
	AssignmentRoleReviewer     AssignmentRole = "reviewer"
)

// Goal is a tenant-owned goal within a company.
type Goal struct {
	TenantID        string
	CompanyID       string
	ID              string
	Title           string
	Description     string
	Status          GoalStatus
	Version         int64
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
	ArchivedAt      *time.Time
}

// Project is a goal-scoped project within a company.
type Project struct {
	TenantID        string
	CompanyID       string
	GoalID          string
	ID              string
	Title           string
	Description     string
	Status          ProjectStatus
	Version         int64
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
	ArchivedAt      *time.Time
}

// Task is a project-scoped unit of work.
type Task struct {
	TenantID     string
	CompanyID    string
	ProjectID    string
	ID           string
	ParentTaskID *string

	Title       string
	Description string
	Status      TaskStatus
	Priority    TaskPriority

	Blocked                   bool
	DependencyCount           int32
	IncompleteDependencyCount int32

	Version         int64
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
	CanceledAt      *time.Time
}

// TaskView is a task enriched with derived dependency state. The blocked
// state is never persisted; it is always recomputed from prerequisites.
type TaskView struct {
	Task
	Blocked                   bool
	DependencyCount           int32
	IncompleteDependencyCount int32
}

// TaskDependency is a directed prerequisite edge within one project.
type TaskDependency struct {
	TenantID        string
	ProjectID       string
	TaskID          string
	DependsOnTaskID string
	CreatedByUserID string
	CreatedAt       time.Time
}

// TaskAssignment is a user or agent assignment on a task.
type TaskAssignment struct {
	TenantID         string
	TaskID           string
	ID               string
	AssigneeType     AssigneeType
	UserID           *string
	AgentID          *string
	Role             AssignmentRole
	AssignedByUserID string
	AssignedAt       time.Time
}

// Comment is a user or agent comment on a task.
type Comment struct {
	TenantID      string
	TaskID        string
	ID            string
	AuthorType    AssigneeType
	AuthorUserID  *string
	AuthorAgentID *string
	Body          string
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

// Artifact is metadata for a file object attached to a task.
//
// The Work Service stores metadata only; object bytes live in object storage
// behind controlled signed-URL uploads.
type Artifact struct {
	TenantID        string
	CompanyID       string
	TaskID          string
	ID              string
	ObjectKey       string
	FileName        string
	ContentType     string
	SizeBytes       int64
	SHA256          string
	Metadata        map[string]any
	CreatedByUserID string
	CreatedAt       time.Time
	DeletedAt       *time.Time
}

// ChecklistItem is one item in a task checklist.
type ChecklistItem struct {
	TenantID  string
	TaskID    string
	ID        string
	Title     string
	Completed bool
	Position  int32
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TaskSchedule is the time window and timezone of a task.
type TaskSchedule struct {
	TenantID  string
	TaskID    string
	NotBefore *time.Time
	DueAt     *time.Time
	Timezone  string
	Version   int64
	UpdatedAt time.Time
}
