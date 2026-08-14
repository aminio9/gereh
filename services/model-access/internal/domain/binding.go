package domain

import "time"

// BindingStatus represents the lifecycle state of an agent model binding.
type BindingStatus string

const (
	// BindingStatusActive indicates the binding is currently active.
	BindingStatusActive BindingStatus = "active"
	// BindingStatusRemoved indicates the binding has been removed.
	BindingStatusRemoved BindingStatus = "removed"
)

// FallbackPolicy represents the policy for evaluating model fallbacks.
type FallbackPolicy string

const (
	// FallbackPolicyNone indicates no fallback is configured.
	FallbackPolicyNone FallbackPolicy = "none"
	// FallbackPolicyOrdered indicates fallbacks should be attempted in ordered sequence.
	FallbackPolicyOrdered FallbackPolicy = "ordered"
)

// AgentModelBinding represents the assignment of model offerings to an agent.
type AgentModelBinding struct {
	TenantID string

	AgentID   string
	CompanyID string

	Status BindingStatus

	PrimaryOfferingID string

	FastOfferingID *string

	FallbackOfferingIDs []string

	FallbackPolicy FallbackPolicy

	MaxModelCostMicroUSD *int64

	Version int64

	CreatedByUserID string
	UpdatedByUserID string

	CreatedAt time.Time
	UpdatedAt time.Time

	RemovedAt *time.Time
}
