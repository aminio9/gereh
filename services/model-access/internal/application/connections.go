package application

import (
	"context"
	"fmt"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/aminio9/gereh/services/model-access/internal/protoutil"
	"github.com/google/uuid"
)

// CreateConnectionInput is a create command.
type CreateConnectionInput struct {
	ActorUserID string
	TenantID    string

	IdempotencyKey string

	ProviderKey string

	ConnectionType domain.ConnectionType

	DisplayName string
}

// UpdateConnectionInput is an update command.
type UpdateConnectionInput struct {
	ActorUserID string
	TenantID    string

	ConnectionID string

	IdempotencyKey string

	ExpectedVersion int64

	DisplayName string
}

// ArchiveConnectionInput is an archive command.
type ArchiveConnectionInput struct {
	ActorUserID string
	TenantID    string

	ConnectionID string

	IdempotencyKey string

	ExpectedVersion int64
}

// ListConnectionsInput is a list query.
type ListConnectionsInput struct {
	ActorUserID string
	TenantID    string

	Limit int

	Cursor *ports.ConnectionCursor

	IncludeArchived bool
}

// ListConnectionsResult is a page of connections.
type ListConnectionsResult struct {
	Connections []domain.Connection

	NextCursor *ports.ConnectionCursor
}

// ListProviders returns the enabled provider catalog.
func (service *Service) ListProviders(
	ctx context.Context,
	actorUserID string,
	tenantID string,
) ([]domain.Provider, error) {
	if err := validateUUID("actor_user_id", actorUserID); err != nil {
		return nil, err
	}

	if err := validateUUID("tenant_id", tenantID); err != nil {
		return nil, err
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_MODEL_PROVIDER_READ,
	); err != nil {
		return nil, err
	}

	return service.repository.ListProviders(ctx, actorUserID, tenantID)
}

// CreateConnection creates a connection idempotently.
//
// Platform-managed:
//
//	entitlement + active tenant + provider pool
//	=> ACTIVE immediately.
//
// BYOK/private endpoint:
//
//	=> DRAFT until later credential/endpoint phases verify them.
func (service *Service) CreateConnection(
	ctx context.Context,
	input CreateConnectionInput,
) (domain.Connection, error) {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return domain.Connection{}, err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return domain.Connection{}, err
	}

	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		return domain.Connection{}, err
	}

	providerKey, err := normalizeProviderKey(input.ProviderKey)
	if err != nil {
		return domain.Connection{}, err
	}

	if !input.ConnectionType.Valid() {
		return domain.Connection{}, fmt.Errorf(
			"%w: invalid connection type",
			domain.ErrInvalidArgument,
		)
	}

	displayName, err := normalizeDisplayName(input.DisplayName)
	if err != nil {
		return domain.Connection{}, err
	}

	requestHash, err := hashCanonical(createFingerprint{
		ProviderKey:    providerKey,
		ConnectionType: string(input.ConnectionType),
		DisplayName:    displayName,
	})
	if err != nil {
		return domain.Connection{}, err
	}

	connectionStatus := domain.ConnectionStatusDraft

	platformManagedRegion := ""

	switch input.ConnectionType {
	case domain.ConnectionTypePlatformManaged:
		accessContext, err := service.authorizer.RequireWithContext(
			ctx,
			input.ActorUserID,
			input.TenantID,
			tenantv1.Permission_PERMISSION_MODEL_CONNECTION_CREATE,
		)
		if err != nil {
			return domain.Connection{}, err
		}

		if !accessContext.Features["platform_managed_models"] {
			return domain.Connection{},
				domain.ErrPlatformManagedEntitlementRequired
		}

		if accessContext.Region == "" {
			return domain.Connection{},
				domain.ErrPlatformManagedPoolUnavailable
		}

		platformManagedRegion = accessContext.Region

		// No customer credential exists to verify.
		//
		// An eligible Gereh provider pool is selected atomically by
		// the repository. Therefore the control-plane connection may
		// become active immediately.
		connectionStatus = domain.ConnectionStatusActive

	default:
		if err := service.authorizer.Require(
			ctx,
			input.ActorUserID,
			input.TenantID,
			tenantv1.Permission_PERMISSION_MODEL_CONNECTION_CREATE,
		); err != nil {
			return domain.Connection{}, err
		}
	}

	connectionID, err := uuid.NewV7()
	if err != nil {
		return domain.Connection{}, fmt.Errorf(
			"generate model connection ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	connection := domain.Connection{
		TenantID:        input.TenantID,
		ID:              connectionID.String(),
		ProviderKey:     providerKey,
		ConnectionType:  input.ConnectionType,
		DisplayName:     displayName,
		Status:          connectionStatus,
		Version:         1,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return service.repository.CreateConnection(
		ctx,
		ports.CreateConnectionParams{
			ActorUserID:           input.ActorUserID,
			Connection:            connection,
			PlatformManagedRegion: platformManagedRegion,
			IdempotencyKey:        input.IdempotencyKey,
			RequestHash:           requestHash,
			IdempotencyExpiresAt:  now.Add(service.config.IdempotencyTTL),
			EventFactory: func(result domain.Connection) (domain.OutboxEvent, error) {
				return service.connectionEvent(
					ctx,
					"model.connection.created",
					result,
					&modelv1.ModelConnectionCreated{
						Connection: protoutil.Connection(result),
					},
					now,
				)
			},
		},
	)
}

// GetConnection returns one connection.
func (service *Service) GetConnection(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
) (domain.Connection, error) {
	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_MODEL_CONNECTION_READ,
	); err != nil {
		return domain.Connection{}, err
	}

	if err := validateUUID("connection_id", connectionID); err != nil {
		return domain.Connection{}, err
	}

	return service.repository.GetConnection(
		ctx,
		actorUserID,
		tenantID,
		connectionID,
	)
}

