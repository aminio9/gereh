package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
)

// TenantCursor provides keyset pagination for UUIDv7 tenant IDs.
type TenantCursor struct {
	TenantID string
}

// MemberCursor provides keyset pagination for user IDs.
type MemberCursor struct {
	UserID string
}

// CreateTenantParams defines an atomic tenant creation.
type CreateTenantParams struct {
	Context   domain.TenantContext
	Operation domain.Operation
	RequestID string
	Event     domain.OutboxEvent
}

// MarkOnboardingRunningParams defines the workflow-start transition.
type MarkOnboardingRunningParams struct {
	ServicePrincipalID string
	TenantID           string
	OperationID        string
	WorkflowID         string
	WorkflowRunID      string
	StartedAt          time.Time
	Event              domain.OutboxEvent
}

// CompleteOnboardingParams defines the activation transition. Event is
// built inside the transaction with the activated context so the outbox
// write stays atomic with the state change.
type CompleteOnboardingParams struct {
	ServicePrincipalID string
	TenantID           string
	OperationID        string
	CompletedAt        time.Time
	Event              func(domain.TenantContext) (domain.OutboxEvent, error)
}

// FailOnboardingParams defines the terminal failure transition. Event is
// built inside the transaction with the failed tenant and terminal
// operation.
type FailOnboardingParams struct {
	ServicePrincipalID string
	TenantID           string
	OperationID        string
	FailedAt           time.Time
	Error              domain.OperationError
	Event              func(domain.Tenant, domain.Operation) (domain.OutboxEvent, error)
}

// UpdateTenantParams defines an atomic tenant update.
type UpdateTenantParams struct {
	ActorUserID     string
	Tenant          domain.Tenant
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// ArchiveTenantParams defines an atomic tenant archive.
type ArchiveTenantParams struct {
	ActorUserID     string
	Tenant          domain.Tenant
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// AddMemberParams defines an atomic membership creation.
type AddMemberParams struct {
	ActorUserID           string
	Membership            domain.Membership
	ExpectedTenantVersion int64
	NewTenantVersion      int64
	Event                 domain.OutboxEvent
}

// UpdateMemberRoleParams defines an atomic membership role change.
type UpdateMemberRoleParams struct {
	ActorUserID               string
	Membership                domain.Membership
	PreviousRole              domain.Role
	ExpectedMembershipVersion int64
	ExpectedTenantVersion     int64
	NewTenantVersion          int64
	Event                     domain.OutboxEvent
}

// RemoveMemberParams defines an atomic membership deletion.
type RemoveMemberParams struct {
	ActorUserID               string
	TenantID                  string
	UserID                    string
	PreviousRole              domain.Role
	ExpectedMembershipVersion int64
	ExpectedTenantVersion     int64
	NewTenantVersion          int64
	Event                     domain.OutboxEvent
}

// Repository is the authoritative tenant persistence boundary.
type Repository interface {
	CreateTenant(
		ctx context.Context,
		params CreateTenantParams,
	) (domain.CreateTenantResult, error)

	GetOperation(
		ctx context.Context,
		actorUserID string,
		operationID string,
	) (domain.Operation, error)

	MarkOnboardingRunning(
		ctx context.Context,
		params MarkOnboardingRunningParams,
	) (domain.Operation, error)

	CompleteOnboarding(
		ctx context.Context,
		params CompleteOnboardingParams,
	) (domain.CreateTenantResult, error)

	FailOnboarding(
		ctx context.Context,
		params FailOnboardingParams,
	) (domain.Operation, domain.Tenant, error)

	GetTenantContext(
		ctx context.Context,
		tenantID string,
		userID string,
	) (domain.TenantContext, error)

	ListTenantContexts(
		ctx context.Context,
		userID string,
		limit int,
		cursor *TenantCursor,
	) ([]domain.TenantContext, error)

	UpdateTenant(
		ctx context.Context,
		params UpdateTenantParams,
	) (domain.TenantContext, error)

	ArchiveTenant(
		ctx context.Context,
		params ArchiveTenantParams,
	) (domain.TenantContext, error)

	GetMembership(
		ctx context.Context,
		tenantID string,
		userID string,
	) (domain.Membership, error)

	ListMembers(
		ctx context.Context,
		tenantID string,
		actorUserID string,
		limit int,
		cursor *MemberCursor,
	) ([]domain.Membership, error)

	AddMember(
		ctx context.Context,
		params AddMemberParams,
	) (domain.Membership, error)

	UpdateMemberRole(
		ctx context.Context,
		params UpdateMemberRoleParams,
	) (domain.Membership, error)

	RemoveMember(
		ctx context.Context,
		params RemoveMemberParams,
	) error

	ClaimOutbox(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.OutboxRecord, error)

	MarkOutboxPublished(
		ctx context.Context,
		outboxID int64,
	) error

	ReleaseOutbox(
		ctx context.Context,
		outboxID int64,
		retryAt time.Time,
		publishError string,
	) error
}
