package projection

import (
	"time"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func tenantSnapshot(
	value *tenantv1.Tenant,
	meta domain.EventMeta,
	now time.Time,
) domain.Tenant {
	return domain.Tenant{
		ID:            value.GetTenantId(),
		Slug:          value.GetSlug(),
		DisplayName:   value.GetDisplayName(),
		Status:        tenantStatus(value.GetStatus()),
		Region:        value.GetRegion(),
		RetentionDays: value.GetRetentionDays(),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func companySnapshot(
	value *organizationv1.Company,
	meta domain.EventMeta,
	now time.Time,
) domain.Company {
	return domain.Company{
		TenantID:    value.GetTenantId(),
		ID:          value.GetCompanyId(),
		Slug:        value.GetSlug(),
		DisplayName: value.GetDisplayName(),
		Description: value.GetDescription(),
		Status:      companyStatus(value.GetStatus()),
		IsDefault:   value.GetIsDefault(),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func agentSnapshot(
	value *organizationv1.Agent,
	meta domain.EventMeta,
	now time.Time,
) domain.Agent {
	projection := domain.Agent{
		TenantID:  value.GetTenantId(),
		CompanyID: value.GetCompanyId(),
		ID:        value.GetAgentId(),

		Slug:        value.GetSlug(),
		DisplayName: value.GetDisplayName(),
		RoleTitle:   value.GetRoleTitle(),
		Objective:   value.GetObjective(),

		Status:           agentStatus(value.GetStatus()),
		ExecutionProfile: executionProfile(value.GetExecutionProfile()),
		AutonomyLevel:    autonomyLevel(value.GetAutonomyLevel()),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}

	if value.ManagerAgentId != nil {
		managerID := value.GetManagerAgentId()

		projection.ManagerAgentID = &managerID
	}

	return projection
}

func goalSnapshot(
	value *workv1.Goal,
	meta domain.EventMeta,
	now time.Time,
) domain.Goal {
	return domain.Goal{
		TenantID:  value.GetTenantId(),
		CompanyID: value.GetCompanyId(),
		ID:        value.GetGoalId(),

		Title:       value.GetTitle(),
		Description: value.GetDescription(),
		Status:      goalStatus(value.GetStatus()),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func projectSnapshot(
	value *workv1.Project,
	meta domain.EventMeta,
	now time.Time,
) domain.Project {
	return domain.Project{
		TenantID:  value.GetTenantId(),
		CompanyID: value.GetCompanyId(),
		GoalID:    value.GetGoalId(),
		ID:        value.GetProjectId(),

		Title:       value.GetTitle(),
		Description: value.GetDescription(),
		Status:      projectStatus(value.GetStatus()),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func taskSnapshot(
	value *workv1.Task,
	meta domain.EventMeta,
	now time.Time,
) domain.Task {
	projection := domain.Task{
		TenantID:  value.GetTenantId(),
		CompanyID: value.GetCompanyId(),
		ProjectID: value.GetProjectId(),
		ID:        value.GetTaskId(),

		Title:       value.GetTitle(),
		Description: value.GetDescription(),
		Status:      taskStatus(value.GetStatus()),
		Priority:    taskPriority(value.GetPriority()),

		CreatedByUserID: value.GetCreatedByUserId(),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}

	if value.ParentTaskId != nil {
		parentID := value.GetParentTaskId()

		projection.ParentTaskID = &parentID
	}

	return projection
}

func dependencySnapshot(
	value *workv1.TaskDependency,
	meta domain.EventMeta,
	now time.Time,
) domain.Dependency {
	return domain.Dependency{
		TenantID:        value.GetTenantId(),
		ProjectID:       value.GetProjectId(),
		TaskID:          value.GetTaskId(),
		DependsOnTaskID: value.GetDependsOnTaskId(),

		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func assignmentSnapshot(
	value *workv1.TaskAssignment,
	meta domain.EventMeta,
	now time.Time,
) domain.Assignment {
	projection := domain.Assignment{
		TenantID: value.GetTenantId(),
		TaskID:   value.GetTaskId(),
		ID:       value.GetAssignmentId(),

		AssigneeType: assigneeType(value.GetAssigneeType()),
		Role:         assignmentRole(value.GetRole()),

		AssignedAt: protoTime(value.GetAssignedAt()),

		SourceEventID: meta.EventID,
		SourceEventAt: meta.OccurredAt,
		ProjectedAt:   now,
	}

	if value.UserId != nil {
		userID := value.GetUserId()

		projection.UserID = &userID
	}

	if value.AgentId != nil {
		agentID := value.GetAgentId()

		projection.AgentID = &agentID
	}

	return projection
}

func companySearchDocument(
	value *organizationv1.Company,
	meta domain.EventMeta,
	now time.Time,
) domain.SearchDocument {
	return domain.SearchDocument{
		TenantID: value.GetTenantId(),

		Type: "company",
		ID:   value.GetCompanyId(),

		CompanyID: stringPointer(value.GetCompanyId()),

		Title:    value.GetDisplayName(),
		Subtitle: value.GetSlug(),
		Body:     value.GetDescription(),
		Status:   companyStatus(value.GetStatus()),

		Deleted: value.GetStatus() ==
			organizationv1.CompanyStatus_COMPANY_STATUS_ARCHIVED,

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		UpdatedAt:     meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func agentSearchDocument(
	value *organizationv1.Agent,
	meta domain.EventMeta,
	now time.Time,
) domain.SearchDocument {
	return domain.SearchDocument{
		TenantID: value.GetTenantId(),

		Type: "agent",
		ID:   value.GetAgentId(),

		CompanyID: stringPointer(value.GetCompanyId()),

		Title:    value.GetDisplayName(),
		Subtitle: value.GetRoleTitle(),
		Body:     value.GetObjective(),
		Status:   agentStatus(value.GetStatus()),

		Deleted: value.GetStatus() ==
			organizationv1.AgentStatus_AGENT_STATUS_DELETED,

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		UpdatedAt:     meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func goalSearchDocument(
	value *workv1.Goal,
	meta domain.EventMeta,
	now time.Time,
) domain.SearchDocument {
	return domain.SearchDocument{
		TenantID: value.GetTenantId(),

		Type: "goal",
		ID:   value.GetGoalId(),

		CompanyID: stringPointer(value.GetCompanyId()),

		Title:    value.GetTitle(),
		Subtitle: value.GetDescription(),
		Status:   goalStatus(value.GetStatus()),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		UpdatedAt:     meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func projectSearchDocument(
	value *workv1.Project,
	meta domain.EventMeta,
	now time.Time,
) domain.SearchDocument {
	return domain.SearchDocument{
		TenantID: value.GetTenantId(),

		Type: "project",
		ID:   value.GetProjectId(),

		CompanyID: stringPointer(value.GetCompanyId()),

		Title:    value.GetTitle(),
		Subtitle: value.GetDescription(),
		Status:   projectStatus(value.GetStatus()),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		UpdatedAt:     meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func taskSearchDocument(
	value *workv1.Task,
	meta domain.EventMeta,
	now time.Time,
) domain.SearchDocument {
	return domain.SearchDocument{
		TenantID: value.GetTenantId(),

		Type: "task",
		ID:   value.GetTaskId(),

		CompanyID: stringPointer(value.GetCompanyId()),

		Title:    value.GetTitle(),
		Subtitle: value.GetDescription(),
		Status:   taskStatus(value.GetStatus()),

		SourceVersion: meta.AggregateVersion,
		SourceEventID: meta.EventID,
		UpdatedAt:     meta.OccurredAt,
		ProjectedAt:   now,
	}
}

func taskEventSummary(
	eventType string,
	value *workv1.Task,
) string {
	switch eventType {
	case "task.created":
		return "task created"

	case "task.updated":
		return "task updated"

	case "task.status_changed":
		return "task status changed to " +
			taskStatus(value.GetStatus())

	default:
		return "task changed"
	}
}

func tenantStatus(value tenantv1.TenantStatus) string {
	switch value {
	case tenantv1.TenantStatus_TENANT_STATUS_ACTIVE:
		return "active"

	case tenantv1.TenantStatus_TENANT_STATUS_ARCHIVED:
		return "archived"

	case tenantv1.TenantStatus_TENANT_STATUS_PROVISIONING:
		return "provisioning"

	case tenantv1.TenantStatus_TENANT_STATUS_PROVISIONING_FAILED:
		return "provisioning_failed"

	default:
		return "unknown"
	}
}

func companyStatus(value organizationv1.CompanyStatus) string {
	switch value {
	case organizationv1.CompanyStatus_COMPANY_STATUS_ACTIVE:
		return "active"

	case organizationv1.CompanyStatus_COMPANY_STATUS_ARCHIVED:
		return "archived"

	default:
		return "unknown"
	}
}

func agentStatus(value organizationv1.AgentStatus) string {
	switch value {
	case organizationv1.AgentStatus_AGENT_STATUS_DRAFT:
		return "draft"

	case organizationv1.AgentStatus_AGENT_STATUS_PROVISIONING:
		return "provisioning"

	case organizationv1.AgentStatus_AGENT_STATUS_CONFIGURING_RUNTIME:
		return "configuring_runtime"

	case organizationv1.AgentStatus_AGENT_STATUS_HEALTH_CHECKING:
		return "health_checking"

	case organizationv1.AgentStatus_AGENT_STATUS_READY:
		return "ready"

	case organizationv1.AgentStatus_AGENT_STATUS_DEGRADED:
		return "degraded"

	case organizationv1.AgentStatus_AGENT_STATUS_PAUSED:
		return "paused"

	case organizationv1.AgentStatus_AGENT_STATUS_FAILED:
		return "failed"

	case organizationv1.AgentStatus_AGENT_STATUS_DELETING:
		return "deleting"

	case organizationv1.AgentStatus_AGENT_STATUS_DELETED:
		return "deleted"

	default:
		return "unknown"
	}
}

func executionProfile(
	value organizationv1.AgentExecutionProfile,
) string {
	switch value {
	case organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_BALANCED:
		return "balanced"

	case organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_PERSISTENT:
		return "persistent"

	case organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_TECHNICAL_WORKER:
		return "technical_worker"

	default:
		return "unknown"
	}
}

func autonomyLevel(
	value organizationv1.AgentAutonomyLevel,
) string {
	switch value {
	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_OBSERVE_ONLY:
		return "observe_only"

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_SUGGEST:
		return "suggest"

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_APPROVAL_REQUIRED:
		return "approval_required"

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_POLICY_BOUNDED:
		return "policy_bounded"

	default:
		return "unknown"
	}
}

func goalStatus(value workv1.GoalStatus) string {
	switch value {
	case workv1.GoalStatus_GOAL_STATUS_ACTIVE:
		return "active"

	case workv1.GoalStatus_GOAL_STATUS_COMPLETED:
		return "completed"

	case workv1.GoalStatus_GOAL_STATUS_CANCELED:
		return "canceled"

	case workv1.GoalStatus_GOAL_STATUS_ARCHIVED:
		return "archived"

	default:
		return "unknown"
	}
}

func projectStatus(value workv1.ProjectStatus) string {
	switch value {
	case workv1.ProjectStatus_PROJECT_STATUS_PLANNED:
		return "planned"

	case workv1.ProjectStatus_PROJECT_STATUS_ACTIVE:
		return "active"

	case workv1.ProjectStatus_PROJECT_STATUS_ON_HOLD:
		return "on_hold"

	case workv1.ProjectStatus_PROJECT_STATUS_COMPLETED:
		return "completed"

	case workv1.ProjectStatus_PROJECT_STATUS_CANCELED:
		return "canceled"

	case workv1.ProjectStatus_PROJECT_STATUS_ARCHIVED:
		return "archived"

	default:
		return "unknown"
	}
}

func taskStatus(value workv1.TaskStatus) string {
	switch value {
	case workv1.TaskStatus_TASK_STATUS_BACKLOG:
		return "backlog"

	case workv1.TaskStatus_TASK_STATUS_READY:
		return "ready"

	case workv1.TaskStatus_TASK_STATUS_IN_PROGRESS:
		return "in_progress"

	case workv1.TaskStatus_TASK_STATUS_WAITING_APPROVAL:
		return "waiting_approval"

	case workv1.TaskStatus_TASK_STATUS_COMPLETED:
		return "completed"

	case workv1.TaskStatus_TASK_STATUS_CANCELED:
		return "canceled"

	default:
		return "unknown"
	}
}

func taskPriority(value workv1.TaskPriority) string {
	switch value {
	case workv1.TaskPriority_TASK_PRIORITY_LOW:
		return "low"

	case workv1.TaskPriority_TASK_PRIORITY_NORMAL:
		return "normal"

	case workv1.TaskPriority_TASK_PRIORITY_HIGH:
		return "high"

	case workv1.TaskPriority_TASK_PRIORITY_URGENT:
		return "urgent"

	default:
		return "unknown"
	}
}

func assigneeType(value workv1.AssigneeType) string {
	switch value {
	case workv1.AssigneeType_ASSIGNEE_TYPE_USER:
		return "user"

	case workv1.AssigneeType_ASSIGNEE_TYPE_AGENT:
		return "agent"

	default:
		return "unknown"
	}
}

func assignmentRole(value workv1.AssignmentRole) string {
	switch value {
	case workv1.AssignmentRole_ASSIGNMENT_ROLE_PRIMARY:
		return "primary"

	case workv1.AssignmentRole_ASSIGNMENT_ROLE_COLLABORATOR:
		return "collaborator"

	case workv1.AssignmentRole_ASSIGNMENT_ROLE_REVIEWER:
		return "reviewer"

	default:
		return "unknown"
	}
}

func protoTime(
	value *timestamppb.Timestamp,
) time.Time {
	if value == nil {
		return time.Time{}
	}

	return value.AsTime().UTC()
}
