// Package grpc exposes the Company and Agent Service over gRPC.
package grpc

import (
	"context"
	"errors"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/organization-agent/internal/application"
	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/aminio9/gereh/services/organization-agent/internal/protoutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements OrganizationService.
type Server struct {
	organizationv1.UnimplementedOrganizationServiceServer

	service *application.Service
}

// New creates the Organization Service gRPC transport.
func New(service *application.Service) *Server {
	return &Server{
		service: service,
	}
}

// CreateCompany creates a company.
func (server *Server) CreateCompany(
	ctx context.Context,
	request *organizationv1.CreateCompanyRequest,
) (*organizationv1.CreateCompanyResponse, error) {
	result, err := server.service.CreateCompany(
		ctx,
		application.CreateCompanyInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			Slug:        request.GetSlug(),
			DisplayName: request.GetDisplayName(),
			Description: request.GetDescription(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.CreateCompanyResponse{
		Company: protoutil.Company(result),
	}, nil
}

// GetCompany returns a company.
func (server *Server) GetCompany(
	ctx context.Context,
	request *organizationv1.GetCompanyRequest,
) (*organizationv1.GetCompanyResponse, error) {
	result, err := server.service.GetCompany(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetCompanyId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.GetCompanyResponse{
		Company: protoutil.Company(result),
	}, nil
}

// ListCompanies lists a tenant's companies.
func (server *Server) ListCompanies(
	ctx context.Context,
	request *organizationv1.ListCompaniesRequest,
) (*organizationv1.ListCompaniesResponse, error) {
	companies, nextToken, err :=
		server.service.ListCompanies(
			ctx,
			request.GetActorUserId(),
			request.GetTenantId(),
			request.GetPageSize(),
			request.GetPageToken(),
			request.GetIncludeArchived(),
		)
	if err != nil {
		return nil, mapError(err)
	}

	items := make(
		[]*organizationv1.Company,
		0,
		len(companies),
	)

	for _, value := range companies {
		items = append(
			items,
			protoutil.Company(value),
		)
	}

	return &organizationv1.ListCompaniesResponse{
		Companies:     items,
		NextPageToken: nextToken,
	}, nil
}

// UpdateCompany updates company settings.
func (server *Server) UpdateCompany(
	ctx context.Context,
	request *organizationv1.UpdateCompanyRequest,
) (*organizationv1.UpdateCompanyResponse, error) {
	result, err := server.service.UpdateCompany(
		ctx,
		application.UpdateCompanyInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			CompanyID:       request.GetCompanyId(),
			ExpectedVersion: request.GetExpectedVersion(),
			DisplayName:     request.DisplayName,
			Description:     request.Description,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.UpdateCompanyResponse{
		Company: protoutil.Company(result),
	}, nil
}

// ArchiveCompany archives a company.
func (server *Server) ArchiveCompany(
	ctx context.Context,
	request *organizationv1.ArchiveCompanyRequest,
) (*organizationv1.ArchiveCompanyResponse, error) {
	result, err := server.service.ArchiveCompany(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetCompanyId(),
		request.GetExpectedVersion(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.ArchiveCompanyResponse{
		Company: protoutil.Company(result),
	}, nil
}

// CreateAgent creates an agent.
func (server *Server) CreateAgent(
	ctx context.Context,
	request *organizationv1.CreateAgentRequest,
) (*organizationv1.CreateAgentResponse, error) {
	var configuration map[string]any

	if value := request.GetConfiguration(); value != nil {
		configuration = value.AsMap()
	}

	result, err := server.service.CreateAgent(
		ctx,
		application.CreateAgentInput{
			ActorUserID:    request.GetActorUserId(),
			TenantID:       request.GetTenantId(),
			CompanyID:      request.GetCompanyId(),
			Slug:           request.GetSlug(),
			DisplayName:    request.GetDisplayName(),
			RoleTitle:      request.GetRoleTitle(),
			Objective:      request.GetObjective(),
			ManagerAgentID: request.ManagerAgentId,

			ExecutionProfile: protoutil.DomainExecutionProfile(
				request.GetExecutionProfile(),
			),
			AutonomyLevel: protoutil.DomainAutonomyLevel(
				request.GetAutonomyLevel(),
			),

			Capabilities:  request.GetCapabilities(),
			Configuration: configuration,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.CreateAgentResponse{
		Agent: protoutil.Agent(result),
	}, nil
}

// GetAgent returns an agent.
func (server *Server) GetAgent(
	ctx context.Context,
	request *organizationv1.GetAgentRequest,
) (*organizationv1.GetAgentResponse, error) {
	result, err := server.service.GetAgent(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetAgentId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.GetAgentResponse{
		Agent: protoutil.Agent(result),
	}, nil
}

// ListAgents lists one company's agents.
func (server *Server) ListAgents(
	ctx context.Context,
	request *organizationv1.ListAgentsRequest,
) (*organizationv1.ListAgentsResponse, error) {
	agents, nextToken, err :=
		server.service.ListAgents(
			ctx,
			request.GetActorUserId(),
			request.GetTenantId(),
			request.GetCompanyId(),
			request.GetPageSize(),
			request.GetPageToken(),
			request.GetIncludeDeleted(),
		)
	if err != nil {
		return nil, mapError(err)
	}

	items := make(
		[]*organizationv1.Agent,
		0,
		len(agents),
	)

	for _, value := range agents {
		items = append(
			items,
			protoutil.Agent(value),
		)
	}

	return &organizationv1.ListAgentsResponse{
		Agents:        items,
		NextPageToken: nextToken,
	}, nil
}

// UpdateAgent updates an agent.
func (server *Server) UpdateAgent(
	ctx context.Context,
	request *organizationv1.UpdateAgentRequest,
) (*organizationv1.UpdateAgentResponse, error) {
	input := application.UpdateAgentInput{
		ActorUserID:     request.GetActorUserId(),
		TenantID:        request.GetTenantId(),
		AgentID:         request.GetAgentId(),
		ExpectedVersion: request.GetExpectedVersion(),

		DisplayName: request.DisplayName,
		RoleTitle:   request.RoleTitle,
		Objective:   request.Objective,
	}

	if value := request.ExecutionProfile; value != nil {
		profile := protoutil.DomainExecutionProfile(*value)
		input.ExecutionProfile = &profile
	}

	if value := request.AutonomyLevel; value != nil {
		level := protoutil.DomainAutonomyLevel(*value)
		input.AutonomyLevel = &level
	}

	if capabilities := request.GetCapabilities(); capabilities != nil {
		values := capabilities.GetValues()
		input.Capabilities = &values
	}

	if configuration := request.GetConfiguration(); configuration != nil {
		input.Configuration = configuration.AsMap()
	}

	result, err := server.service.UpdateAgent(
		ctx,
		input,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.UpdateAgentResponse{
		Agent: protoutil.Agent(result),
	}, nil
}

// SetAgentManager reassigns an agent's manager.
func (server *Server) SetAgentManager(
	ctx context.Context,
	request *organizationv1.SetAgentManagerRequest,
) (*organizationv1.SetAgentManagerResponse, error) {
	result, err := server.service.SetAgentManager(
		ctx,
		application.SetAgentManagerInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			AgentID:         request.GetAgentId(),
			ExpectedVersion: request.GetExpectedVersion(),
			ManagerAgentID:  request.ManagerAgentId,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.SetAgentManagerResponse{
		Agent: protoutil.Agent(result),
	}, nil
}

// PauseAgent pauses an agent.
func (server *Server) PauseAgent(
	ctx context.Context,
	request *organizationv1.PauseAgentRequest,
) (*organizationv1.PauseAgentResponse, error) {
	result, err := server.service.PauseAgent(
		ctx,
		application.LifecycleInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			AgentID:         request.GetAgentId(),
			ExpectedVersion: request.GetExpectedVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.PauseAgentResponse{
		Agent: protoutil.Agent(result),
	}, nil
}

// ResumeAgent resumes a paused agent.
func (server *Server) ResumeAgent(
	ctx context.Context,
	request *organizationv1.ResumeAgentRequest,
) (*organizationv1.ResumeAgentResponse, error) {
	result, err := server.service.ResumeAgent(
		ctx,
		application.LifecycleInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			AgentID:         request.GetAgentId(),
			ExpectedVersion: request.GetExpectedVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.ResumeAgentResponse{
		Agent: protoutil.Agent(result),
	}, nil
}

// DeleteAgent soft-deletes an agent.
func (server *Server) DeleteAgent(
	ctx context.Context,
	request *organizationv1.DeleteAgentRequest,
) (*organizationv1.DeleteAgentResponse, error) {
	result, err := server.service.DeleteAgent(
		ctx,
		application.LifecycleInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			AgentID:         request.GetAgentId(),
			ExpectedVersion: request.GetExpectedVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &organizationv1.DeleteAgentResponse{
		Agent: protoutil.Agent(result),
	}, nil
}

// GetAgentHierarchy returns a company's reporting tree.
func (server *Server) GetAgentHierarchy(
	ctx context.Context,
	request *organizationv1.GetAgentHierarchyRequest,
) (*organizationv1.GetAgentHierarchyResponse, error) {
	nodes, err := server.service.GetHierarchy(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetCompanyId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	items := make(
		[]*organizationv1.AgentHierarchyNode,
		0,
		len(nodes),
	)

	for _, value := range nodes {
		items = append(
			items,
			&organizationv1.AgentHierarchyNode{
				Agent: protoutil.Agent(value.Agent),
				Depth: value.Depth,
			},
		)
	}

	return &organizationv1.GetAgentHierarchyResponse{
		Nodes: items,
	}, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(
			codes.InvalidArgument,
			"invalid organization request",
		)

	case errors.Is(err, domain.ErrNotFound):
		return status.Error(
			codes.NotFound,
			"organization resource not found",
		)

	case errors.Is(err, domain.ErrForbidden):
		return status.Error(
			codes.PermissionDenied,
			"organization operation forbidden",
		)

	case errors.Is(err, domain.ErrTenantNotActive):
		return status.Error(
			codes.FailedPrecondition,
			"tenant is not active",
		)

	case errors.Is(err, domain.ErrVersionConflict):
		return status.Error(
			codes.Aborted,
			"organization resource changed; reload and retry",
		)

	case errors.Is(err, domain.ErrConflict):
		return status.Error(
			codes.AlreadyExists,
			"organization resource already exists",
		)

	case errors.Is(err, domain.ErrHierarchyCycle):
		return status.Error(
			codes.FailedPrecondition,
			"agent reporting hierarchy would contain a cycle",
		)

	case errors.Is(err, domain.ErrAgentHasReports):
		return status.Error(
			codes.FailedPrecondition,
			"agent still has direct reports",
		)

	case errors.Is(err, domain.ErrCompanyHasAgents):
		return status.Error(
			codes.FailedPrecondition,
			"company still has active agents",
		)

	case errors.Is(err, domain.ErrInvalidTransition):
		return status.Error(
			codes.FailedPrecondition,
			"invalid agent lifecycle transition",
		)

	case errors.Is(err, domain.ErrDefaultCompany):
		return status.Error(
			codes.FailedPrecondition,
			"default company cannot be archived",
		)

	default:
		return status.Error(
			codes.Internal,
			"organization operation failed",
		)
	}
}
