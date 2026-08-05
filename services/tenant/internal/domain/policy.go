package domain

// IsKnownRole reports whether a role is supported.
func IsKnownRole(role Role) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true

	default:
		return false
	}
}

// CanReadTenant remains as a compatibility helper for repository enforcement.
func CanReadTenant(role Role) bool {
	return RoleAllows(
		role,
		PermissionTenantRead,
	)
}

// CanUpdateTenant remains as a compatibility helper for repository enforcement.
func CanUpdateTenant(role Role) bool {
	return RoleAllows(
		role,
		PermissionTenantUpdate,
	)
}

// CanArchiveTenant remains as a compatibility helper for repository enforcement.
func CanArchiveTenant(role Role) bool {
	return RoleAllows(
		role,
		PermissionTenantArchive,
	)
}

// CanManageMember validates target-role constraints.
//
// Capability authorization must be checked before this function. This function
// enforces hierarchy rules that cannot be represented by a simple permission.
func CanManageMember(
	actor Role,
	target Role,
	newRole Role,
) bool {
	switch actor {
	case RoleOwner:
		return IsKnownRole(newRole)

	case RoleAdmin:
		return target != RoleOwner &&
			newRole != RoleOwner &&
			IsKnownRole(newRole)

	default:
		return false
	}
}
