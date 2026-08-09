// Package ports defines the policy-approval service boundaries.
package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
)

// PolicyCursor paginates policy lists.
type PolicyCursor struct {
	PolicyID string
}

// DecisionCursor paginates decision lists.
type DecisionCursor struct {
	DecidedAt  time.Time
	DecisionID string
}

// CreatePolicyParams carries a new policy set and its outbox event.
type CreatePolicyParams struct {
	ActorUserID string
	Policy      domain.Policy
	Event       domain.OutboxEvent
}

// CreateVersionParams carries a new immutable policy version.
type CreateVersionParams struct {
	ActorUserID             string
	Policy                  domain.Policy
	Version                 domain.PolicyVersion
	ExpectedResourceVersion int64
	Event                   domain.OutboxEvent
}

// ActivatePolicyParams activates an existing policy version.
type ActivatePolicyParams struct {
	ActorUserID             string
	TenantID                string
	PolicyID                string
	PolicyVersion           int64
	ExpectedResourceVersion int64
	ActivatedAt             time.Time
	Event                   domain.OutboxEvent
}

// ArchivePolicyParams archives a policy set.
type ArchivePolicyParams struct {
	ActorUserID             string
	TenantID                string
	PolicyID                string
	ExpectedResourceVersion int64
	ArchivedAt              time.Time
	Event                   domain.OutboxEvent
}

// RecordDecisionParams carries a signed decision and its outbox event.
type RecordDecisionParams struct {
	ServicePrincipalID string
	Decision           domain.Decision
	Event              domain.OutboxEvent
}

// EnsureDefaultsParams carries the default-policy bootstrap.
type EnsureDefaultsParams struct {
	ServicePrincipalID    string
	TenantID              string
	OnboardingOperationID string
	ActorUserID           string
	Policies              []domain.Policy
	Versions              []domain.PolicyVersion
	Events                []domain.OutboxEvent
	CreatedAt             time.Time
}

// Repository persists policy state and decisions.
type Repository interface {
	CreatePolicy(
		context.Context,
		CreatePolicyParams,
	) (domain.Policy, error)

	GetPolicy(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		policyID string,
	) (domain.Policy, *domain.PolicyVersion, error)

	ListPolicies(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		limit int,
		cursor *PolicyCursor,
		includeArchived bool,
	) ([]domain.Policy, error)

	CreateVersion(
		context.Context,
		CreateVersionParams,
	) (domain.Policy, domain.PolicyVersion, error)

	ActivatePolicy(
		context.Context,
		ActivatePolicyParams,
	) (domain.Policy, domain.PolicyVersion, error)

	ArchivePolicy(
		context.Context,
		ArchivePolicyParams,
	) (domain.Policy, error)

	ListActiveBundles(
		ctx context.Context,
		servicePrincipalID string,
		tenantID string,
		companyID *string,
		agentID *string,
	) ([]domain.ActiveBundle, error)

	FindDecisionByRequestID(
		ctx context.Context,
		servicePrincipalID string,
		tenantID string,
		requestID string,
	) (domain.Decision, error)

	RecordDecision(
		context.Context,
		RecordDecisionParams,
	) (domain.Decision, error)

	GetDecision(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		decisionID string,
	) (domain.Decision, error)

	ListDecisions(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		subjectID *string,
		limit int,
		cursor *DecisionCursor,
	) ([]domain.Decision, error)

	EnsureDefaults(
		context.Context,
		EnsureDefaultsParams,
	) ([]domain.Policy, error)

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
