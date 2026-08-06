package domain

import "slices"

// Permission is a stable tenant-scoped capability.
type Permission string

// Permission values.
const (
	PermissionTenantRead    Permission = "tenant.read"
	PermissionTenantUpdate  Permission = "tenant.update"
	PermissionTenantArchive Permission = "tenant.archive"

	PermissionMemberList       Permission = "member.list"
	PermissionMemberAdd        Permission = "member.add"
	PermissionMemberUpdateRole Permission = "member.update_role"
	PermissionMemberRemove     Permission = "member.remove"

	PermissionEntitlementRead Permission = "entitlement.read"

	PermissionCompanyRead    Permission = "company.read"
	PermissionCompanyCreate  Permission = "company.create"
	PermissionCompanyUpdate  Permission = "company.update"
	PermissionCompanyArchive Permission = "company.archive"

	PermissionAgentRead            Permission = "agent.read"
	PermissionAgentCreate          Permission = "agent.create"
	PermissionAgentUpdate          Permission = "agent.update"
	PermissionAgentDelete          Permission = "agent.delete"
	PermissionAgentHierarchyManage Permission = "agent.hierarchy_manage"
	PermissionAgentLifecycleManage Permission = "agent.lifecycle_manage"
)

// DenialReason identifies why authorization was denied.
type DenialReason string

// DenialReason values.
const (
	DenialReasonNone                 DenialReason = "none"
	DenialReasonNotMember            DenialReason = "not_member"
	DenialReasonTenantArchived       DenialReason = "tenant_archived"
	DenialReasonTenantNotActive      DenialReason = "tenant_not_active"
	DenialReasonPermissionNotGranted DenialReason = "permission_not_granted"
)

// AuthorizationDecision is the domain authorization result.
type AuthorizationDecision struct {
	Allowed           bool
	TenantID          string
	ActorUserID       string
	Permission        Permission
	Role              Role
	TenantVersion     int64
	MembershipVersion int64
	DenialReason      DenialReason
}

var knownPermissions = []Permission{
	PermissionTenantRead,
	PermissionTenantUpdate,
	PermissionTenantArchive,
	PermissionMemberList,
	PermissionMemberAdd,
	PermissionMemberUpdateRole,
	PermissionMemberRemove,
	PermissionEntitlementRead,

	PermissionCompanyRead,
	PermissionCompanyCreate,
	PermissionCompanyUpdate,
	PermissionCompanyArchive,

	PermissionAgentRead,
	PermissionAgentCreate,
	PermissionAgentUpdate,
	PermissionAgentDelete,
	PermissionAgentHierarchyManage,
	PermissionAgentLifecycleManage,
}

var ownerPermissions = append(
	[]Permission(nil),
	knownPermissions...,
)

var adminPermissions = []Permission{
	PermissionTenantRead,
	PermissionTenantUpdate,

	PermissionMemberList,
	PermissionMemberAdd,
	PermissionMemberUpdateRole,
	PermissionMemberRemove,

	PermissionEntitlementRead,

	PermissionCompanyRead,
	PermissionCompanyCreate,
	PermissionCompanyUpdate,
	PermissionCompanyArchive,

	PermissionAgentRead,
	PermissionAgentCreate,
	PermissionAgentUpdate,
	PermissionAgentDelete,
	PermissionAgentHierarchyManage,
	PermissionAgentLifecycleManage,
}

var memberPermissions = []Permission{
	PermissionTenantRead,
	PermissionMemberList,
	PermissionEntitlementRead,
	PermissionCompanyRead,
	PermissionAgentRead,
}

var viewerPermissions = []Permission{
	PermissionTenantRead,
	PermissionEntitlementRead,
	PermissionCompanyRead,
	PermissionAgentRead,
}

// IsKnownPermission reports whether a permission is supported.
func IsKnownPermission(permission Permission) bool {
	return slices.Contains(
		knownPermissions,
		permission,
	)
}

// PermissionsForRole returns a defensive copy of the role permission set.
func PermissionsForRole(role Role) []Permission {
	var permissions []Permission

	switch role {
	case RoleOwner:
		permissions = ownerPermissions

	case RoleAdmin:
		permissions = adminPermissions

	case RoleMember:
		permissions = memberPermissions

	case RoleViewer:
		permissions = viewerPermissions

	default:
		return nil
	}

	return append(
		[]Permission(nil),
		permissions...,
	)
}

// RoleAllows reports whether a role grants a permission.
//
// This method checks only the role mapping. Tenant lifecycle state is evaluated
// separately through EffectivePermissions and EvaluateAuthorization.
func RoleAllows(
	role Role,
	permission Permission,
) bool {
	return slices.Contains(
		PermissionsForRole(role),
		permission,
	)
}

// IsMutationPermission reports whether the permission changes tenant state.
func IsMutationPermission(
	permission Permission,
) bool {
	switch permission {
	case PermissionTenantUpdate,
		PermissionTenantArchive,
		PermissionMemberAdd,
		PermissionMemberUpdateRole,
		PermissionMemberRemove,
		PermissionCompanyCreate,
		PermissionCompanyUpdate,
		PermissionCompanyArchive,
		PermissionAgentCreate,
		PermissionAgentUpdate,
		PermissionAgentDelete,
		PermissionAgentHierarchyManage,
		PermissionAgentLifecycleManage:
		return true

	default:
		return false
	}
}

// EffectivePermissions returns permissions after applying tenant state.
//
// Non-active tenants remain readable but grant no mutation permissions.
func EffectivePermissions(
	status Status,
	role Role,
) []Permission {
	rolePermissions := PermissionsForRole(role)

	if status == StatusActive {
		return rolePermissions
	}

	result := make(
		[]Permission,
		0,
		len(rolePermissions),
	)

	for _, permission := range rolePermissions {
		if !IsMutationPermission(permission) {
			result = append(result, permission)
		}
	}

	return result
}

// EvaluateAuthorization evaluates a known tenant context.
func EvaluateAuthorization(
	tenant Tenant,
	membership Membership,
	permission Permission,
) AuthorizationDecision {
	decision := AuthorizationDecision{
		TenantID:          tenant.ID,
		ActorUserID:       membership.UserID,
		Permission:        permission,
		Role:              membership.Role,
		TenantVersion:     tenant.Version,
		MembershipVersion: membership.Version,
		DenialReason:      DenialReasonPermissionNotGranted,
	}

	if IsMutationPermission(permission) &&
		tenant.Status != StatusActive {
		if tenant.Status == StatusArchived {
			decision.DenialReason =
				DenialReasonTenantArchived
		} else {
			decision.DenialReason =
				DenialReasonTenantNotActive
		}

		return decision
	}

	if !RoleAllows(
		membership.Role,
		permission,
	) {
		return decision
	}

	decision.Allowed = true
	decision.DenialReason = DenialReasonNone

	return decision
}
