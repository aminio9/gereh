// Package protoutil maps organization domain types to Protobuf messages.
package protoutil

import (
	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Company maps a domain company to Protobuf.
func Company(
	value domain.Company,
) *organizationv1.Company {
	result := &organizationv1.Company{
		TenantId:        value.TenantID,
		CompanyId:       value.ID,
		Slug:            value.Slug,
		DisplayName:     value.DisplayName,
		Description:     value.Description,
		Status:          CompanyStatus(value.Status),
		IsDefault:       value.IsDefault,
		Version:         value.Version,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       timestamppb.New(value.CreatedAt),
		UpdatedAt:       timestamppb.New(value.UpdatedAt),
	}

	if value.ArchivedAt != nil {
		result.ArchivedAt = timestamppb.New(*value.ArchivedAt)
	}

	return result
}

// Agent maps a domain agent to Protobuf.
func Agent(
	value domain.Agent,
) *organizationv1.Agent {
	configuration, _ := structpb.NewStruct(value.Configuration)

	result := &organizationv1.Agent{
		TenantId:    value.TenantID,
		CompanyId:   value.CompanyID,
		AgentId:     value.ID,
		Slug:        value.Slug,
		DisplayName: value.DisplayName,
		RoleTitle:   value.RoleTitle,
		Objective:   value.Objective,
		Status:      AgentStatus(value.Status),
		ExecutionProfile: executionProfile(
			value.ExecutionProfile,
		),
		AutonomyLevel:   AutonomyLevel(value.AutonomyLevel),
		Capabilities:    append([]string(nil), value.Capabilities...),
		Configuration:   configuration,
		Version:         value.Version,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       timestamppb.New(value.CreatedAt),
		UpdatedAt:       timestamppb.New(value.UpdatedAt),
	}

	if value.ManagerAgentID != nil {
		result.ManagerAgentId = value.ManagerAgentID
	}

	if value.DeletedAt != nil {
		result.DeletedAt = timestamppb.New(*value.DeletedAt)
	}

	return result
}

// CompanyStatus maps a domain company status to Protobuf.
func CompanyStatus(
	value domain.CompanyStatus,
) organizationv1.CompanyStatus {
	switch value {
	case domain.CompanyStatusActive:
		return organizationv1.CompanyStatus_COMPANY_STATUS_ACTIVE

	case domain.CompanyStatusArchived:
		return organizationv1.CompanyStatus_COMPANY_STATUS_ARCHIVED

	default:
		return organizationv1.CompanyStatus_COMPANY_STATUS_UNSPECIFIED
	}
}

// AgentStatus maps a domain agent status to Protobuf.
func AgentStatus(
	value domain.AgentStatus,
) organizationv1.AgentStatus {
	switch value {
	case domain.AgentStatusDraft:
		return organizationv1.AgentStatus_AGENT_STATUS_DRAFT

	case domain.AgentStatusProvisioning:
		return organizationv1.AgentStatus_AGENT_STATUS_PROVISIONING

	case domain.AgentStatusConfiguringRuntime:
		return organizationv1.AgentStatus_AGENT_STATUS_CONFIGURING_RUNTIME

	case domain.AgentStatusHealthChecking:
		return organizationv1.AgentStatus_AGENT_STATUS_HEALTH_CHECKING

	case domain.AgentStatusReady:
		return organizationv1.AgentStatus_AGENT_STATUS_READY

	case domain.AgentStatusDegraded:
		return organizationv1.AgentStatus_AGENT_STATUS_DEGRADED

	case domain.AgentStatusPaused:
		return organizationv1.AgentStatus_AGENT_STATUS_PAUSED

	case domain.AgentStatusFailed:
		return organizationv1.AgentStatus_AGENT_STATUS_FAILED

	case domain.AgentStatusDeleting:
		return organizationv1.AgentStatus_AGENT_STATUS_DELETING

	case domain.AgentStatusDeleted:
		return organizationv1.AgentStatus_AGENT_STATUS_DELETED

	default:
		return organizationv1.AgentStatus_AGENT_STATUS_UNSPECIFIED
	}
}

// DomainAgentStatus maps a Protobuf agent status to the domain.
func DomainAgentStatus(
	value organizationv1.AgentStatus,
) domain.AgentStatus {
	switch value {
	case organizationv1.AgentStatus_AGENT_STATUS_DRAFT:
		return domain.AgentStatusDraft

	case organizationv1.AgentStatus_AGENT_STATUS_PROVISIONING:
		return domain.AgentStatusProvisioning

	case organizationv1.AgentStatus_AGENT_STATUS_CONFIGURING_RUNTIME:
		return domain.AgentStatusConfiguringRuntime

	case organizationv1.AgentStatus_AGENT_STATUS_HEALTH_CHECKING:
		return domain.AgentStatusHealthChecking

	case organizationv1.AgentStatus_AGENT_STATUS_READY:
		return domain.AgentStatusReady

	case organizationv1.AgentStatus_AGENT_STATUS_DEGRADED:
		return domain.AgentStatusDegraded

	case organizationv1.AgentStatus_AGENT_STATUS_PAUSED:
		return domain.AgentStatusPaused

	case organizationv1.AgentStatus_AGENT_STATUS_FAILED:
		return domain.AgentStatusFailed

	case organizationv1.AgentStatus_AGENT_STATUS_DELETING:
		return domain.AgentStatusDeleting

	case organizationv1.AgentStatus_AGENT_STATUS_DELETED:
		return domain.AgentStatusDeleted

	default:
		return ""
	}
}

func executionProfile(
	value domain.ExecutionProfile,
) organizationv1.AgentExecutionProfile {
	switch value {
	case domain.ExecutionProfileBalanced:
		return organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_BALANCED

	case domain.ExecutionProfilePersistent:
		return organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_PERSISTENT

	case domain.ExecutionProfileTechnicalWorker:
		return organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_TECHNICAL_WORKER

	default:
		return organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_UNSPECIFIED
	}
}

// DomainExecutionProfile maps a Protobuf profile to the domain.
func DomainExecutionProfile(
	value organizationv1.AgentExecutionProfile,
) domain.ExecutionProfile {
	switch value {
	case organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_BALANCED:
		return domain.ExecutionProfileBalanced

	case organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_PERSISTENT:
		return domain.ExecutionProfilePersistent

	case organizationv1.AgentExecutionProfile_AGENT_EXECUTION_PROFILE_TECHNICAL_WORKER:
		return domain.ExecutionProfileTechnicalWorker

	default:
		return ""
	}
}

// AutonomyLevel maps a domain autonomy level to its gRPC enum.
func AutonomyLevel(
	value domain.AutonomyLevel,
) organizationv1.AgentAutonomyLevel {
	switch value {
	case domain.AutonomyObserveOnly:
		return organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_OBSERVE_ONLY

	case domain.AutonomySuggest:
		return organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_SUGGEST

	case domain.AutonomyApprovalRequired:
		return organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_APPROVAL_REQUIRED

	case domain.AutonomyPolicyBounded:
		return organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_POLICY_BOUNDED

	default:
		return organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_UNSPECIFIED
	}
}

// DomainAutonomyLevel maps a Protobuf autonomy level to the domain.
func DomainAutonomyLevel(
	value organizationv1.AgentAutonomyLevel,
) domain.AutonomyLevel {
	switch value {
	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_OBSERVE_ONLY:
		return domain.AutonomyObserveOnly

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_SUGGEST:
		return domain.AutonomySuggest

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_APPROVAL_REQUIRED:
		return domain.AutonomyApprovalRequired

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_POLICY_BOUNDED:
		return domain.AutonomyPolicyBounded

	default:
		return ""
	}
}
