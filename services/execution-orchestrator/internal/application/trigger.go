package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/domain"
	enumsv1 "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/proto"
)

// TenantCreatedTrigger starts a provisioning workflow for each
// tenant.created event. It is idempotent: Kafka redelivery after a successful
// workflow start is not an error because the workflow ID is deterministic.
type TenantCreatedTrigger struct {
	temporal  client.Client
	taskQueue string
}

// NewTenantCreatedTrigger creates the Kafka-to-Temporal trigger.
func NewTenantCreatedTrigger(
	temporalClient client.Client,
	taskQueue string,
) *TenantCreatedTrigger {
	return &TenantCreatedTrigger{
		temporal: temporalClient,
		taskQueue: strings.TrimSpace(
			taskQueue,
		),
	}
}

// Handle processes one tenant event.
func (trigger *TenantCreatedTrigger) Handle(
	ctx context.Context,
	message platformkafka.Message,
) error {
	envelope := message.Envelope
	if envelope == nil {
		return &PermanentEventError{
			Reason: "event envelope is nil",
		}
	}

	if envelope.GetEventType() != "tenant.created" {
		return nil
	}

	if envelope.GetEventVersion() != 1 {
		return &PermanentEventError{
			Reason: fmt.Sprintf(
				"unsupported tenant.created version %d",
				envelope.GetEventVersion(),
			),
		}
	}

	payload := new(tenantv1.TenantCreated)

	if err := proto.Unmarshal(
		envelope.GetPayload(),
		payload,
	); err != nil {
		return &PermanentEventError{
			Reason: fmt.Sprintf(
				"decode TenantCreated payload: %v",
				err,
			),
		}
	}

	tenantContext := payload.GetContext()
	tenant := tenantContext.GetTenant()

	if tenant == nil ||
		tenant.GetTenantId() == "" ||
		payload.GetOperationId() == "" {
		return &PermanentEventError{
			Reason: "TenantCreated is missing tenant or operation identity",
		}
	}

	if envelope.GetTenantId() != tenant.GetTenantId() {
		return &PermanentEventError{
			Reason: "envelope and payload tenant IDs differ",
		}
	}

	workflowID := "tenant-onboarding/" +
		payload.GetOperationId()

	_, err := trigger.temporal.ExecuteWorkflow(
		ctx,
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: trigger.taskQueue,

			WorkflowExecutionTimeout: 48 * time.Hour,
			WorkflowRunTimeout:       48 * time.Hour,
			WorkflowTaskTimeout:      10 * time.Second,

			WorkflowIDReusePolicy: enumsv1.
				WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,

			WorkflowExecutionErrorWhenAlreadyStarted: true,
		},
		ProvisionTenantWorkflow,
		domain.ProvisionTenantInput{
			TenantID:    tenant.GetTenantId(),
			OperationID: payload.GetOperationId(),
			Region:      tenant.GetRegion(),
		},
	)
	if err == nil {
		return nil
	}

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted

	if errors.As(err, &alreadyStarted) {
		return nil
	}

	return fmt.Errorf(
		"start tenant onboarding workflow: %w",
		err,
	)
}

// PermanentEventError marks an event that cannot be processed in any
// retry. It is used for observability so a handler can classify failures.
type PermanentEventError struct {
	Reason string
}

// Error implements the error interface.
func (failure *PermanentEventError) Error() string {
	return failure.Reason
}
