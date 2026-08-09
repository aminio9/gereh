package ports

import (
	"context"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
)

// AgentPolicyContext resolves trusted agent context for evaluation.
type AgentPolicyContext struct {
	TenantID      string
	CompanyID     string
	AgentID       string
	Status        string
	AutonomyLevel string
	Capabilities  []string
	Version       int64
}

// PolicyContextClient resolves trusted agent context from the Organization
// Service.
type PolicyContextClient interface {
	GetAgentPolicyContext(
		ctx context.Context,
		tenantID string,
		agentID string,
	) (domain.Subject, error)
}
