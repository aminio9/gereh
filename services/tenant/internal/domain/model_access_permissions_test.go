package domain

import (
	"slices"
	"testing"
)

func TestModelAccessPermissions(t *testing.T) {
	t.Parallel()

	admin := PermissionsForRole(RoleAdmin)

	for _, permission := range []Permission{
		PermissionModelProviderRead,
		PermissionModelConnectionRead,
		PermissionModelConnectionCreate,
		PermissionModelConnectionUpdate,
		PermissionModelConnectionArchive,
	} {
		if !slices.Contains(admin, permission) {
			t.Errorf("admin role must grant %q", permission)
		}
	}

	member := PermissionsForRole(RoleMember)

	if !slices.Contains(member, PermissionModelProviderRead) {
		t.Error("member role must grant model provider read")
	}

	for _, permission := range []Permission{
		PermissionModelConnectionRead,
		PermissionModelConnectionCreate,
		PermissionModelConnectionUpdate,
		PermissionModelConnectionArchive,
	} {
		if slices.Contains(member, permission) {
			t.Errorf("member role must not grant %q", permission)
		}
	}

	viewer := PermissionsForRole(RoleViewer)

	if !slices.Contains(viewer, PermissionModelProviderRead) {
		t.Error("viewer role must grant model provider read")
	}

	if slices.Contains(viewer, PermissionModelConnectionRead) {
		t.Error("viewer role must not grant model connection read")
	}

	for _, permission := range []Permission{
		PermissionModelConnectionCreate,
		PermissionModelConnectionUpdate,
		PermissionModelConnectionArchive,
	} {
		if !IsMutationPermission(permission) {
			t.Errorf("%q must be a mutation permission", permission)
		}
	}

	if IsMutationPermission(PermissionModelProviderRead) {
		t.Error("model provider read must not be a mutation permission")
	}

	if IsMutationPermission(PermissionModelConnectionRead) {
		t.Error("model connection read must not be a mutation permission")
	}

	for _, permission := range []Permission{
		PermissionModelProviderRead,
		PermissionModelConnectionRead,
		PermissionModelConnectionCreate,
		PermissionModelConnectionUpdate,
		PermissionModelConnectionArchive,
	} {
		if !IsKnownPermission(permission) {
			t.Errorf("%q must be a known permission", permission)
		}
	}
}

func TestModelAccessMutationsBlockedForNonActiveTenants(t *testing.T) {
	t.Parallel()

	provisioningTenant := Tenant{
		ID:      "0198abc0-0000-7000-8000-000000000011",
		Status:  StatusProvisioning,
		Version: 1,
	}

	owner := Membership{
		TenantID: provisioningTenant.ID,
		UserID:   "0198abc0-0000-7000-8000-000000000012",
		Role:     RoleOwner,
		Version:  1,
	}

	for _, permission := range []Permission{
		PermissionModelConnectionCreate,
		PermissionModelConnectionUpdate,
		PermissionModelConnectionArchive,
	} {
		decision := EvaluateAuthorization(
			provisioningTenant,
			owner,
			permission,
		)

		if decision.Allowed {
			t.Errorf(
				"mutation %q must be denied for provisioning tenant",
				permission,
			)
		}

		if decision.DenialReason != DenialReasonTenantNotActive {
			t.Errorf(
				"mutation %q denial reason = %q, want %q",
				permission,
				decision.DenialReason,
				DenialReasonTenantNotActive,
			)
		}
	}

	decision := EvaluateAuthorization(
		provisioningTenant,
		owner,
		PermissionModelProviderRead,
	)

	if !decision.Allowed {
		t.Error(
			"provider read must remain allowed for provisioning tenant",
		)
	}
}
