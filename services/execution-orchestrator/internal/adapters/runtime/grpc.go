package runtime

import (
	"context"
	"fmt"
	"time"

	runtimev1 "github.com/aminio9/gereh/gen/go/gereh/runtime/v1"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/application"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
	"google.golang.org/grpc"
)

// GRPCProvisioner provisions runtime infrastructure through the Runtime
// Manager service.
type GRPCProvisioner struct {
	client runtimev1.RuntimeManagerServiceClient
}

// NewGRPCProvisioner creates the Runtime Manager gRPC adapter.
func NewGRPCProvisioner(
	connection grpc.ClientConnInterface,
) *GRPCProvisioner {
	return &GRPCProvisioner{
		client: runtimev1.NewRuntimeManagerServiceClient(
			connection,
		),
	}
}

// EnsureTenantRuntime asks the Runtime Manager to converge the tenant runtime.
func (provisioner *GRPCProvisioner) EnsureTenantRuntime(
	ctx context.Context,
	request ports.EnsureTenantRuntimeRequest,
) error {
	response, err := provisioner.client.EnsureTenantRuntime(
		ctx,
		&runtimev1.EnsureTenantRuntimeRequest{
			TenantId:      request.TenantID,
			OperationId:   request.OperationID,
			Region:        request.Region,
			IsolationTier: request.IsolationTier,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"ensure runtime through Runtime Manager: %w",
			err,
		)
	}

	switch response.GetState() {
	case runtimev1.
		RuntimeProvisioningState_RUNTIME_PROVISIONING_STATE_READY:
		return nil

	case runtimev1.
		RuntimeProvisioningState_RUNTIME_PROVISIONING_STATE_PENDING:
		retryAfter := 5 * time.Second

		if value := response.GetRetryAfter(); value != nil {
			if duration := value.AsDuration(); duration > 0 {
				retryAfter = duration
			}
		}

		return fmt.Errorf(
			"runtime provisioning remains pending; retry after %s",
			retryAfter,
		)

	case runtimev1.
		RuntimeProvisioningState_RUNTIME_PROVISIONING_STATE_FAILED:
		return &application.PermanentProvisioningError{
			Code:    response.GetErrorCode(),
			Message: "Runtime Manager rejected tenant provisioning",
		}

	default:
		return fmt.Errorf(
			"runtime manager returned an unspecified state",
		)
	}
}
