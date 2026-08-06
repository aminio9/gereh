package grpc

import (
	"context"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/tenant/internal/application"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/protoutil"
)

// OnboardingServer implements the trusted workload-only
// TenantOnboardingService.
type OnboardingServer struct {
	tenantv1.UnimplementedTenantOnboardingServiceServer

	service *application.Service
}

// NewOnboarding creates the internal onboarding gRPC transport.
func NewOnboarding(
	service *application.Service,
) *OnboardingServer {
	return &OnboardingServer{
		service: service,
	}
}

// MarkOnboardingRunning records that the provisioning workflow started.
func (server *OnboardingServer) MarkOnboardingRunning(
	ctx context.Context,
	request *tenantv1.MarkOnboardingRunningRequest,
) (*tenantv1.MarkOnboardingRunningResponse, error) {
	result, err := server.service.MarkOnboardingRunning(
		ctx,
		application.MarkOnboardingRunningInput{
			TenantID:      request.GetTenantId(),
			OperationID:   request.GetOperationId(),
			WorkflowID:    request.GetWorkflowId(),
			WorkflowRunID: request.GetWorkflowRunId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.MarkOnboardingRunningResponse{
		Operation: protoutil.Operation(result),
	}, nil
}

// CompleteOnboarding activates a tenant and succeeds its operation.
func (server *OnboardingServer) CompleteOnboarding(
	ctx context.Context,
	request *tenantv1.CompleteOnboardingRequest,
) (*tenantv1.CompleteOnboardingResponse, error) {
	result, err := server.service.CompleteOnboarding(
		ctx,
		application.CompleteOnboardingInput{
			TenantID:    request.GetTenantId(),
			OperationID: request.GetOperationId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.CompleteOnboardingResponse{
		Context:   protoutil.Context(result.Context),
		Operation: protoutil.Operation(result.Operation),
	}, nil
}

// FailOnboarding records a terminal provisioning failure.
func (server *OnboardingServer) FailOnboarding(
	ctx context.Context,
	request *tenantv1.FailOnboardingRequest,
) (*tenantv1.FailOnboardingResponse, error) {
	inputError := request.GetError()

	if inputError == nil {
		inputError = &commonv1.OperationError{
			Code:    "onboarding_failed",
			Message: "Tenant provisioning could not be completed",
		}
	}

	operation, tenant, err :=
		server.service.FailOnboarding(
			ctx,
			application.FailOnboardingInput{
				TenantID:    request.GetTenantId(),
				OperationID: request.GetOperationId(),
				Error: domain.OperationError{
					Code:      inputError.GetCode(),
					Message:   inputError.GetMessage(),
					Retryable: inputError.GetRetryable(),
					Details:   inputError.GetDetails().AsMap(),
				},
			},
		)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.FailOnboardingResponse{
		Tenant:    protoutil.Tenant(tenant),
		Operation: protoutil.Operation(operation),
	}, nil
}
