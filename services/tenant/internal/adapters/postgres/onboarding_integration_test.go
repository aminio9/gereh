package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/ports"
	"github.com/google/uuid"
)

const workflowServicePrincipalID = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae0"

func testOutboxEvent(tenantID string) domain.OutboxEvent {
	return domain.OutboxEvent{
		ID:         uuid.NewString(),
		Topic:      "gereh.tenant.events.v1",
		Key:        tenantID,
		Envelope:   []byte{0x0a, 0x00},
		OccurredAt: time.Now().UTC(),
	}
}

func testOutboxEventFailureClosure(
	t *testing.T,
	tenantID string,
) func(domain.Tenant, domain.Operation) (domain.OutboxEvent, error) {
	t.Helper()

	return func(
		_ domain.Tenant,
		_ domain.Operation,
	) (domain.OutboxEvent, error) {
		return testOutboxEvent(tenantID), nil
	}
}

func testOutboxEventClosure(
	t *testing.T,
	tenantID string,
) func(domain.TenantContext) (domain.OutboxEvent, error) {
	t.Helper()

	return func(
		_ domain.TenantContext,
	) (domain.OutboxEvent, error) {
		return testOutboxEvent(tenantID), nil
	}
}

func TestOnboardingLifecycle(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actorID := mustV7(t)

	created, err := createTestTenantRaw(
		ctx,
		t,
		repository,
		actorID,
		"lifecycle-test",
		"onboarding-lifecycle",
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(
			t,
			testCleanupPool(t),
			[]string{created.Context.Tenant.ID},
			actorID,
		)
	})

	if created.Context.Tenant.Status != domain.StatusProvisioning {
		t.Fatalf(
			"tenant status = %q, want provisioning",
			created.Context.Tenant.Status,
		)
	}

	if created.Operation.State != domain.OperationStatePending {
		t.Fatalf(
			"operation state = %q, want pending",
			created.Operation.State,
		)
	}

	running, err := repository.MarkOnboardingRunning(
		ctx,
		ports.MarkOnboardingRunningParams{
			ServicePrincipalID: workflowServicePrincipalID,
			TenantID:           created.Context.Tenant.ID,
			OperationID:        created.Operation.ID,
			WorkflowID:         "tenant-onboarding/" + created.Operation.ID,
			WorkflowRunID:      mustV7(t),
			StartedAt:          time.Now().UTC(),
			Event:              testOutboxEvent(created.Context.Tenant.ID),
		},
	)
	if err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if running.State != domain.OperationStateRunning {
		t.Fatalf(
			"operation state = %q, want running",
			running.State,
		)
	}

	if running.WorkflowID == "" ||
		running.WorkflowRunID == "" {
		t.Fatal("workflow identifiers were not persisted")
	}

	completed, err := repository.CompleteOnboarding(
		ctx,
		ports.CompleteOnboardingParams{
			ServicePrincipalID: workflowServicePrincipalID,
			TenantID:           created.Context.Tenant.ID,
			OperationID:        created.Operation.ID,
			CompletedAt:        time.Now().UTC(),
			Event: testOutboxEventClosure(
				t,
				created.Context.Tenant.ID,
			),
		},
	)
	if err != nil {
		t.Fatalf("complete onboarding: %v", err)
	}

	if completed.Context.Tenant.Status != domain.StatusActive {
		t.Fatalf(
			"tenant status = %q, want active",
			completed.Context.Tenant.Status,
		)
	}

	if completed.Operation.State != domain.OperationStateSucceeded {
		t.Fatalf(
			"operation state = %q, want succeeded",
			completed.Operation.State,
		)
	}
}

