package grpc

import (
	"context"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/application"
	"github.com/aminio9/gereh/services/policy-approval/internal/protoutil"
)

// BootstrapServer implements PolicyBootstrapService.
type BootstrapServer struct {
	policyv1.UnimplementedPolicyBootstrapServiceServer

	service *application.Service
}

// NewBootstrap creates the Policy Bootstrap Service gRPC transport.
func NewBootstrap(
	service *application.Service,
) *BootstrapServer {
	return &BootstrapServer{
		service: service,
	}
}

// EnsureDefaultPolicies idempotently creates the tenant default policies.
func (server *BootstrapServer) EnsureDefaultPolicies(
	ctx context.Context,
	request *policyv1.EnsureDefaultPoliciesRequest,
) (*policyv1.EnsureDefaultPoliciesResponse, error) {
	policies, err := server.service.EnsureDefaultPolicies(
		ctx,
		application.EnsureDefaultPoliciesInput{
			TenantID:              request.GetTenantId(),
			OnboardingOperationID: request.GetOnboardingOperationId(),
			ActorUserID:           request.GetActorUserId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*policyv1.Policy, 0, len(policies))

	for _, policy := range policies {
		items = append(items, protoutil.Policy(policy))
	}

	return &policyv1.EnsureDefaultPoliciesResponse{
		Policies: items,
	}, nil
}
