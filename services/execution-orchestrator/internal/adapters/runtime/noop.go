// Package runtime contains runtime provisioning adapters.
package runtime

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
)

// NoopProvisioner is a local-development no-op runtime provisioner. It must
// never be selected in production.
type NoopProvisioner struct{}

// EnsureTenantRuntime validates the request and returns without effect.
func (NoopProvisioner) EnsureTenantRuntime(
	_ context.Context,
	request ports.EnsureTenantRuntimeRequest,
) error {
	if request.TenantID == "" ||
		request.OperationID == "" {
		return fmt.Errorf(
			"tenant and operation identity are required",
		)
	}

	return nil
}
