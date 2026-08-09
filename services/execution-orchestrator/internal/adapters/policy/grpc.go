// Package policy contains the Policy Service gRPC adapter.
package policy

import (
	"context"
	"fmt"
	"strings"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Client implements ports.PolicyBootstrapClient over gRPC.
type Client struct {
	client policyv1.PolicyBootstrapServiceClient
	token  string
}

// New creates the Policy Service bootstrap client.
//
// The token is a local-development fallback. In production the connection
// must use workload mTLS with a SPIFFE identity.
func New(
	connection grpc.ClientConnInterface,
	developmentToken string,
) *Client {
	return &Client{
		client: policyv1.NewPolicyBootstrapServiceClient(
			connection,
		),
		token: strings.TrimSpace(
			developmentToken,
		),
	}
}

// EnsureDefaultPolicies idempotently creates a tenant's default policies.
func (client *Client) EnsureDefaultPolicies(
	ctx context.Context,
	request ports.EnsureDefaultPoliciesRequest,
) error {
	ctx = client.authorizedContext(ctx)

	_, err := client.client.EnsureDefaultPolicies(
		ctx,
		&policyv1.EnsureDefaultPoliciesRequest{
			TenantId:              request.TenantID,
			OnboardingOperationId: request.OnboardingOperationID,
			ActorUserId:           request.ActorUserID,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"ensure default policies: %w",
			err,
		)
	}

	return nil
}

func (client *Client) authorizedContext(
	ctx context.Context,
) context.Context {
	if client.token == "" {
		return ctx
	}

	return metadata.AppendToOutgoingContext(
		ctx,
		"authorization",
		"Bearer "+client.token,
	)
}
