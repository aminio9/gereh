package application

import (
	"time"

	"github.com/aminio9/gereh/services/execution-orchestrator/internal/domain"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// ProvisionTenantWorkflowName is the registered workflow name.
	ProvisionTenantWorkflowName = "gereh.provision-tenant.v1"

	// OnboardingTaskQueue is the worker task queue for tenant onboarding.
	OnboardingTaskQueue = "gereh-tenant-onboarding"
)

// ProvisionTenantWorkflow provisions tenant infrastructure and activates the
// tenant. It contains no network or database calls; every external effect is
// an activity, preserving Temporal determinism.
func ProvisionTenantWorkflow(
	ctx workflow.Context,
	input domain.ProvisionTenantInput,
) error {
	stateActivityOptions := workflow.ActivityOptions{
		StartToCloseTimeout:    30 * time.Second,
		ScheduleToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    20,
		},
	}

	runtimeActivityOptions := workflow.ActivityOptions{
		StartToCloseTimeout:    2 * time.Minute,
		ScheduleToCloseTimeout: 24 * time.Hour,
		HeartbeatTimeout:       30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    100,
			NonRetryableErrorTypes: []string{
				"invalid_region",
				"unsupported_isolation_tier",
				"tenant_not_entitled",
				"invalid_runtime_configuration",
			},
		},
	}

	ctx = workflow.WithActivityOptions(
		ctx,
		stateActivityOptions,
	)

	if err := workflow.ExecuteActivity(
		ctx,
		"MarkRunning",
		MarkRunningInput{
			TenantID:    input.TenantID,
			OperationID: input.OperationID,
		},
	).Get(ctx, nil); err != nil {
		return err
	}

	runtimeContext := workflow.WithActivityOptions(
		ctx,
		runtimeActivityOptions,
	)

	err := workflow.ExecuteActivity(
		runtimeContext,
		"EnsureTenantRuntime",
		input,
	).Get(runtimeContext, nil)
	if err != nil {
		return failWorkflow(ctx, input, err)
	}

	if err := workflow.ExecuteActivity(
		ctx,
		"Complete",
		input,
	).Get(ctx, nil); err != nil {
		return failWorkflow(ctx, input, err)
	}

	return nil
}

// failWorkflow records a customer-safe terminal failure. It runs on a
// disconnected context so failure persistence is attempted even if the
// workflow is being canceled.
func failWorkflow(
	ctx workflow.Context,
	input domain.ProvisionTenantInput,
	cause error,
) error {
	failure := domain.OperationFailure{
		Code:      "tenant_provisioning_failed",
		Message:   "Tenant infrastructure could not be provisioned",
		Retryable: false,
		Details: map[string]any{
			// The cause is intentionally excluded from customer-visible
			// details. It may contain internal topology or provider data.
			"workflow": ProvisionTenantWorkflowName,
		},
	}

	disconnectedContext, _ := workflow.NewDisconnectedContext(ctx)

	failureContext := workflow.WithActivityOptions(
		disconnectedContext,
		workflow.ActivityOptions{
			StartToCloseTimeout:    30 * time.Second,
			ScheduleToCloseTimeout: 10 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2,
				MaximumInterval:    30 * time.Second,
				MaximumAttempts:    20,
			},
		},
	)

	failErr := workflow.ExecuteActivity(
		failureContext,
		"Fail",
		FailInput{
			Provision: input,
			Failure:   failure,
		},
	).Get(failureContext, nil)

	if failErr != nil {
		return temporal.NewApplicationError(
			"tenant provisioning failed and failure state could not be persisted",
			"tenant_failure_persistence_failed",
			cause,
			failErr,
		)
	}

	return cause
}