func TestCompleteOnboardingIsIdempotent(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actorID := mustV7(t)

	created, err := createTestTenantRaw(
		ctx,
		t,
		repository,
		actorID,
		"complete-idempotent-test",
		"onboarding-complete-idempotent",
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(
			t,
			testCleanupPool(t),
			[]string{created.Context.Tenant.ID},
			actorID,
		)
	})

	if _, err := repository.MarkOnboardingRunning(
		ctx,
		ports.MarkOnboardingRunningParams{
			ServicePrincipalID: workflowServicePrincipalID,
			TenantID:           created.Context.Tenant.ID,
			OperationID:        created.Operation.ID,
			WorkflowID:         "tenant-onboarding/" + created.Operation.ID,
			WorkflowRunID:      mustV7(t),
			StartedAt:          time.Now().UTC(),
			Event:              testOutboxEvent(created.Context.Tenant.ID),
		},
	); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	now := time.Now().UTC()

	params := ports.CompleteOnboardingParams{
		ServicePrincipalID: workflowServicePrincipalID,
		TenantID:           created.Context.Tenant.ID,
		OperationID:        created.Operation.ID,
		CompletedAt:        now,
		Event: testOutboxEventClosure(
			t,
			created.Context.Tenant.ID,
		),
	}

	first, err := repository.CompleteOnboarding(
		ctx,
		params,
	)
	if err != nil {
		t.Fatalf("first complete: %v", err)
	}

	second, err := repository.CompleteOnboarding(
		ctx,
		params,
	)
	if err != nil {
		t.Fatalf("second complete: %v", err)
	}

	if first.Context.Tenant.ID !=
		second.Context.Tenant.ID {
		t.Fatal("idempotent complete returned a different tenant")
	}

	if first.Operation.ID != second.Operation.ID {
		t.Fatal("idempotent complete returned a different operation")
	}
}

func TestFailOnboardingIsTerminal(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actorID := mustV7(t)

	created, err := createTestTenantRaw(
		ctx,
		t,
		repository,
		actorID,
		"fail-terminal-test",
		"onboarding-fail-terminal",
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(
			t,
			testCleanupPool(t),
			[]string{created.Context.Tenant.ID},
			actorID,
		)
	})

	failed, failedTenant, err :=
		repository.FailOnboarding(
			ctx,
			ports.FailOnboardingParams{
				ServicePrincipalID: workflowServicePrincipalID,
				TenantID:           created.Context.Tenant.ID,
				OperationID:        created.Operation.ID,
				FailedAt:           time.Now().UTC(),
				Error: domain.OperationError{
					Code:      "runtime_unavailable",
					Message:   "provisioning could not complete",
					Retryable: false,
					Details:   map[string]any{"workflow": "gereh.provision-tenant.v1"},
				},
				Event: testOutboxEventFailureClosure(
					t,
					created.Context.Tenant.ID,
				),
			},
		)
	if err != nil {
		t.Fatalf("fail onboarding: %v", err)
	}

	if failedTenant.Status !=
		domain.StatusProvisioningFailed {
		t.Fatalf(
			"tenant status = %q, want provisioning_failed",
			failedTenant.Status,
		)
	}

	if failed.State != domain.OperationStateFailed {
		t.Fatalf(
			"operation state = %q, want failed",
			failed.State,
		)
	}

	if failed.Error == nil ||
		failed.Error.Code != "runtime_unavailable" {
		t.Fatalf("operation error was not persisted: %+v", failed.Error)
	}

	_, _, err = repository.FailOnboarding(
		ctx,
		ports.FailOnboardingParams{
			ServicePrincipalID: workflowServicePrincipalID,
			TenantID:           created.Context.Tenant.ID,
			OperationID:        created.Operation.ID,
			FailedAt:           time.Now().UTC(),
			Error: domain.OperationError{
				Code:      "runtime_unavailable",
				Message:   "provisioning could not complete",
				Retryable: false,
			},
			Event: testOutboxEventFailureClosure(
				t,
				created.Context.Tenant.ID,
			),
		},
	)
	if err != nil {
		t.Fatalf("idempotent fail must succeed: %v", err)
	}
}

func TestOperationReadRequiresActor(t *testing.T) {
	repository := repositoryIntegrationTest(t)

	ctx := context.Background()
	actorID := mustV7(t)

	created, err := createTestTenantRaw(
		ctx,
		t,
		repository,
		actorID,
		"operation-actor-test",
		"onboarding-operation-actor",
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenants(
			t,
			testCleanupPool(t),
			[]string{created.Context.Tenant.ID},
			actorID,
		)
	})

	read, err := repository.GetOperation(
		ctx,
		actorID,
		created.Operation.ID,
	)
	if err != nil {
		t.Fatalf("read own operation: %v", err)
	}

	if read.ID != created.Operation.ID {
		t.Fatal("read operation ID mismatch")
	}

	otherActor := mustV7(t)

	_, err = repository.GetOperation(
		ctx,
		otherActor,
		created.Operation.ID,
	)
	if err == nil {
		t.Fatal("another actor must not read this operation")
	}
}
