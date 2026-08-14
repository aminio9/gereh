package domain

import "time"

type BindingStatus string

const (
	BindingStatusActive  BindingStatus = "active"
	BindingStatusRemoved BindingStatus = "removed"
)

type FallbackPolicy string

const (
	FallbackPolicyNone    FallbackPolicy = "none"
	FallbackPolicyOrdered FallbackPolicy = "ordered"
)

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
