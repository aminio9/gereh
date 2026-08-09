// Package protoutil maps projection domain values to Protobuf messages.
package protoutil

import (
	"time"

	projectionv1 "github.com/aminio9/gereh/gen/go/gereh/projection/v1"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Metadata maps projection metadata to the Protobuf message.
func Metadata(
	value domain.ProjectionMetadata,
) *projectionv1.ProjectionMetadata {
	return &projectionv1.ProjectionMetadata{
		ProjectedThroughEventTime: protoTimestamp(&value.ProjectedThroughEventTime),
		LastProcessedAt:           protoTimestamp(&value.LastProcessedAt),
	}
}

// DashboardSummary maps a domain dashboard summary to the Protobuf message.
func DashboardSummary(
	value domain.DashboardSummary,
) *projectionv1.DashboardSummary {
	return &projectionv1.DashboardSummary{
		CompaniesTotal:  value.CompaniesTotal,
		CompaniesActive: value.CompaniesActive,

		AgentsTotal:    value.AgentsTotal,
		AgentsReady:    value.AgentsReady,
		AgentsDegraded: value.AgentsDegraded,
		AgentsPaused:   value.AgentsPaused,
		AgentsFailed:   value.AgentsFailed,

		GoalsActive:    value.GoalsActive,
		GoalsCompleted: value.GoalsCompleted,

		ProjectsActive:    value.ProjectsActive,
		ProjectsOnHold:    value.ProjectsOnHold,
		ProjectsCompleted: value.ProjectsCompleted,

		TasksTotal:           value.TasksTotal,
		TasksBacklog:         value.TasksBacklog,
		TasksReady:           value.TasksReady,
		TasksInProgress:      value.TasksInProgress,
		TasksWaitingApproval: value.TasksWaitingApproval,
		TasksCompleted:       value.TasksCompleted,
		TasksCanceled:        value.TasksCanceled,
		TasksBlocked:         value.TasksBlocked,
	}
}

// CompanyOverview maps a domain company overview to the Protobuf message.
func CompanyOverview(
	value domain.CompanyOverview,
) *projectionv1.CompanyOverview {
	return &projectionv1.CompanyOverview{
		TenantId:  value.TenantID,
		CompanyId: value.CompanyID,

		Slug:        value.Slug,
		DisplayName: value.DisplayName,
		Status:      value.Status,
		IsDefault:   value.IsDefault,

		AgentsTotal:    value.AgentsTotal,
		AgentsReady:    value.AgentsReady,
		AgentsDegraded: value.AgentsDegraded,
		AgentsPaused:   value.AgentsPaused,

		TasksTotal:           value.TasksTotal,
		TasksReady:           value.TasksReady,
		TasksInProgress:      value.TasksInProgress,
		TasksWaitingApproval: value.TasksWaitingApproval,
		TasksCompleted:       value.TasksCompleted,
		TasksBlocked:         value.TasksBlocked,
	}
}

// AgentOverview maps a domain agent overview to the Protobuf message.
func AgentOverview(
	value domain.AgentOverview,
) *projectionv1.AgentOverview {
	return &projectionv1.AgentOverview{
		TenantId:  value.TenantID,
		CompanyId: value.CompanyID,
		AgentId:   value.AgentID,

		Slug:        value.Slug,
		DisplayName: value.DisplayName,
		RoleTitle:   value.RoleTitle,
		Status:      value.Status,

		ManagerAgentId: optionalString(value.ManagerAgentID),

		AssignedTaskCount: value.AssignedTaskCount,
		ActiveTaskCount:   value.ActiveTaskCount,

		UpdatedAt: protoTimestamp(&value.UpdatedAt),
	}
}

// Activity maps a domain activity item to the Protobuf message.
func Activity(
	value domain.Activity,
) *projectionv1.TaskActivityItem {
	return &projectionv1.TaskActivityItem{
		EventId:   value.EventID,
		EventType: value.EventType,

		CompanyId: optionalString(value.CompanyID),
		ProjectId: optionalString(value.ProjectID),
		TaskId:    optionalString(value.TaskID),

		ActorType: optionalString(value.ActorType),
		ActorId:   optionalString(value.ActorID),

		Summary: value.Summary,

		OccurredAt: protoTimestamp(&value.OccurredAt),
	}
}

// SearchResult maps a domain search result to the Protobuf message.
func SearchResult(
	value domain.SearchResult,
) *projectionv1.SearchResult {
	return &projectionv1.SearchResult{
		Type:      documentType(value.Type),
		Id:        value.ID,
		CompanyId: optionalString(value.CompanyID),

		Title:    value.Title,
		Subtitle: value.Subtitle,
		Status:   value.Status,

		Rank: value.Rank,

		UpdatedAt: protoTimestamp(&value.UpdatedAt),
	}
}

func documentType(
	value string,
) projectionv1.SearchDocumentType {
	switch value {
	case "company":
		return projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_COMPANY

	case "agent":
		return projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_AGENT

	case "goal":
		return projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_GOAL

	case "project":
		return projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_PROJECT

	case "task":
		return projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_TASK

	default:
		return projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_UNSPECIFIED
	}
}

func optionalString(
	value *string,
) *string {
	if value == nil {
		return nil
	}

	cloned := *value

	return &cloned
}

func protoTimestamp(
	value *time.Time,
) *timestamppb.Timestamp {
	if value == nil || value.IsZero() {
		return nil
	}

	return timestamppb.New(*value)
}
