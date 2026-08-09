package grpc

import (
	"context"

	projectionv1 "github.com/aminio9/gereh/gen/go/gereh/projection/v1"
	"github.com/aminio9/gereh/services/projection/internal/application"
	"github.com/aminio9/gereh/services/projection/internal/protoutil"
)

// Server exposes the Projection Service use cases over gRPC.
type Server struct {
	projectionv1.UnimplementedProjectionServiceServer

	service *application.Service
}

// NewServer creates the gRPC adapter for the application service.
func NewServer(
	service *application.Service,
) *Server {
	return &Server{
		service: service,
	}
}

// GetDashboardSummary returns the tenant dashboard read model.
func (server *Server) GetDashboardSummary(
	ctx context.Context,
	request *projectionv1.GetDashboardSummaryRequest,
) (
	*projectionv1.GetDashboardSummaryResponse,
	error,
) {
	summary, metadata, err := server.service.GetDashboardSummary(
		ctx,
		application.GetDashboardSummaryInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &projectionv1.GetDashboardSummaryResponse{
		Summary:  protoutil.DashboardSummary(summary),
		Metadata: protoutil.Metadata(metadata),
	}, nil
}

// GetCompanyOverview returns one company read model.
func (server *Server) GetCompanyOverview(
	ctx context.Context,
	request *projectionv1.GetCompanyOverviewRequest,
) (
	*projectionv1.GetCompanyOverviewResponse,
	error,
) {
	overview, metadata, err := server.service.GetCompanyOverview(
		ctx,
		application.GetCompanyOverviewInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			CompanyID:   request.GetCompanyId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &projectionv1.GetCompanyOverviewResponse{
		Company:  protoutil.CompanyOverview(overview),
		Metadata: protoutil.Metadata(metadata),
	}, nil
}

// ListAgentOverviews returns a page of agent read models.
func (server *Server) ListAgentOverviews(
	ctx context.Context,
	request *projectionv1.ListAgentOverviewsRequest,
) (
	*projectionv1.ListAgentOverviewsResponse,
	error,
) {
	cursor, err := parseAgentCursor(
		request.GetPageToken(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	agents, nextCursor, metadata, err := server.service.ListAgentOverviews(
		ctx,
		application.ListAgentOverviewsInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			CompanyID:   request.GetCompanyId(),
			PageSize:    normalizePageSize(request.GetPageSize()),
			Cursor:      cursor,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	response := &projectionv1.ListAgentOverviewsResponse{
		NextPageToken: encodeAgentCursor(nextCursor),
		Metadata:      protoutil.Metadata(metadata),
	}

	for _, agent := range agents {
		response.Agents = append(
			response.Agents,
			protoutil.AgentOverview(agent),
		)
	}

	return response, nil
}

// ListTaskActivity returns a page of the tenant task activity feed.
func (server *Server) ListTaskActivity(
	ctx context.Context,
	request *projectionv1.ListTaskActivityRequest,
) (
	*projectionv1.ListTaskActivityResponse,
	error,
) {
	cursor, err := parseActivityCursor(
		request.GetPageToken(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	items, nextCursor, metadata, err := server.service.ListTaskActivity(
		ctx,
		application.ListTaskActivityInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			CompanyID:   request.GetCompanyId(),
			TaskID:      request.GetTaskId(),
			PageSize:    normalizePageSize(request.GetPageSize()),
			Cursor:      cursor,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	response := &projectionv1.ListTaskActivityResponse{
		NextPageToken: encodeActivityCursor(nextCursor),
		Metadata:      protoutil.Metadata(metadata),
	}

	for _, item := range items {
		response.Items = append(
			response.Items,
			protoutil.Activity(item),
		)
	}

	return response, nil
}

// Search returns a page of tenant search results.
func (server *Server) Search(
	ctx context.Context,
	request *projectionv1.SearchRequest,
) (
	*projectionv1.SearchResponse,
	error,
) {
	documentTypes, err := mapSearchTypes(
		request.GetTypes(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	cursor, err := parseSearchCursor(
		request.GetPageToken(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	results, nextCursor, metadata, err := server.service.Search(
		ctx,
		application.SearchInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			Query:       request.GetQuery(),
			CompanyID:   request.GetCompanyId(),
			Types:       documentTypes,
			PageSize:    normalizePageSize(request.GetPageSize()),
			Cursor:      cursor,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	response := &projectionv1.SearchResponse{
		NextPageToken: encodeSearchCursor(nextCursor),
		Metadata:      protoutil.Metadata(metadata),
	}

	for _, result := range results {
		response.Results = append(
			response.Results,
			protoutil.SearchResult(result),
		)
	}

	return response, nil
}
