package grpc

import (
	"context"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/organization-agent/internal/application"
	"github.com/aminio9/gereh/services/organization-agent/internal/protoutil"
)

// BootstrapServer implements OrganizationBootstrapService.
//
// This service is workload-only and is protected by the internal workload
// interceptor.
type BootstrapServer struct {
	organizationv1.UnimplementedOrganizationBootstrapServiceServer

	service *application.Service
}

// NewBootstrap creates the bootstrap gRPC transport.
func NewBootstrap(
	service *application.Service,
) *BootstrapServer {
	return &BootstrapServer{
		service: service,
	}
}

// EnsureDefaultCompany idempotently creates a tenant's default company.
func (server *BootstrapServer) EnsureDefaultCompany(
	ctx context.Context,
	request *organizationv1.EnsureDefaultCompanyRequest,
) (*organizationv1.EnsureDefaultCompanyResponse, error) {
	result, err := server.service.EnsureDefaultCompany(
		ctx,
		application.EnsureDefaultCompanyInput{
			TenantID:              request.GetTenantId(),
			OnboardingOperationID: request.GetOnboardingOperationId(),
			ActorUserID:           request.GetActorUserId(),
			TenantDisplayName:     request.GetTenantDisplayName(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.EnsureDefaultCompanyResponse{
		Company: protoutil.Company(result),
	}, nil
}
