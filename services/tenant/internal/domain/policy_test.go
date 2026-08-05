package domain

import "testing"

func TestCanManageMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		actor    Role
		target   Role
		newRole  Role
		expected bool
	}{
		{
			name:     "owner may assign owner",
			actor:    RoleOwner,
			target:   RoleAdmin,
			newRole:  RoleOwner,
			expected: true,
		},
		{
			name:     "admin cannot assign owner",
			actor:    RoleAdmin,
			target:   RoleMember,
			newRole:  RoleOwner,
			expected: false,
		},
		{
			name:     "admin cannot modify owner",
			actor:    RoleAdmin,
			target:   RoleOwner,
			newRole:  RoleAdmin,
			expected: false,
		},
		{
			name:     "member cannot manage memberships",
			actor:    RoleMember,
			target:   RoleViewer,
			newRole:  RoleMember,
			expected: false,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual := CanManageMember(
					test.actor,
					test.target,
					test.newRole,
				)

				if actual != test.expected {
					t.Fatalf(
						"CanManageMember() = %v, want %v",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}
