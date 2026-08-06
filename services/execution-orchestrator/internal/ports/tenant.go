package ports

import (
	"context"

	"github.com/aminio9/gereh/services/execution-orchestrator/internal/domain"
)

// MarkRunningRequest records the workflow start in the Tenant Service.
type MarkRunningRequest struct {
	TenantID      string
	OperationID   string
	WorkflowID    string
	WorkflowRunID string
}

// TenantOnboardingClient is the Tenant Service internal onboarding API.
type TenantOnboardingClient interface {
	MarkRunning(
		ctx context.Context,
		request MarkRunningRequest,
	) error

	Complete(
		ctx context.Context,
		tenantID string,
		operationID string,
	) error

	Fail(
		ctx context.Context,
		tenantID string,
		operationID string,
		failure domain.OperationFailure,
	) error
}
