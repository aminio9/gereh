package application

import (
	"context"
	"errors"
	"testing"

	"github.com/aminio9/gereh/services/execution-orchestrator/internal/domain"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// mockTenantClient satisfies ports.TenantOnboardingClient.
type mockTenantClient struct {
	mock.Mock
}

func (client *mockTenantClient) MarkRunning(
	_ context.Context,
	request ports.MarkRunningRequest,
) error {
	args := client.Called(request)
	return args.Error(0)
}

func (client *mockTenantClient) Complete(
	_ context.Context,
	tenantID string,
	operationID string,
) error {
	args := client.Called(tenantID, operationID)
	return args.Error(0)
}

func (client *mockTenantClient) Fail(
	_ context.Context,
	tenantID string,
	operationID string,
	failure domain.OperationFailure,
) error {
	args := client.Called(tenantID, operationID, failure)
	return args.Error(0)
}

// mockRuntimeProvisioner satisfies ports.RuntimeProvisioner.
type mockRuntimeProvisioner struct {
	mock.Mock
}

func (provisioner *mockRuntimeProvisioner) EnsureTenantRuntime(
	_ context.Context,
	request ports.EnsureTenantRuntimeRequest,
) error {
	args := provisioner.Called(request)
	return args.Error(0)
}

// mockOrganizationClient satisfies ports.OrganizationBootstrapClient.
type mockOrganizationClient struct {
	mock.Mock
}

func (client *mockOrganizationClient) EnsureDefaultCompany(
	_ context.Context,
	request ports.EnsureDefaultCompanyRequest,
) error {
	args := client.Called(request)
	return args.Error(0)
}

// mockPolicyClient satisfies ports.PolicyBootstrapClient.
type mockPolicyClient struct {
	mock.Mock
}

func (client *mockPolicyClient) EnsureDefaultPolicies(
	_ context.Context,
	request ports.EnsureDefaultPoliciesRequest,
) error {
	args := client.Called(request)
	return args.Error(0)
}

func newTestWorkflowEnvironment(
	t *testing.T,
	tenant *mockTenantClient,
	runtime *mockRuntimeProvisioner,
	organization *mockOrganizationClient,
	policy *mockPolicyClient,
) *testsuite.TestWorkflowEnvironment {
	t.Helper()

	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()

	activities := NewActivities(tenant, runtime, organization, policy)
	environment.RegisterActivity(activities)

	return environment
}

func TestProvisionTenantWorkflowCompletes(
	t *testing.T,
) {
	t.Parallel()

	tenant := new(mockTenantClient)
	defer tenant.AssertExpectations(t)

	runtime := new(mockRuntimeProvisioner)
	defer runtime.AssertExpectations(t)

	organization := new(mockOrganizationClient)
	defer organization.AssertExpectations(t)

	policy := new(mockPolicyClient)
	defer policy.AssertExpectations(t)

	environment := newTestWorkflowEnvironment(
		t,
		tenant,
		runtime,
		organization,
		policy,
	)

	input := testProvisionInput()

	tenant.
		On("MarkRunning", mock.Anything).
		Return(nil).
		Once()

	organization.
		On("EnsureDefaultCompany", mock.Anything).
		Return(nil).
		Once()

	policy.
		On("EnsureDefaultPolicies", mock.Anything).
		Return(nil).
		Once()

	runtime.
		On("EnsureTenantRuntime", mock.Anything).
		Return(nil).
		Once()

	tenant.
		On("Complete", input.TenantID, input.OperationID).
		Return(nil).
		Once()

	environment.ExecuteWorkflow(
		ProvisionTenantWorkflow,
		input,
	)

	require.True(t, environment.IsWorkflowCompleted())
	require.NoError(t, environment.GetWorkflowError())
}

func TestProvisionTenantWorkflowPersistsFailure(
	t *testing.T,
) {
	t.Parallel()

	tenant := new(mockTenantClient)
	defer tenant.AssertExpectations(t)
	runtime := new(mockRuntimeProvisioner)
	defer runtime.AssertExpectations(t)
	organization := new(mockOrganizationClient)
	defer organization.AssertExpectations(t)

	policy := new(mockPolicyClient)
	defer policy.AssertExpectations(t)

	environment := newTestWorkflowEnvironment(
		t,
		tenant,
		runtime,
		organization,
		policy,
	)

	input := testProvisionInput()

	tenant.
		On("MarkRunning", mock.Anything).
		Return(nil).
		Once()

	organization.
		On("EnsureDefaultCompany", mock.Anything).
		Return(nil).
		Once()

	policy.
		On("EnsureDefaultPolicies", mock.Anything).
		Return(nil).
		Once()

	runtime.
		On("EnsureTenantRuntime", mock.Anything).
		Return(errors.New("runtime unavailable")).
		Once()

	tenant.
		On("Fail", input.TenantID, input.OperationID, mock.Anything).
		Return(nil).
		Once()

	environment.ExecuteWorkflow(
		ProvisionTenantWorkflow,
		input,
	)

	require.True(t, environment.IsWorkflowCompleted())
	require.Error(t, environment.GetWorkflowError())
}

func TestProvisionTenantWorkflowPersistsCompleteFailure(
	t *testing.T,
) {
	t.Parallel()

	tenant := new(mockTenantClient)
	defer tenant.AssertExpectations(t)
	runtime := new(mockRuntimeProvisioner)
	defer runtime.AssertExpectations(t)
	organization := new(mockOrganizationClient)
	defer organization.AssertExpectations(t)

	policy := new(mockPolicyClient)
	defer policy.AssertExpectations(t)

	environment := newTestWorkflowEnvironment(
		t,
		tenant,
		runtime,
		organization,
		policy,
	)

	input := testProvisionInput()

	tenant.
		On("MarkRunning", mock.Anything).
		Return(nil).
		Once()

	organization.
		On("EnsureDefaultCompany", mock.Anything).
		Return(nil).
		Once()

	policy.
		On("EnsureDefaultPolicies", mock.Anything).
		Return(nil).
		Once()

	runtime.
		On("EnsureTenantRuntime", mock.Anything).
		Return(nil).
		Once()

	tenant.
		On("Complete", input.TenantID, input.OperationID).
		Return(errors.New("activation rejected")).
		Once()

	tenant.
		On("Fail", input.TenantID, input.OperationID, mock.Anything).
		Return(nil).
		Once()

	environment.ExecuteWorkflow(
		ProvisionTenantWorkflow,
		input,
	)

	require.True(t, environment.IsWorkflowCompleted())
	require.Error(t, environment.GetWorkflowError())
}

func testProvisionInput() domain.ProvisionTenantInput {
	return domain.ProvisionTenantInput{
		TenantID:          "018f7767-28d2-7f5c-a693-0bb4c8ee4ae1",
		OperationID:       "018f7767-28d2-7f5c-a693-0bb4c8ee4ae2",
		Region:            "local",
		ActorUserID:       "018f7767-28d2-7f5c-a693-0bb4c8ee4ae3",
		TenantDisplayName: "Example Org",
	}
}
