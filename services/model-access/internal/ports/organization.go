package ports

import (
	"context"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
)

// AgentReference represents the minimal agent metadata required by model access.
type AgentReference struct {
	TenantID string

	CompanyID string
	AgentID   string

	Status organizationv1.AgentStatus

	Version int64
}

// AgentDirectory describes the capability to query agent records from the Organization service.
type AgentDirectory interface {
	GetAgent(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		agentID string,
	) (AgentReference, error)
}
