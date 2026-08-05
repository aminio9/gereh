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

// CanReadTenant reports whether a role can read tenant state.
func CanReadTenant(role Role) bool {
	return IsKnownRole(role)
}

// CanUpdateTenant reports whether a role can update tenant settings.
func CanUpdateTenant(role Role) bool {
	return role == RoleOwner || role == RoleAdmin
}

// CanArchiveTenant reports whether a role can archive a tenant.
func CanArchiveTenant(role Role) bool {
	return role == RoleOwner
}

// CanManageMember reports whether an actor may manage a target membership.
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
