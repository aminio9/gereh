package application

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
)

// GetAgentPolicyContextInput identifies the agent to resolve for the Policy
// Service.
type GetAgentPolicyContextInput struct {
	TenantID string
	AgentID  string
}

// GetAgentPolicyContext resolves trusted agent context under the organization
// service principal so the Policy Service never trusts caller-supplied agent
// context.
func (service *Service) GetAgentPolicyContext(
	ctx context.Context,
	input GetAgentPolicyContextInput,
) (domain.Agent, error) {
	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := validateUUID(
		"agent_id",
		input.AgentID,
	); err != nil {
		return domain.Agent{}, err
	}

	agent, err := service.repository.GetAgentAsService(
		ctx,
		input.TenantID,
		service.config.BootstrapServicePrincipalID,
		input.AgentID,
	)
	if err != nil {
		return domain.Agent{}, fmt.Errorf(
			"get agent policy context: %w",
			err,
		)
	}

	return agent, nil
}
