// Package organization contains the Organization Service gRPC adapter.
package organization

import (
	"context"
	"fmt"
	"strings"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Client implements ports.OrganizationBootstrapClient over gRPC.
type Client struct {
	client organizationv1.OrganizationBootstrapServiceClient
	token  string
}

// New creates the Organization Service bootstrap client.
//
// The token is a local-development fallback. In production the connection
// must use workload mTLS with a SPIFFE identity.
func New(
	connection grpc.ClientConnInterface,
	developmentToken string,
) *Client {
	return &Client{
		client: organizationv1.NewOrganizationBootstrapServiceClient(
			connection,
		),
		token: strings.TrimSpace(
			developmentToken,
		),
	}
}

// EnsureDefaultCompany idempotently creates a tenant's default company.
func (client *Client) EnsureDefaultCompany(
	ctx context.Context,
	request ports.EnsureDefaultCompanyRequest,
) error {
	ctx = client.authorizedContext(ctx)

	_, err := client.client.EnsureDefaultCompany(
		ctx,
		&organizationv1.EnsureDefaultCompanyRequest{
			TenantId:              request.TenantID,
			OnboardingOperationId: request.OnboardingOperationID,
			ActorUserId:           request.ActorUserID,
			TenantDisplayName:     request.TenantDisplayName,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"ensure default company: %w",
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
