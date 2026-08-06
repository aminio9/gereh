// Package application contains the Temporal workflows and activities.
package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/execution-orchestrator/internal/domain"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// Activities are the Temporal activities used by the provisioning workflow.
type Activities struct {
	tenant       ports.TenantOnboardingClient
	runtime      ports.RuntimeProvisioner
	organization ports.OrganizationBootstrapClient
}

// NewActivities creates the activity implementation.
func NewActivities(
	tenant ports.TenantOnboardingClient,
	runtime ports.RuntimeProvisioner,
	organization ports.OrganizationBootstrapClient,
) *Activities {
	return &Activities{
		tenant:       tenant,
		runtime:      runtime,
		organization: organization,
	}
}

// MarkRunningInput identifies the operation whose workflow is starting.
type MarkRunningInput struct {
	TenantID    string
	OperationID string
}

// MarkRunning records the workflow start in the Tenant Service.
func (activities *Activities) MarkRunning(
	ctx context.Context,
	input MarkRunningInput,
) error {
	info := activity.GetInfo(ctx)

	return activities.tenant.MarkRunning(
		ctx,
		ports.MarkRunningRequest{
			TenantID:      input.TenantID,
			OperationID:   input.OperationID,
			WorkflowID:    info.WorkflowExecution.ID,
			WorkflowRunID: info.WorkflowExecution.RunID,
		},
	)
}

// EnsureDefaultCompany idempotently creates a tenant's default company.
func (activities *Activities) EnsureDefaultCompany(
	ctx context.Context,
	input domain.ProvisionTenantInput,
) error {
	return activities.organization.EnsureDefaultCompany(
		ctx,
		ports.EnsureDefaultCompanyRequest{
			TenantID:              input.TenantID,
			OnboardingOperationID: input.OperationID,
			ActorUserID:           input.ActorUserID,
			TenantDisplayName:     input.TenantDisplayName,
		},
	)
}

// EnsureTenantRuntime provisions the tenant's runtime infrastructure.
func (activities *Activities) EnsureTenantRuntime(
	ctx context.Context,
	input domain.ProvisionTenantInput,
) error {
	err := activities.runtime.EnsureTenantRuntime(
		ctx,
		ports.EnsureTenantRuntimeRequest{
			TenantID:      input.TenantID,
			OperationID:   input.OperationID,
			Region:        input.Region,
			IsolationTier: "standard",
		},
	)
	if err == nil {
		return nil
	}

	var permanent *PermanentProvisioningError
	if errors.As(err, &permanent) {
		return temporal.NewNonRetryableApplicationError(
			permanent.Message,
			permanent.Code,
			err,
		)
	}

	return fmt.Errorf(
		"ensure tenant runtime: %w",
		err,
	)
}

// Complete activates the tenant after provisioning succeeds.
func (activities *Activities) Complete(
	ctx context.Context,
	input domain.ProvisionTenantInput,
) error {
	return activities.tenant.Complete(
		ctx,
		input.TenantID,
		input.OperationID,
	)
}

// FailInput carries the provisioning input and the failure to persist.
type FailInput struct {
	Provision domain.ProvisionTenantInput
	Failure   domain.OperationFailure
}

// Fail records a terminal provisioning failure in the Tenant Service.
func (activities *Activities) Fail(
	ctx context.Context,
	input FailInput,
) error {
	return activities.tenant.Fail(
		ctx,
		input.Provision.TenantID,
		input.Provision.OperationID,
		input.Failure,
	)
}

// PermanentProvisioningError marks a provisioning failure that must not be
// retried.
type PermanentProvisioningError struct {
	Code    string
	Message string
	Cause   error
}

// Error implements the error interface.
func (failure *PermanentProvisioningError) Error() string {
	return failure.Message
}

// Unwrap exposes the underlying cause.
func (failure *PermanentProvisioningError) Unwrap() error {
	return failure.Cause
}
