package protoutil

import (
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
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
	return &tenantv1.TenantContext{
		Tenant:       Tenant(value.Tenant),
		Membership:   Membership(value.Membership),
		Entitlements: Entitlements(value.Entitlements),
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
	case domain.StatusActive:
		return tenantv1.TenantStatus_TENANT_STATUS_ACTIVE
	case domain.StatusArchived:
		return tenantv1.TenantStatus_TENANT_STATUS_ARCHIVED
	default:
		return tenantv1.TenantStatus_TENANT_STATUS_UNSPECIFIED
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
