// Package grpc implements the Model Access gRPC transport.
package grpc

import (
	"context"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	"github.com/aminio9/gereh/services/model-access/internal/application"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/aminio9/gereh/services/model-access/internal/protoutil"
)

// Server implements ModelAccessServiceServer.
type Server struct {
	modelv1.UnimplementedModelAccessServiceServer

	service *application.Service
}

// New constructs the gRPC server.
func New(service *application.Service) *Server {
	return &Server{service: service}
}

// ListProviders returns the enabled provider catalog.
func (server *Server) ListProviders(
	ctx context.Context,
	request *modelv1.ListProvidersRequest,
) (*modelv1.ListProvidersResponse, error) {
	values, err := server.service.ListProviders(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*modelv1.ModelProvider, 0, len(values))

	for _, value := range values {
		items = append(items, protoutil.Provider(value))
	}

	return &modelv1.ListProvidersResponse{Providers: items}, nil
}

// CreateConnection creates a draft connection.
func (server *Server) CreateConnection(
	ctx context.Context,
	request *modelv1.CreateConnectionRequest,
) (*modelv1.CreateConnectionResponse, error) {
	connectionType, err := protoutil.DomainConnectionType(
		request.GetConnectionType(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	value, err := server.service.CreateConnection(
		ctx,
		application.CreateConnectionInput{
			ActorUserID:    request.GetActorUserId(),
			TenantID:       request.GetTenantId(),
			IdempotencyKey: request.GetIdempotencyKey(),
			ProviderKey:    request.GetProviderKey(),
			ConnectionType: connectionType,
			DisplayName:    request.GetDisplayName(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.CreateConnectionResponse{
		Connection: protoutil.Connection(value),
	}, nil
}

// CreateBYOKConnection creates and verifies a BYOK connection.
func (server *Server) CreateBYOKConnection(
	ctx context.Context,
	request *modelv1.CreateBYOKConnectionRequest,
) (*modelv1.CreateBYOKConnectionResponse, error) {
	value, err := server.service.CreateBYOKConnection(
		ctx,
		application.CreateBYOKConnectionInput{
			ActorUserID:    request.GetActorUserId(),
			TenantID:       request.GetTenantId(),
			IdempotencyKey: request.GetIdempotencyKey(),
			ProviderKey:    request.GetProviderKey(),
			DisplayName:    request.GetDisplayName(),
			APIKey:         request.GetApiKey(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.CreateBYOKConnectionResponse{
		Connection: protoutil.Connection(value),
	}, nil
}

// RotateBYOKCredential rotates a BYOK credential.
func (server *Server) RotateBYOKCredential(
	ctx context.Context,
	request *modelv1.RotateBYOKCredentialRequest,
) (*modelv1.RotateBYOKCredentialResponse, error) {
	value, err := server.service.RotateBYOKCredential(
		ctx,
		application.RotateBYOKCredentialInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			ConnectionID:    request.GetConnectionId(),
			IdempotencyKey:  request.GetIdempotencyKey(),
			ExpectedVersion: request.GetExpectedVersion(),
			APIKey:          request.GetApiKey(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.RotateBYOKCredentialResponse{
		Connection: protoutil.Connection(value),
	}, nil
}

// GetConnection returns one connection.
func (server *Server) GetConnection(
	ctx context.Context,
	request *modelv1.GetConnectionRequest,
) (*modelv1.GetConnectionResponse, error) {
	value, err := server.service.GetConnection(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetConnectionId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.GetConnectionResponse{
		Connection: protoutil.Connection(value),
	}, nil
}

// ListConnections returns a page of connections.
func (server *Server) ListConnections(
	ctx context.Context,
	request *modelv1.ListConnectionsRequest,
) (*modelv1.ListConnectionsResponse, error) {
	cursor, err := decodeConnectionToken(request.GetPageToken())
	if err != nil {
		return nil, mapError(err)
	}

	result, err := server.service.ListConnections(
		ctx,
		application.ListConnectionsInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			Limit:           normalizePageSize(request.GetPageSize()),
			Cursor:          cursor,
			IncludeArchived: request.GetIncludeArchived(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*modelv1.ModelConnection, 0, len(result.Connections))

	for _, value := range result.Connections {
		items = append(items, protoutil.Connection(value))
	}

	return &modelv1.ListConnectionsResponse{
		Connections:   items,
		NextPageToken: encodeConnectionToken(result.NextCursor),
	}, nil
}

// UpdateConnection renames a connection.
func (server *Server) UpdateConnection(
	ctx context.Context,
	request *modelv1.UpdateConnectionRequest,
) (*modelv1.UpdateConnectionResponse, error) {
	if request.DisplayName == nil {
		return nil, mapError(domain.ErrInvalidArgument)
	}

	value, err := server.service.UpdateConnection(
		ctx,
		application.UpdateConnectionInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			ConnectionID:    request.GetConnectionId(),
			IdempotencyKey:  request.GetIdempotencyKey(),
			ExpectedVersion: request.GetExpectedVersion(),
			DisplayName:     request.GetDisplayName(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.UpdateConnectionResponse{
		Connection: protoutil.Connection(value),
	}, nil
}

// ArchiveConnection archives a connection.
func (server *Server) ArchiveConnection(
	ctx context.Context,
	request *modelv1.ArchiveConnectionRequest,
) (*modelv1.ArchiveConnectionResponse, error) {
	value, err := server.service.ArchiveConnection(
		ctx,
		application.ArchiveConnectionInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			ConnectionID:    request.GetConnectionId(),
			IdempotencyKey:  request.GetIdempotencyKey(),
			ExpectedVersion: request.GetExpectedVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.ArchiveConnectionResponse{
		Connection: protoutil.Connection(value),
	}, nil
}

// ListModelOfferings returns a page of model offerings.
func (server *Server) ListModelOfferings(
	ctx context.Context,
	request *modelv1.ListModelOfferingsRequest,
) (*modelv1.ListModelOfferingsResponse, error) {
	var cursor *ports.OfferingCursor
	if request.GetPageToken() != "" {
		cursor = &ports.OfferingCursor{OfferingID: request.GetPageToken()}
	}

	var connectionID string
	if request.ConnectionId != nil {
		connectionID = *request.ConnectionId
	}

	result, err := server.service.ListModelOfferings(
		ctx,
		application.ListOfferingsInput{
			ActorUserID:  request.GetActorUserId(),
			TenantID:     request.GetTenantId(),
			ConnectionID: connectionID,
			Limit:        normalizePageSize(request.GetPageSize()),
			Cursor:       cursor,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*modelv1.ModelOffering, 0, len(result.Offerings))
	for _, item := range result.Offerings {
		items = append(items, protoutil.ModelOffering(item))
	}

	nextPageToken := ""
	if result.NextCursor != nil {
		nextPageToken = result.NextCursor.OfferingID
	}

	return &modelv1.ListModelOfferingsResponse{
		Offerings:     items,
		NextPageToken: nextPageToken,
	}, nil
}

// RefreshModelCatalog enqueues a catalog refresh for a connection.
func (server *Server) RefreshModelCatalog(
	ctx context.Context,
	request *modelv1.RefreshModelCatalogRequest,
) (*modelv1.RefreshModelCatalogResponse, error) {
	refresh, err := server.service.RefreshModelCatalog(
		ctx,
		application.RefreshModelCatalogInput{
			ActorUserID:  request.GetActorUserId(),
			TenantID:     request.GetTenantId(),
			ConnectionID: request.GetConnectionId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.RefreshModelCatalogResponse{
		Refresh: protoutil.ModelCatalogRefresh(refresh),
	}, nil
}

// GetModelCatalogRefresh returns the status of a catalog refresh.
func (server *Server) GetModelCatalogRefresh(
	ctx context.Context,
	request *modelv1.GetModelCatalogRefreshRequest,
) (*modelv1.GetModelCatalogRefreshResponse, error) {
	refresh, err := server.service.GetModelCatalogRefresh(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetRefreshId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.GetModelCatalogRefreshResponse{
		Refresh: protoutil.ModelCatalogRefresh(refresh),
	}, nil
}

// GetAgentModelBinding returns the model binding for an agent.
func (server *Server) GetAgentModelBinding(
	ctx context.Context,
	request *modelv1.GetAgentModelBindingRequest,
) (*modelv1.GetAgentModelBindingResponse, error) {
	binding, err := server.service.GetAgentModelBinding(
		ctx,
		request.GetActorUserId(),
		request.GetTenantId(),
		request.GetAgentId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.GetAgentModelBindingResponse{
		Binding: protoutil.AgentModelBinding(binding),
	}, nil
}

// SetAgentModelBinding binds an agent to model offerings.
func (server *Server) SetAgentModelBinding(
	ctx context.Context,
	request *modelv1.SetAgentModelBindingRequest,
) (*modelv1.SetAgentModelBindingResponse, error) {
	fallbackPolicy := protoutil.DomainFallbackPolicy(request.GetFallbackPolicy())

	binding, err := server.service.SetAgentModelBinding(
		ctx,
		application.SetAgentBindingInput{
			ActorUserID:          request.GetActorUserId(),
			TenantID:             request.GetTenantId(),
			AgentID:              request.GetAgentId(),
			IdempotencyKey:       request.GetIdempotencyKey(),
			ExpectedVersion:      request.GetExpectedVersion(),
			PrimaryOfferingID:    request.GetPrimaryOfferingId(),
			FastOfferingID:       request.FastOfferingId,
			FallbackOfferingIDs:  request.GetFallbackOfferingIds(),
			FallbackPolicy:       fallbackPolicy,
			MaxModelCostMicroUSD: request.MaxModelCostMicrousd,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.SetAgentModelBindingResponse{
		Binding: protoutil.AgentModelBinding(binding),
	}, nil
}

// RemoveAgentModelBinding unbinds an agent from models.
func (server *Server) RemoveAgentModelBinding(
	ctx context.Context,
	request *modelv1.RemoveAgentModelBindingRequest,
) (*modelv1.RemoveAgentModelBindingResponse, error) {
	binding, err := server.service.RemoveAgentModelBinding(
		ctx,
		application.RemoveAgentBindingInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			AgentID:         request.GetAgentId(),
			IdempotencyKey:  request.GetIdempotencyKey(),
			ExpectedVersion: request.GetExpectedVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelv1.RemoveAgentModelBindingResponse{
		Binding: protoutil.AgentModelBinding(binding),
	}, nil
}
