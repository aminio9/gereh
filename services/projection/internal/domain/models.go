// Package domain contains the projection read-model types.
package domain

import "time"

// EventMeta is the transport and identity metadata of one consumed event.
type EventMeta struct {
	EventID  string
	TenantID string

	Topic     string
	Partition int32
	Offset    int64

	EventType    string
	EventVersion uint32

	AggregateType    string
	AggregateID      string
	AggregateVersion uint64

	EventHash []byte

	OccurredAt  time.Time
	ProcessedAt time.Time
}

// Tenant is the projected tenant snapshot.
type Tenant struct {
	ID            string
	Slug          string
	DisplayName   string
	Status        string
	Region        string
	RetentionDays int32

	SourceVersion uint64
	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Company is the projected company snapshot.
type Company struct {
	TenantID    string
	ID          string
	Slug        string
	DisplayName string
	Description string
	Status      string
	IsDefault   bool

	SourceVersion uint64
	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Agent is the projected agent snapshot.
type Agent struct {
	TenantID  string
	CompanyID string
	ID        string

	Slug        string
	DisplayName string
	RoleTitle   string
	Objective   string

	ManagerAgentID *string

	Status           string
	ExecutionProfile string
	AutonomyLevel    string

	SourceVersion uint64
	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Goal is the projected goal snapshot.
type Goal struct {
	TenantID  string
	CompanyID string
	ID        string

	Title       string
	Description string
	Status      string

	SourceVersion uint64
	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Project is the projected project snapshot.
type Project struct {
	TenantID  string
	CompanyID string
	GoalID    string
	ID        string

	Title       string
	Description string
	Status      string

	SourceVersion uint64
	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Task is the projected task snapshot.
type Task struct {
	TenantID  string
	CompanyID string
	ProjectID string
	ID        string

	ParentTaskID *string

	Title       string
	Description string
	Status      string
	Priority    string

	CreatedByUserID string

	SourceVersion uint64
	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Dependency is the projected task prerequisite edge.
type Dependency struct {
	TenantID        string
	ProjectID       string
	TaskID          string
	DependsOnTaskID string

	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Assignment is the projected task assignment.
type Assignment struct {
	TenantID string
	TaskID   string
	ID       string

	AssigneeType string

	UserID  *string
	AgentID *string

	Role string

	AssignedAt time.Time

	SourceEventID string
	SourceEventAt time.Time
	ProjectedAt   time.Time
}

// Activity is one safe task activity feed item.
type Activity struct {
	TenantID string
	EventID  string

	EventType string

	CompanyID *string
	ProjectID *string
	TaskID    *string

	ActorType *string
	ActorID   *string

	Summary string

	OccurredAt  time.Time
	ProjectedAt time.Time
}

// SearchDocument is the projected tenant search document.
type SearchDocument struct {
	TenantID string

	Type string
	ID   string

	CompanyID *string

	Title    string
	Subtitle string
	Body     string
	Status   string

	Deleted bool

	SourceVersion uint64
	SourceEventID string

	UpdatedAt   time.Time
	ProjectedAt time.Time
}

// ProjectionMetadata reports how current the read model is.
type ProjectionMetadata struct {
	ProjectedThroughEventTime time.Time
	LastProcessedAt           time.Time
}

// DashboardSummary is the tenant dashboard read model.
type DashboardSummary struct {
	CompaniesTotal  int64
	CompaniesActive int64

	AgentsTotal    int64
	AgentsReady    int64
	AgentsDegraded int64
	AgentsPaused   int64
	AgentsFailed   int64

	GoalsActive    int64
	GoalsCompleted int64

	ProjectsActive    int64
	ProjectsOnHold    int64
	ProjectsCompleted int64

	TasksTotal           int64
	TasksBacklog         int64
	TasksReady           int64
	TasksInProgress      int64
	TasksWaitingApproval int64
	TasksCompleted       int64
	TasksCanceled        int64
	TasksBlocked         int64
}

// CompanyOverview is the company read model.
type CompanyOverview struct {
	TenantID  string
	CompanyID string

	Slug        string
	DisplayName string
	Status      string
	IsDefault   bool

	AgentsTotal    int64
	AgentsReady    int64
	AgentsDegraded int64
	AgentsPaused   int64

	TasksTotal           int64
	TasksReady           int64
	TasksInProgress      int64
	TasksWaitingApproval int64
	TasksCompleted       int64
	TasksBlocked         int64
}

// AgentOverview is the agent read model.
type AgentOverview struct {
	TenantID  string
	CompanyID string
	AgentID   string

	Slug        string
	DisplayName string
	RoleTitle   string
	Status      string

	ManagerAgentID *string

	AssignedTaskCount int64
	ActiveTaskCount   int64

	UpdatedAt time.Time
}

// SearchResult is one tenant search hit.
type SearchResult struct {
	Type      string
	ID        string
	CompanyID *string

	Title    string
	Subtitle string
	Status   string

	Rank float64

	UpdatedAt time.Time
}
