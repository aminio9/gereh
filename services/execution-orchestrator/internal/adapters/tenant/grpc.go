// Package tenant contains the Tenant Service gRPC adapter.
package tenant

import (
	"context"
	"fmt"
	"strings"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/domain"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

// Client implements ports.TenantOnboardingClient over gRPC.
type Client struct {
	client tenantv1.TenantOnboardingServiceClient
	token  string
}

// New creates the Tenant Service onboarding client.
//
// The token is a local-development fallback. In production the connection
// must use workload mTLS with a SPIFFE identity.
func New(
	connection grpc.ClientConnInterface,
	developmentToken string,
) *Client {
	return &Client{
		client: tenantv1.NewTenantOnboardingServiceClient(
			connection,
		),
		token: strings.TrimSpace(
			developmentToken,
		),
	}
}

// MarkRunning records the workflow start.
func (client *Client) MarkRunning(
	ctx context.Context,
	request ports.MarkRunningRequest,
) error {
	ctx = client.authorizedContext(ctx)

	_, err := client.client.MarkOnboardingRunning(
		ctx,
		&tenantv1.MarkOnboardingRunningRequest{
			TenantId:      request.TenantID,
			OperationId:   request.OperationID,
			WorkflowId:    request.WorkflowID,
			WorkflowRunId: request.WorkflowRunID,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"mark tenant onboarding running: %w",
			err,
		)
	}

	return nil
}

// Complete activates the tenant.
func (client *Client) Complete(
	ctx context.Context,
	tenantID string,
	operationID string,
) error {
	ctx = client.authorizedContext(ctx)

	_, err := client.client.CompleteOnboarding(
		ctx,
		&tenantv1.CompleteOnboardingRequest{
			TenantId:    tenantID,
			OperationId: operationID,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"complete tenant onboarding: %w",
			err,
		)
	}

	return nil
}

// Fail records a terminal provisioning failure.
func (client *Client) Fail(
	ctx context.Context,
	tenantID string,
	operationID string,
	failure domain.OperationFailure,
) error {
	ctx = client.authorizedContext(ctx)

	details, err := structpb.NewStruct(
		failure.Details,
	)
	if err != nil {
		return fmt.Errorf(
			"encode onboarding failure details: %w",
			err,
		)
	}

	_, err = client.client.FailOnboarding(
		ctx,
		&tenantv1.FailOnboardingRequest{
			TenantId:    tenantID,
			OperationId: operationID,
			Error: &commonv1.OperationError{
				Code:      failure.Code,
				Message:   failure.Message,
				Retryable: failure.Retryable,
				Details:   details,
			},
		},
	)
	if err != nil {
		return fmt.Errorf(
			"fail tenant onboarding: %w",
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
