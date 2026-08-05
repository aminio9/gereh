package grpc

import (
	"context"
	"errors"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/tenant/internal/application"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/protoutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements TenantService.
type Server struct {
	tenantv1.UnimplementedTenantServiceServer

	service *application.Service
}

// New creates the Tenant Service gRPC transport.
func New(service *application.Service) *Server {
	return &Server{
		service: service,
	}
}

// CreateTenant creates a tenant.
func (server *Server) CreateTenant(
	ctx context.Context,
	request *tenantv1.CreateTenantRequest,
) (*tenantv1.CreateTenantResponse, error) {
	result, err := server.service.CreateTenant(
		ctx,
		application.CreateTenantInput{
			ActorUserID:   request.GetActorUserId(),
			RequestID:     request.GetRequestId(),
			Slug:          request.GetSlug(),
			DisplayName:   request.GetDisplayName(),
			Region:        request.GetRegion(),
			RetentionDays: request.GetRetentionDays(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.CreateTenantResponse{
		Context: protoutil.Context(result),
	}, nil
}

// GetTenant returns tenant context.
func (server *Server) GetTenant(
	ctx context.Context,
	request *tenantv1.GetTenantRequest,
) (*tenantv1.GetTenantResponse, error) {
	result, err := server.service.GetTenantContext(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.GetTenantResponse{
		Context: protoutil.Context(result),
	}, nil
}

// ListTenants lists actor-visible tenants.
func (server *Server) ListTenants(
	ctx context.Context,
	request *tenantv1.ListTenantsRequest,
) (*tenantv1.ListTenantsResponse, error) {
	results, nextToken, err := server.service.ListTenants(
		ctx,
		request.GetActorUserId(),
		request.GetPageSize(),
		request.GetPageToken(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	contexts := make(
		[]*tenantv1.TenantContext,
		0,
		len(results),
	)

	for _, value := range results {
		contexts = append(
			contexts,
			protoutil.Context(value),
		)
	}

	return &tenantv1.ListTenantsResponse{
		Contexts:      contexts,
		NextPageToken: nextToken,
	}, nil
}

// UpdateTenant updates tenant settings.
func (server *Server) UpdateTenant(
	ctx context.Context,
	request *tenantv1.UpdateTenantRequest,
) (*tenantv1.UpdateTenantResponse, error) {
	result, err := server.service.UpdateTenant(
		ctx,
		application.UpdateTenantInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			ExpectedVersion: request.GetExpectedVersion(),
			DisplayName:     request.DisplayName,
			Region:          request.Region,
			RetentionDays:   request.RetentionDays,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.UpdateTenantResponse{
		Context: protoutil.Context(result),
	}, nil
}

// ArchiveTenant archives a tenant.
func (server *Server) ArchiveTenant(
	ctx context.Context,
	request *tenantv1.ArchiveTenantRequest,
) (*tenantv1.ArchiveTenantResponse, error) {
	result, err := server.service.ArchiveTenant(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetExpectedVersion(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.ArchiveTenantResponse{
		Context: protoutil.Context(result),
	}, nil
}

// GetTenantContext validates and returns tenant context.
func (server *Server) GetTenantContext(
	ctx context.Context,
	request *tenantv1.GetTenantContextRequest,
) (*tenantv1.GetTenantContextResponse, error) {
	result, err := server.service.GetTenantContext(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.GetTenantContextResponse{
		Context: protoutil.Context(result),
	}, nil
}

// ListMembers lists tenant memberships.
func (server *Server) ListMembers(
	ctx context.Context,
	request *tenantv1.ListMembersRequest,
) (*tenantv1.ListMembersResponse, error) {
	results, nextToken, err := server.service.ListMembers(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetPageSize(),
		request.GetPageToken(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	memberships := make(
		[]*tenantv1.TenantMembership,
		0,
		len(results),
	)

	for _, value := range results {
		memberships = append(
			memberships,
			protoutil.Membership(value),
		)
	}

	return &tenantv1.ListMembersResponse{
		Memberships:   memberships,
		NextPageToken: nextToken,
	}, nil
}

// AddMember adds a tenant member.
func (server *Server) AddMember(
	ctx context.Context,
	request *tenantv1.AddMemberRequest,
) (*tenantv1.AddMemberResponse, error) {
	membership, tenantVersion, err :=
		server.service.AddMember(
			ctx,
			application.MemberInput{
				ActorUserID: request.GetActorUserId(),
				TenantID:    request.GetTenantId(),
				UserID:      request.GetUserId(),
				Role:        protoutil.DomainRole(request.GetRole()),
			},
		)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.AddMemberResponse{
		Membership:    protoutil.Membership(membership),
		TenantVersion: tenantVersion,
	}, nil
}

// UpdateMemberRole updates a member role.
func (server *Server) UpdateMemberRole(
	ctx context.Context,
	request *tenantv1.UpdateMemberRoleRequest,
) (*tenantv1.UpdateMemberRoleResponse, error) {
	membership, tenantVersion, err :=
		server.service.UpdateMemberRole(
			ctx,
			application.UpdateMemberRoleInput{
				ActorUserID: request.GetActorUserId(),
				TenantID:    request.GetTenantId(),
				UserID:      request.GetUserId(),
				Role:        protoutil.DomainRole(request.GetRole()),
				ExpectedMembershipVersion: request.
					GetExpectedMembershipVersion(),
			},
		)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.UpdateMemberRoleResponse{
		Membership:    protoutil.Membership(membership),
		TenantVersion: tenantVersion,
	}, nil
}

// RemoveMember removes a tenant member.
func (server *Server) RemoveMember(
	ctx context.Context,
	request *tenantv1.RemoveMemberRequest,
) (*tenantv1.RemoveMemberResponse, error) {
	tenantVersion, err := server.service.RemoveMember(
		ctx,
		application.RemoveMemberInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			UserID:      request.GetUserId(),
			ExpectedMembershipVersion: request.
				GetExpectedMembershipVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &tenantv1.RemoveMemberResponse{
		TenantVersion: tenantVersion,
	}, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(
			codes.InvalidArgument,
			"invalid tenant request",
		)

	case errors.Is(err, domain.ErrNotFound):
		return status.Error(
			codes.NotFound,
			"tenant resource not found",
		)

	case errors.Is(err, domain.ErrForbidden):
		return status.Error(
			codes.PermissionDenied,
			"tenant operation forbidden",
		)

	case errors.Is(err, domain.ErrVersionConflict):
		return status.Error(
			codes.Aborted,
			"tenant resource changed; reload and retry",
		)

	case errors.Is(err, domain.ErrConflict):
		return status.Error(
			codes.AlreadyExists,
			"tenant resource already exists",
		)

	case errors.Is(err, domain.ErrLastOwner):
		return status.Error(
			codes.FailedPrecondition,
			"tenant must retain at least one owner",
		)

	case errors.Is(err, domain.ErrArchived):
		return status.Error(
			codes.FailedPrecondition,
			"tenant is archived",
		)

	default:
		return status.Error(
			codes.Internal,
			"tenant operation failed",
		)
	}
}
