package protoutil

import (
	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Tenant maps a domain tenant to Protobuf.
func Tenant(value domain.Tenant) *tenantv1.Tenant {
	message := &tenantv1.Tenant{
		TenantId:        value.ID,
		Slug:            value.Slug,
		DisplayName:     value.DisplayName,
		Status:          Status(value.Status),
		Region:          value.Region,
		RetentionDays:   value.RetentionDays,
		Version:         value.Version,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       timestamppb.New(value.CreatedAt),
		UpdatedAt:       timestamppb.New(value.UpdatedAt),
	}

	if value.ArchivedAt != nil {
		message.ArchivedAt = timestamppb.New(
			*value.ArchivedAt,
		)
	}

	return message
}

// Membership maps a domain membership to Protobuf.
func Membership(
	value domain.Membership,
) *tenantv1.TenantMembership {
	return &tenantv1.TenantMembership{
		TenantId:        value.TenantID,
		UserId:          value.UserID,
		Role:            Role(value.Role),
		Version:         value.Version,
		CreatedByUserId: value.CreatedBy,
		CreatedAt:       timestamppb.New(value.CreatedAt),
		UpdatedAt:       timestamppb.New(value.UpdatedAt),
	}
}

// Entitlements maps domain entitlements to Protobuf.
func Entitlements(
	value domain.Entitlements,
) *tenantv1.TenantEntitlements {
	return &tenantv1.TenantEntitlements{
		TenantId:  value.TenantID,
		PlanKey:   value.PlanKey,
		Features:  cloneBoolMap(value.Features),
		Limits:    cloneIntMap(value.Limits),
		Version:   value.Version,
		UpdatedAt: timestamppb.New(value.UpdatedAt),
	}
}

// Context maps a trusted tenant context to Protobuf.
func Context(
	value domain.TenantContext,
) *tenantv1.TenantContext {
	permissions := make(
		[]tenantv1.Permission,
		0,
		len(value.Permissions),
	)

	for _, permission := range value.Permissions {
		permissions = append(
			permissions,
			Permission(permission),
		)
	}

	return &tenantv1.TenantContext{
		Tenant:       Tenant(value.Tenant),
		Membership:   Membership(value.Membership),
		Entitlements: Entitlements(value.Entitlements),
		Permissions:  permissions,
	}
}

// Permission maps a domain permission to Protobuf.
func Permission(
	value domain.Permission,
) tenantv1.Permission {
	switch value {
	case domain.PermissionTenantRead:
		return tenantv1.Permission_PERMISSION_TENANT_READ

	case domain.PermissionTenantUpdate:
		return tenantv1.Permission_PERMISSION_TENANT_UPDATE

	case domain.PermissionTenantArchive:
		return tenantv1.Permission_PERMISSION_TENANT_ARCHIVE

	case domain.PermissionMemberList:
		return tenantv1.Permission_PERMISSION_MEMBER_LIST

	case domain.PermissionMemberAdd:
		return tenantv1.Permission_PERMISSION_MEMBER_ADD

	case domain.PermissionMemberUpdateRole:
		return tenantv1.Permission_PERMISSION_MEMBER_UPDATE_ROLE

	case domain.PermissionMemberRemove:
		return tenantv1.Permission_PERMISSION_MEMBER_REMOVE

	case domain.PermissionEntitlementRead:
		return tenantv1.Permission_PERMISSION_ENTITLEMENT_READ

	case domain.PermissionCompanyRead:
		return tenantv1.Permission_PERMISSION_COMPANY_READ

	case domain.PermissionCompanyCreate:
		return tenantv1.Permission_PERMISSION_COMPANY_CREATE

	case domain.PermissionCompanyUpdate:
		return tenantv1.Permission_PERMISSION_COMPANY_UPDATE

	case domain.PermissionCompanyArchive:
		return tenantv1.Permission_PERMISSION_COMPANY_ARCHIVE

	case domain.PermissionAgentRead:
		return tenantv1.Permission_PERMISSION_AGENT_READ

	case domain.PermissionAgentCreate:
		return tenantv1.Permission_PERMISSION_AGENT_CREATE

	case domain.PermissionAgentUpdate:
		return tenantv1.Permission_PERMISSION_AGENT_UPDATE

	case domain.PermissionAgentDelete:
		return tenantv1.Permission_PERMISSION_AGENT_DELETE

	case domain.PermissionAgentHierarchyManage:
		return tenantv1.Permission_PERMISSION_AGENT_HIERARCHY_MANAGE

	case domain.PermissionAgentLifecycleManage:
		return tenantv1.Permission_PERMISSION_AGENT_LIFECYCLE_MANAGE

	case domain.PermissionWorkRead:
		return tenantv1.Permission_PERMISSION_WORK_READ

	case domain.PermissionGoalCreate:
		return tenantv1.Permission_PERMISSION_GOAL_CREATE

	case domain.PermissionGoalUpdate:
		return tenantv1.Permission_PERMISSION_GOAL_UPDATE

	case domain.PermissionProjectCreate:
		return tenantv1.Permission_PERMISSION_PROJECT_CREATE

	case domain.PermissionProjectUpdate:
		return tenantv1.Permission_PERMISSION_PROJECT_UPDATE

	case domain.PermissionTaskCreate:
		return tenantv1.Permission_PERMISSION_TASK_CREATE

	case domain.PermissionTaskUpdate:
		return tenantv1.Permission_PERMISSION_TASK_UPDATE

	case domain.PermissionTaskStatusUpdate:
		return tenantv1.Permission_PERMISSION_TASK_STATUS_UPDATE

	case domain.PermissionTaskAssign:
		return tenantv1.Permission_PERMISSION_TASK_ASSIGN

	case domain.PermissionTaskDependencyManage:
		return tenantv1.Permission_PERMISSION_TASK_DEPENDENCY_MANAGE

	case domain.PermissionTaskCommentCreate:
		return tenantv1.Permission_PERMISSION_TASK_COMMENT_CREATE

	case domain.PermissionTaskCommentModerate:
		return tenantv1.Permission_PERMISSION_TASK_COMMENT_MODERATE

	case domain.PermissionTaskArtifactManage:
		return tenantv1.Permission_PERMISSION_TASK_ARTIFACT_MANAGE

	case domain.PermissionTaskChecklistManage:
		return tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE

	case domain.PermissionTaskScheduleManage:
		return tenantv1.Permission_PERMISSION_TASK_SCHEDULE_MANAGE

	case domain.PermissionPolicyRead:
		return tenantv1.Permission_PERMISSION_POLICY_READ

	case domain.PermissionPolicyCreate:
		return tenantv1.Permission_PERMISSION_POLICY_CREATE

	case domain.PermissionPolicyUpdate:
		return tenantv1.Permission_PERMISSION_POLICY_UPDATE

	case domain.PermissionPolicyActivate:
		return tenantv1.Permission_PERMISSION_POLICY_ACTIVATE

	case domain.PermissionPolicyArchive:
		return tenantv1.Permission_PERMISSION_POLICY_ARCHIVE

	case domain.PermissionPolicyDecisionRead:
		return tenantv1.Permission_PERMISSION_POLICY_DECISION_READ

	default:
		return tenantv1.Permission_PERMISSION_UNSPECIFIED
	}
}

// DomainPermission maps a Protobuf permission to the domain.
func DomainPermission(
	value tenantv1.Permission,
) domain.Permission {
	switch value {
	case tenantv1.Permission_PERMISSION_TENANT_READ:
		return domain.PermissionTenantRead

	case tenantv1.Permission_PERMISSION_TENANT_UPDATE:
		return domain.PermissionTenantUpdate

	case tenantv1.Permission_PERMISSION_TENANT_ARCHIVE:
		return domain.PermissionTenantArchive

	case tenantv1.Permission_PERMISSION_MEMBER_LIST:
		return domain.PermissionMemberList

	case tenantv1.Permission_PERMISSION_MEMBER_ADD:
		return domain.PermissionMemberAdd

	case tenantv1.Permission_PERMISSION_MEMBER_UPDATE_ROLE:
		return domain.PermissionMemberUpdateRole

	case tenantv1.Permission_PERMISSION_MEMBER_REMOVE:
		return domain.PermissionMemberRemove

	case tenantv1.Permission_PERMISSION_ENTITLEMENT_READ:
		return domain.PermissionEntitlementRead

	case tenantv1.Permission_PERMISSION_COMPANY_READ:
		return domain.PermissionCompanyRead

	case tenantv1.Permission_PERMISSION_COMPANY_CREATE:
		return domain.PermissionCompanyCreate

	case tenantv1.Permission_PERMISSION_COMPANY_UPDATE:
		return domain.PermissionCompanyUpdate

	case tenantv1.Permission_PERMISSION_COMPANY_ARCHIVE:
		return domain.PermissionCompanyArchive

	case tenantv1.Permission_PERMISSION_AGENT_READ:
		return domain.PermissionAgentRead

	case tenantv1.Permission_PERMISSION_AGENT_CREATE:
		return domain.PermissionAgentCreate

	case tenantv1.Permission_PERMISSION_AGENT_UPDATE:
		return domain.PermissionAgentUpdate

	case tenantv1.Permission_PERMISSION_AGENT_DELETE:
		return domain.PermissionAgentDelete

	case tenantv1.Permission_PERMISSION_AGENT_HIERARCHY_MANAGE:
		return domain.PermissionAgentHierarchyManage

	case tenantv1.Permission_PERMISSION_AGENT_LIFECYCLE_MANAGE:
		return domain.PermissionAgentLifecycleManage

	case tenantv1.Permission_PERMISSION_WORK_READ:
		return domain.PermissionWorkRead

	case tenantv1.Permission_PERMISSION_GOAL_CREATE:
		return domain.PermissionGoalCreate

	case tenantv1.Permission_PERMISSION_GOAL_UPDATE:
		return domain.PermissionGoalUpdate

	case tenantv1.Permission_PERMISSION_PROJECT_CREATE:
		return domain.PermissionProjectCreate

	case tenantv1.Permission_PERMISSION_PROJECT_UPDATE:
		return domain.PermissionProjectUpdate

	case tenantv1.Permission_PERMISSION_TASK_CREATE:
		return domain.PermissionTaskCreate

	case tenantv1.Permission_PERMISSION_TASK_UPDATE:
		return domain.PermissionTaskUpdate

	case tenantv1.Permission_PERMISSION_TASK_STATUS_UPDATE:
		return domain.PermissionTaskStatusUpdate

	case tenantv1.Permission_PERMISSION_TASK_ASSIGN:
		return domain.PermissionTaskAssign

	case tenantv1.Permission_PERMISSION_TASK_DEPENDENCY_MANAGE:
		return domain.PermissionTaskDependencyManage

	case tenantv1.Permission_PERMISSION_TASK_COMMENT_CREATE:
		return domain.PermissionTaskCommentCreate

	case tenantv1.Permission_PERMISSION_TASK_COMMENT_MODERATE:
		return domain.PermissionTaskCommentModerate

	case tenantv1.Permission_PERMISSION_TASK_ARTIFACT_MANAGE:
		return domain.PermissionTaskArtifactManage

	case tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE:
		return domain.PermissionTaskChecklistManage

	case tenantv1.Permission_PERMISSION_TASK_SCHEDULE_MANAGE:
		return domain.PermissionTaskScheduleManage

	case tenantv1.Permission_PERMISSION_POLICY_READ:
		return domain.PermissionPolicyRead

	case tenantv1.Permission_PERMISSION_POLICY_CREATE:
		return domain.PermissionPolicyCreate

	case tenantv1.Permission_PERMISSION_POLICY_UPDATE:
		return domain.PermissionPolicyUpdate

	case tenantv1.Permission_PERMISSION_POLICY_ACTIVATE:
		return domain.PermissionPolicyActivate

	case tenantv1.Permission_PERMISSION_POLICY_ARCHIVE:
		return domain.PermissionPolicyArchive

	case tenantv1.Permission_PERMISSION_POLICY_DECISION_READ:
		return domain.PermissionPolicyDecisionRead

	default:
		return ""
	}
}

// DenialReason maps a domain denial reason to Protobuf.
func DenialReason(
	value domain.DenialReason,
) tenantv1.AuthorizationDenialReason {
	switch value {
	case domain.DenialReasonNone:
		return tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_NONE

	case domain.DenialReasonNotMember:
		return tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_NOT_A_MEMBER

	case domain.DenialReasonTenantArchived:
		return tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_TENANT_ARCHIVED

	case domain.DenialReasonPermissionNotGranted:
		return tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_PERMISSION_NOT_GRANTED

	case domain.DenialReasonTenantNotActive:
		return tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_TENANT_NOT_ACTIVE

	default:
		return tenantv1.AuthorizationDenialReason_AUTHORIZATION_DENIAL_REASON_UNSPECIFIED
	}
}

// AuthorizationDecision maps a domain decision to Protobuf.
func AuthorizationDecision(
	value domain.AuthorizationDecision,
) *tenantv1.AuthorizationDecision {
	return &tenantv1.AuthorizationDecision{
		Allowed:           value.Allowed,
		TenantId:          value.TenantID,
		ActorUserId:       value.ActorUserID,
		Permission:        Permission(value.Permission),
		Role:              Role(value.Role),
		TenantVersion:     value.TenantVersion,
		MembershipVersion: value.MembershipVersion,
		DenialReason:      DenialReason(value.DenialReason),
	}
}

// Role maps a domain role to Protobuf.
func Role(value domain.Role) tenantv1.TenantRole {
	switch value {
	case domain.RoleOwner:
		return tenantv1.TenantRole_TENANT_ROLE_OWNER
	case domain.RoleAdmin:
		return tenantv1.TenantRole_TENANT_ROLE_ADMIN
	case domain.RoleMember:
		return tenantv1.TenantRole_TENANT_ROLE_MEMBER
	case domain.RoleViewer:
		return tenantv1.TenantRole_TENANT_ROLE_VIEWER
	default:
		return tenantv1.TenantRole_TENANT_ROLE_UNSPECIFIED
	}
}

// DomainRole maps a Protobuf role to the tenant domain.
func DomainRole(value tenantv1.TenantRole) domain.Role {
	switch value {
	case tenantv1.TenantRole_TENANT_ROLE_OWNER:
		return domain.RoleOwner
	case tenantv1.TenantRole_TENANT_ROLE_ADMIN:
		return domain.RoleAdmin
	case tenantv1.TenantRole_TENANT_ROLE_MEMBER:
		return domain.RoleMember
	case tenantv1.TenantRole_TENANT_ROLE_VIEWER:
		return domain.RoleViewer
	default:
		return ""
	}
}

// Status maps a domain status to Protobuf.
func Status(value domain.Status) tenantv1.TenantStatus {
	switch value {
	case domain.StatusProvisioning:
		return tenantv1.TenantStatus_TENANT_STATUS_PROVISIONING
	case domain.StatusActive:
		return tenantv1.TenantStatus_TENANT_STATUS_ACTIVE
	case domain.StatusProvisioningFailed:
		return tenantv1.TenantStatus_TENANT_STATUS_PROVISIONING_FAILED
	case domain.StatusArchived:
		return tenantv1.TenantStatus_TENANT_STATUS_ARCHIVED
	default:
		return tenantv1.TenantStatus_TENANT_STATUS_UNSPECIFIED
	}
}

// Operation maps a domain operation to Protobuf.
func Operation(
	value domain.Operation,
) *commonv1.Operation {
	operation := &commonv1.Operation{
		OperationId:   value.ID,
		TenantId:      value.TenantID,
		ActorUserId:   value.ActorUserID,
		RequestId:     value.RequestID,
		ResourceName:  value.ResourceName,
		WorkflowId:    value.WorkflowID,
		WorkflowRunId: value.WorkflowRunID,
		State:         OperationState(value.State),
		Version:       value.Version,
		Metadata:      value.Metadata,
		CreatedAt:     timestamppb.New(value.CreatedAt),
		UpdatedAt:     timestamppb.New(value.UpdatedAt),
	}

	if value.Error != nil {
		operation.Error = &commonv1.OperationError{
			Code:      value.Error.Code,
			Message:   value.Error.Message,
			Retryable: value.Error.Retryable,
		}

		if len(value.Error.Details) > 0 {
			details, err := structpb.NewStruct(
				value.Error.Details,
			)
			if err == nil {
				operation.Error.Details = details
			}
		}
	}

	if value.StartedAt != nil {
		operation.StartedAt = timestamppb.New(
			*value.StartedAt,
		)
	}

	if value.CompletedAt != nil {
		operation.CompletedAt = timestamppb.New(
			*value.CompletedAt,
		)
	}

	return operation
}

// OperationState maps a domain operation state to Protobuf.
func OperationState(
	value domain.OperationState,
) commonv1.OperationState {
	switch value {
	case domain.OperationStatePending:
		return commonv1.OperationState_OPERATION_STATE_PENDING
	case domain.OperationStateRunning:
		return commonv1.OperationState_OPERATION_STATE_RUNNING
	case domain.OperationStateSucceeded:
		return commonv1.OperationState_OPERATION_STATE_SUCCEEDED
	case domain.OperationStateFailed:
		return commonv1.OperationState_OPERATION_STATE_FAILED
	case domain.OperationStateCanceled:
		return commonv1.OperationState_OPERATION_STATE_CANCELED
	default:
		return commonv1.OperationState_OPERATION_STATE_UNSPECIFIED
	}
}

func cloneBoolMap(
	source map[string]bool,
) map[string]bool {
	result := make(map[string]bool, len(source))

	for key, value := range source {
		result[key] = value
	}

	return result
}

func cloneIntMap(
	source map[string]int64,
) map[string]int64 {
	result := make(map[string]int64, len(source))

	for key, value := range source {
		result[key] = value
	}

	return result
}
