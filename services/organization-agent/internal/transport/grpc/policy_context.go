package grpc

import (
	"context"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/organization-agent/internal/application"
	"github.com/aminio9/gereh/services/organization-agent/internal/protoutil"
)

// PolicyContextServer implements OrganizationPolicyContextService.
//
// This service is workload-only and is protected by the internal workload
// interceptor.
type PolicyContextServer struct {
	organizationv1.UnimplementedOrganizationPolicyContextServiceServer

	service *application.Service
}

// NewPolicyContext creates the policy-context gRPC transport.
func NewPolicyContext(
	service *application.Service,
) *PolicyContextServer {
	return &PolicyContextServer{
		service: service,
	}
}

// GetAgentPolicyContext resolves trusted agent context for the Policy Service.
func (server *PolicyContextServer) GetAgentPolicyContext(
	ctx context.Context,
	request *organizationv1.GetAgentPolicyContextRequest,
) (*organizationv1.GetAgentPolicyContextResponse, error) {
	agent, err := server.service.GetAgentPolicyContext(
		ctx,
		application.GetAgentPolicyContextInput{
			TenantID: request.GetTenantId(),
			AgentID:  request.GetAgentId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.GetAgentPolicyContextResponse{
		Context: &organizationv1.AgentPolicyContext{
			TenantId:      agent.TenantID,
			CompanyId:     agent.CompanyID,
			AgentId:       agent.ID,
			Status:        protoutil.AgentStatus(agent.Status),
			AutonomyLevel: protoutil.AutonomyLevel(agent.AutonomyLevel),
			Capabilities:  agent.Capabilities,
			Version:       agent.Version,
		},
	}, nil
}
