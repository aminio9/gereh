package ports

import (
	"context"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
)

type AgentReference struct {
	TenantID string

	CompanyID string
	AgentID   string

	Status organizationv1.AgentStatus

	Version int64
}

type AgentDirectory interface {
	GetAgent(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		agentID string,
	) (AgentReference, error)
}
