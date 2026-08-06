package application

import (
	"context"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/ports"
	"github.com/aminio9/gereh/services/tenant/internal/protoutil"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MarkOnboardingRunningInput defines the workflow-start transition.
type MarkOnboardingRunningInput struct {
	TenantID      string
	OperationID   string
	WorkflowID    string
	WorkflowRunID string
}

// CompleteOnboardingInput defines the activation transition.
type CompleteOnboardingInput struct {
	TenantID    string
	OperationID string
}

// FailOnboardingInput defines the terminal failure transition.
type FailOnboardingInput struct {
	TenantID    string
	OperationID string
	Error       domain.OperationError
}

// MarkOnboardingRunning records that the provisioning workflow started.
func (service *Service) MarkOnboardingRunning(
	ctx context.Context,
	input MarkOnboardingRunningInput,
) (domain.Operation, error) {
	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return domain.Operation{}, err
	}

	if err := validateUUID(
		"operation_id",
		input.OperationID,
	); err != nil {
		return domain.Operation{}, err
	}

	now := service.now().UTC()

	event, err := newOperationOutboxEvent(
		ctx,
		service.config.EventTopic,
		"tenant.onboarding_started",
		input.TenantID,
		input.OperationID,
		2,
		&tenantv1.TenantOnboardingStarted{
			TenantId:      input.TenantID,
			OperationId:   input.OperationID,
			WorkflowId:    input.WorkflowID,
			WorkflowRunId: input.WorkflowRunID,
			StartedAt:     timestamppb.New(now),
		},
		now,
	)
	if err != nil {
		return domain.Operation{}, err
	}

	return service.repository.MarkOnboardingRunning(
		ctx,
		ports.MarkOnboardingRunningParams{
			ServicePrincipalID: service.config.WorkflowServicePrincipalID,
			TenantID:           input.TenantID,
			OperationID:        input.OperationID,
			WorkflowID:         input.WorkflowID,
			WorkflowRunID:      input.WorkflowRunID,
			StartedAt:          now,
			Event:              event,
		},
	)
}

// CompleteOnboarding activates the tenant and succeeds the operation. The
// activation outbox event is built inside the storage transaction with the
// freshly activated context.
func (service *Service) CompleteOnboarding(
	ctx context.Context,
	input CompleteOnboardingInput,
) (domain.CreateTenantResult, error) {
	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	if err := validateUUID(
		"operation_id",
		input.OperationID,
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	now := service.now().UTC()

	return service.repository.CompleteOnboarding(
		ctx,
		ports.CompleteOnboardingParams{
			ServicePrincipalID: service.config.WorkflowServicePrincipalID,
			TenantID:           input.TenantID,
			OperationID:        input.OperationID,
			CompletedAt:        now,
			Event: func(contextValue domain.TenantContext,
			) (domain.OutboxEvent, error) {
				return newTenantOutboxEvent(
					ctx,
					service.config.EventTopic,
					"tenant.activated",
					input.TenantID,
					2,
					&tenantv1.TenantActivated{
						Context:     protoutil.Context(contextValue),
						OperationId: input.OperationID,
						ActivatedAt: timestamppb.New(now),
					},
					now,
				)
			},
		},
	)
}

// FailOnboarding records a terminal provisioning failure. The failure
// outbox event is built inside the storage transaction with the failed
// tenant.
func (service *Service) FailOnboarding(
	ctx context.Context,
	input FailOnboardingInput,
) (domain.Operation, domain.Tenant, error) {
	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	if err := validateUUID(
		"operation_id",
		input.OperationID,
	); err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	if err := validateOperationError(input.Error); err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	operation, tenant, err := service.repository.FailOnboarding(
		ctx,
		ports.FailOnboardingParams{
			ServicePrincipalID: service.config.WorkflowServicePrincipalID,
			TenantID:           input.TenantID,
			OperationID:        input.OperationID,
			FailedAt:           service.now().UTC(),
			Error:              input.Error,
			Event: func(
				failedTenant domain.Tenant,
				failedOperation domain.Operation,
			) (domain.OutboxEvent, error) {
				failedAt := service.now().UTC()

				return newTenantOutboxEvent(
					ctx,
					service.config.EventTopic,
					"tenant.onboarding_failed",
					input.TenantID,
					2,
					&tenantv1.TenantOnboardingFailed{
						Tenant:    protoutil.Tenant(failedTenant),
						Operation: protoutil.Operation(failedOperation),
						FailedAt:  timestamppb.New(failedAt),
					},
					failedAt,
				)
			},
		},
	)
	if err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	return operation, tenant, nil
}

// GetOperation returns the caller's operation by ID.
func (service *Service) GetOperation(
	ctx context.Context,
	actorUserID string,
	operationID string,
) (domain.Operation, error) {
	if err := validateUUID(
		"actor_user_id",
		actorUserID,
	); err != nil {
		return domain.Operation{}, err
	}

	if err := validateUUID(
		"operation_id",
		operationID,
	); err != nil {
		return domain.Operation{}, err
	}

	return service.repository.GetOperation(
		ctx,
		actorUserID,
		operationID,
	)
}

// ToOperationProto maps an operation to the common contract. It is exposed
// here so transport layers share one mapper.
func ToOperationProto(
	operation domain.Operation,
) *commonv1.Operation {
	return protoutil.Operation(operation)
}
