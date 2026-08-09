// Package organization adapts the Organization Service agent policy context
// for the Policy Approval Service.
package organization

import (
	"context"
	"fmt"
	"time"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client resolves trusted agent context through the Organization Service.
type Client struct {
	client  organizationv1.OrganizationPolicyContextServiceClient
	timeout time.Duration
}

// NewClient creates an Organization Service-backed agent context client.
func NewClient(
	client organizationv1.OrganizationPolicyContextServiceClient,
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

// GetAgentPolicyContext resolves agent context under service identity so the
// Policy Service never trusts caller-supplied agent context.
func (client *Client) GetAgentPolicyContext(
	ctx context.Context,
	tenantID string,
	agentID string,
) (domain.Subject, error) {
	callContext, cancel := context.WithTimeout(
		ctx,
		client.timeout,
	)
	defer cancel()

	response, err := client.client.GetAgentPolicyContext(
		callContext,
		&organizationv1.GetAgentPolicyContextRequest{
			TenantId: tenantID,
			AgentId:  agentID,
		},
	)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return domain.Subject{}, domain.ErrNotFound

		case codes.PermissionDenied,
			codes.Unauthenticated:
			return domain.Subject{}, domain.ErrForbidden

		case codes.DeadlineExceeded,
			codes.Unavailable:
			return domain.Subject{}, fmt.Errorf(
				"organization service unavailable: %w",
				err,
			)

		default:
			return domain.Subject{}, fmt.Errorf(
				"resolve agent policy context: %w",
				err,
			)
		}
	}

	contextValue := response.GetContext()
	if contextValue == nil {
		return domain.Subject{}, fmt.Errorf(
			"organization service returned no agent policy context",
		)
	}

	companyID := contextValue.GetCompanyId()

	return domain.Subject{
		Type:              domain.SubjectAgent,
		ID:                contextValue.GetAgentId(),
		CompanyID:         &companyID,
		AgentAutonomy:     autonomyLevel(contextValue.GetAutonomyLevel()),
		AgentStatus:       agentStatus(contextValue.GetStatus()),
		AgentCapabilities: contextValue.GetCapabilities(),
		AgentVersion:      contextValue.GetVersion(),
	}, nil
}

func autonomyLevel(
	level organizationv1.AgentAutonomyLevel,
) string {
	switch level {
	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_OBSERVE_ONLY:
		return "observe_only"

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_SUGGEST:
		return "suggest"

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_APPROVAL_REQUIRED:
		return "approval_required"

	case organizationv1.AgentAutonomyLevel_AGENT_AUTONOMY_LEVEL_POLICY_BOUNDED:
		return "policy_bounded"

	default:
		return "unknown"
	}
}

func agentStatus(
	value organizationv1.AgentStatus,
) string {
	switch value {
	case organizationv1.AgentStatus_AGENT_STATUS_DRAFT:
		return "draft"

	case organizationv1.AgentStatus_AGENT_STATUS_PROVISIONING:
		return "provisioning"

	case organizationv1.AgentStatus_AGENT_STATUS_CONFIGURING_RUNTIME:
		return "configuring_runtime"

	case organizationv1.AgentStatus_AGENT_STATUS_HEALTH_CHECKING:
		return "health_checking"

	case organizationv1.AgentStatus_AGENT_STATUS_READY:
		return "ready"

	case organizationv1.AgentStatus_AGENT_STATUS_DEGRADED:
		return "degraded"

	case organizationv1.AgentStatus_AGENT_STATUS_PAUSED:
		return "paused"

	case organizationv1.AgentStatus_AGENT_STATUS_FAILED:
		return "failed"

	case organizationv1.AgentStatus_AGENT_STATUS_DELETING:
		return "deleting"

	case organizationv1.AgentStatus_AGENT_STATUS_DELETED:
		return "deleted"

	default:
		return "unknown"
	}
}

// Ensure the client satisfies the PolicyContextClient port.
var _ ports.PolicyContextClient = (*Client)(nil)
