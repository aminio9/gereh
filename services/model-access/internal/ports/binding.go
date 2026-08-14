package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

// SetBindingParams contains the parameters for creating or updating an agent model binding.
type SetBindingParams struct {
	ActorUserID string
	TenantID    string

	AgentID   string
	CompanyID string

	ExpectedVersion int64

	PrimaryOfferingID   string
	FastOfferingID      *string
	FallbackOfferingIDs []string
	FallbackPolicy      domain.FallbackPolicy

	MaxModelCostMicroUSD *int64

	IdempotencyKey       string
	RequestHash          string
	IdempotencyExpiresAt time.Time

	Now time.Time

	EventFactory func(domain.AgentModelBinding) (domain.OutboxEvent, error)
}

// RemoveBindingParams contains the parameters for removing an agent model binding.
type RemoveBindingParams struct {
	ActorUserID string
	TenantID    string

	AgentID string

	ExpectedVersion int64

	IdempotencyKey       string
	RequestHash          string
	IdempotencyExpiresAt time.Time

	Now time.Time

	EventFactory func(domain.AgentModelBinding) (domain.OutboxEvent, error)
}

// BindingRepository specifies the persistence operations for agent model bindings.
type BindingRepository interface {
	GetAgentBinding(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		agentID string,
	) (domain.AgentModelBinding, error)

	SetAgentBinding(
		ctx context.Context,
		params SetBindingParams,
	) (domain.AgentModelBinding, error)

	RemoveAgentBinding(
		ctx context.Context,
		params RemoveBindingParams,
	) (domain.AgentModelBinding, error)
}
