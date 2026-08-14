// Package organization implements an adapter to query agent information from the Organization service.
package organization

import (
	"context"
	"errors"
	"fmt"
	"time"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client adapts OrganizationServiceClient to ports.AgentDirectory.
type Client struct {
	client  organizationv1.OrganizationServiceClient
	timeout time.Duration
}

// NewClient constructs an Organization service client adapter.
func NewClient(
	client organizationv1.OrganizationServiceClient,
	timeout time.Duration,
) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &Client{
		client:  client,
		timeout: timeout,
	}
}

// GetAgent fetches agent metadata from the Organization service.
func (c *Client) GetAgent(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	agentID string,
) (ports.AgentReference, error) {
	callContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	response, err := c.client.GetAgent(
		callContext,
		&organizationv1.GetAgentRequest{
			ActorUserId: actorUserID,
			TenantId:    tenantID,
			AgentId:     agentID,
		},
	)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ports.AgentReference{}, domain.ErrAgentNotFound
		}
		if status.Code(err) == codes.PermissionDenied {
			return ports.AgentReference{}, domain.ErrForbidden
		}

		return ports.AgentReference{}, fmt.Errorf("get agent from organization service: %w", err)
	}

	agent := response.GetAgent()
	if agent == nil {
		return ports.AgentReference{}, errors.New("organization service returned empty agent")
	}

	return ports.AgentReference{
		TenantID:  agent.GetTenantId(),
		CompanyID: agent.GetCompanyId(),
		AgentID:   agent.GetAgentId(),
		Status:    agent.GetStatus(),
		Version:   agent.GetVersion(),
	}, nil
}
