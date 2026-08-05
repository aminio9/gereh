package domain

import "time"

// Status is the lifecycle state of a tenant.
type Status string

// Status values.
const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

// Role is a tenant-scoped membership role.
type Role string

// Role values.
const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Tenant is the tenant aggregate root.
type Tenant struct {
	ID              string
	Slug            string
	DisplayName     string
	Status          Status
	Region          string
	RetentionDays   int32
	Version         int64
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

// Membership connects an identity-owned user ID to a tenant.
type Membership struct {
	TenantID  string
	UserID    string
	Role      Role
	Version   int64
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Entitlements are tenant-level plan features and limits.
type Entitlements struct {
	TenantID  string
	PlanKey   string
	Features  map[string]bool
	Limits    map[string]int64
	Version   int64
	UpdatedAt time.Time
}

// TenantContext is the trusted tenant authorization context for one user.
type TenantContext struct {
	Tenant       Tenant
	Membership   Membership
	Entitlements Entitlements
	Permissions  []Permission
}

// OutboxEvent is an event committed atomically with a domain mutation.
type OutboxEvent struct {
	ID         string
	Topic      string
	Key        string
	Envelope   []byte
	OccurredAt time.Time
}

// OutboxRecord is an unpublished database outbox row.
type OutboxRecord struct {
	OutboxID int64
	Event    OutboxEvent
	Attempts int
}
