package domain

import (
	"slices"
	"testing"
)

func TestPermissionsForRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		role        Role
		permissions []Permission
	}{
		{
			name: "owner receives every permission",
			role: RoleOwner,
			permissions: []Permission{
				PermissionTenantRead,
				PermissionTenantUpdate,
				PermissionTenantArchive,
				PermissionMemberList,
				PermissionMemberAdd,
				PermissionMemberUpdateRole,
				PermissionMemberRemove,
				PermissionEntitlementRead,
			},
		},
		{
			name: "admin cannot archive tenant",
			role: RoleAdmin,
			permissions: []Permission{
				PermissionTenantRead,
				PermissionTenantUpdate,
				PermissionMemberList,
				PermissionMemberAdd,
				PermissionMemberUpdateRole,
				PermissionMemberRemove,
				PermissionEntitlementRead,
			},
		},
		{
			name: "member has read and list permissions",
			role: RoleMember,
			permissions: []Permission{
				PermissionTenantRead,
				PermissionMemberList,
				PermissionEntitlementRead,
			},
		},
		{
			name: "viewer has minimal read access",
			role: RoleViewer,
			permissions: []Permission{
				PermissionTenantRead,
				PermissionEntitlementRead,
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual := PermissionsForRole(
					test.role,
				)

				if !slices.Equal(
					actual,
					test.permissions,
				) {
					t.Fatalf(
						"PermissionsForRole(%q) = %v, want %v",
						test.role,
						actual,
						test.permissions,
					)
				}
			},
		)
	}
}

func TestEvaluateAuthorization(t *testing.T) {
	t.Parallel()

	activeTenant := Tenant{
		ID:      "0198abc0-0000-7000-8000-000000000001",
		Status:  StatusActive,
		Version: 7,
	}

	archivedTenant := activeTenant
	archivedTenant.Status = StatusArchived

	admin := Membership{
		TenantID: activeTenant.ID,
		UserID:   "0198abc0-0000-7000-8000-000000000002",
		Role:     RoleAdmin,
		Version:  3,
	}

	tests := []struct {
		name       string
		tenant     Tenant
		membership Membership
		permission Permission
		allowed    bool
		reason     DenialReason
	}{
		{
			name:       "admin may update active tenant",
			tenant:     activeTenant,
			membership: admin,
			permission: PermissionTenantUpdate,
			allowed:    true,
			reason:     DenialReasonNone,
		},
		{
			name:       "admin cannot archive tenant",
			tenant:     activeTenant,
			membership: admin,
			permission: PermissionTenantArchive,
			allowed:    false,
			reason:     DenialReasonPermissionNotGranted,
		},
		{
			name:       "archived tenant remains readable",
			tenant:     archivedTenant,
			membership: admin,
			permission: PermissionTenantRead,
			allowed:    true,
			reason:     DenialReasonNone,
		},
		{
			name:       "archived tenant rejects mutation",
			tenant:     archivedTenant,
			membership: admin,
			permission: PermissionTenantUpdate,
			allowed:    false,
			reason:     DenialReasonTenantArchived,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				decision := EvaluateAuthorization(
					test.tenant,
					test.membership,
					test.permission,
				)

				if decision.Allowed != test.allowed {
					t.Fatalf(
						"Allowed = %v, want %v",
						decision.Allowed,
						test.allowed,
					)
				}

				if decision.DenialReason !=
					test.reason {
					t.Fatalf(
						"DenialReason = %q, want %q",
						decision.DenialReason,
						test.reason,
					)
				}
			},
		)
	}
}

func TestUnknownPermissionDenied(t *testing.T) {
	t.Parallel()

	if RoleAllows(
		RoleOwner,
		Permission("unknown.permission"),
	) {
		t.Fatal(
			"unknown permission was unexpectedly allowed",
		)
	}
}
