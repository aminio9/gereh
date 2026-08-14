package domain

import (
	"time"
)

// InferenceRoute contains routing details for a single model offering.
type InferenceRoute struct {
	OfferingID          string
	ConnectionID        string
	ProviderKey         string
	ProviderModelID     string
	ConnectionType      string
	ProviderPoolKey     *string
	ContextWindowTokens int64
	MaxOutputTokens     int64
	Capabilities        []string
}

// InferencePlan represents the complete resolved routing plan for an agent.
type InferencePlan struct {
	TenantID             string
	AgentID              string
	CompanyID            string
	BindingVersion       int64
	PrimaryRoute         InferenceRoute
	FastRoute            *InferenceRoute
	FallbackRoutes       []InferenceRoute
	MaxModelCostMicroUSD *int64
	ResolvedAt           time.Time
}