// ListConnections returns a page of connections.
func (service *Service) ListConnections(
	ctx context.Context,
	input ListConnectionsInput,
) (ListConnectionsResult, error) {
	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_MODEL_CONNECTION_READ,
	); err != nil {
		return ListConnectionsResult{}, err
	}

	limit := input.Limit

	if limit <= 0 {
		limit = 25
	}

	if limit > 100 {
		limit = 100
	}

	values, err := service.repository.ListConnections(
		ctx,
		input.ActorUserID,
		input.TenantID,
		limit+1,
		input.Cursor,
		input.IncludeArchived,
	)
	if err != nil {
		return ListConnectionsResult{}, err
	}

	result := ListConnectionsResult{Connections: values}

	if len(values) > limit {
		result.Connections = values[:limit]

		last := result.Connections[len(result.Connections)-1]

		result.NextCursor = &ports.ConnectionCursor{
			ConnectionID: last.ID,
		}
	}

	return result, nil
}

// UpdateConnection renames a connection with optimistic concurrency.
func (service *Service) UpdateConnection(
	ctx context.Context,
	input UpdateConnectionInput,
) (domain.Connection, error) {
	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_MODEL_CONNECTION_UPDATE,
	); err != nil {
		return domain.Connection{}, err
	}

	if input.ExpectedVersion <= 0 {
		return domain.Connection{}, fmt.Errorf(
			"%w: expected version must be positive",
			domain.ErrInvalidArgument,
		)
	}

	if err := validateUUID("connection_id", input.ConnectionID); err != nil {
		return domain.Connection{}, err
	}

	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		return domain.Connection{}, err
	}

	displayName, err := normalizeDisplayName(input.DisplayName)
	if err != nil {
		return domain.Connection{}, err
	}

	requestHash, err := hashCanonical(updateFingerprint{
		ConnectionID:    input.ConnectionID,
		ExpectedVersion: input.ExpectedVersion,
		DisplayName:     displayName,
	})
	if err != nil {
		return domain.Connection{}, err
	}

	now := service.now().UTC()

	return service.repository.UpdateConnection(
		ctx,
		ports.UpdateConnectionParams{
			ActorUserID:          input.ActorUserID,
			TenantID:             input.TenantID,
			ConnectionID:         input.ConnectionID,
			ExpectedVersion:      input.ExpectedVersion,
			DisplayName:          displayName,
			UpdatedAt:            now,
			IdempotencyKey:       input.IdempotencyKey,
			RequestHash:          requestHash,
			IdempotencyExpiresAt: now.Add(service.config.IdempotencyTTL),
			EventFactory: func(result domain.Connection) (domain.OutboxEvent, error) {
				return service.connectionEvent(
					ctx,
					"model.connection.updated",
					result,
					&modelv1.ModelConnectionUpdated{
						Connection:      protoutil.Connection(result),
						UpdatedByUserId: input.ActorUserID,
					},
					now,
				)
			},
		},
	)
}

// ArchiveConnection archives a connection with optimistic concurrency.
func (service *Service) ArchiveConnection(
	ctx context.Context,
	input ArchiveConnectionInput,
) (domain.Connection, error) {
	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_MODEL_CONNECTION_ARCHIVE,
	); err != nil {
		return domain.Connection{}, err
	}

	if input.ExpectedVersion <= 0 {
		return domain.Connection{}, fmt.Errorf(
			"%w: expected version must be positive",
			domain.ErrInvalidArgument,
		)
	}

	if err := validateUUID("connection_id", input.ConnectionID); err != nil {
		return domain.Connection{}, err
	}

	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		return domain.Connection{}, err
	}

	requestHash, err := hashCanonical(archiveFingerprint{
		ConnectionID:    input.ConnectionID,
		ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return domain.Connection{}, err
	}

	now := service.now().UTC()

	return service.repository.ArchiveConnection(
		ctx,
		ports.ArchiveConnectionParams{
			ActorUserID:          input.ActorUserID,
			TenantID:             input.TenantID,
			ConnectionID:         input.ConnectionID,
			ExpectedVersion:      input.ExpectedVersion,
			ArchivedAt:           now,
			IdempotencyKey:       input.IdempotencyKey,
			RequestHash:          requestHash,
			IdempotencyExpiresAt: now.Add(service.config.IdempotencyTTL),
			EventFactory: func(result domain.Connection) (domain.OutboxEvent, error) {
				return service.connectionEvent(
					ctx,
					"model.connection.archived",
					result,
					&modelv1.ModelConnectionArchived{
						Connection:       protoutil.Connection(result),
						ArchivedByUserId: input.ActorUserID,
					},
					now,
				)
			},
		},
	)
}
