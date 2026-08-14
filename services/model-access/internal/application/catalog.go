package application

import (
	"context"
	"fmt"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListOfferingsInput contains query and pagination filters for listing offerings.
type ListOfferingsInput struct {
	ActorUserID string
	TenantID    string

	ConnectionID    string
	AgentUsableOnly bool

	Limit  int
	Cursor *ports.OfferingCursor
}

// ListOfferingsResult contains a page of offerings and an optional next page cursor.
type ListOfferingsResult struct {
	Offerings  []domain.ModelOffering
	NextCursor *ports.OfferingCursor
}

// ListModelOfferings lists model offerings available to a tenant.
func (service *Service) ListModelOfferings(
	ctx context.Context,
	input ListOfferingsInput,
) (ListOfferingsResult, error) {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return ListOfferingsResult{}, err
	}
	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return ListOfferingsResult{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_MODEL_CATALOG_READ,
	); err != nil {
		return ListOfferingsResult{}, err
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	offerings, err := service.repository.ListOfferings(
		ctx,
		ports.ListOfferingsParams{
			ActorUserID:     input.ActorUserID,
			TenantID:        input.TenantID,
			ConnectionID:    input.ConnectionID,
			AgentUsableOnly: input.AgentUsableOnly,
			Limit:           limit + 1,
			Cursor:          input.Cursor,
		},
	)
	if err != nil {
		return ListOfferingsResult{}, err
	}

	result := ListOfferingsResult{Offerings: offerings}
	if len(offerings) > limit {
		result.Offerings = offerings[:limit]
		last := result.Offerings[len(result.Offerings)-1]
		result.NextCursor = &ports.OfferingCursor{
			OfferingID: last.ID,
		}
	}

	return result, nil
}

// RefreshModelCatalogInput specifies the connection and actor for triggering a catalog refresh.
type RefreshModelCatalogInput struct {
	ActorUserID    string
	TenantID       string
	ConnectionID   string
	IdempotencyKey string
	Reason         string
}

// RefreshModelCatalog enqueues an asynchronous catalog refresh for a connection.
func (service *Service) RefreshModelCatalog(
	ctx context.Context,
	input RefreshModelCatalogInput,
) (domain.CatalogRefresh, error) {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return domain.CatalogRefresh{}, err
	}
	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return domain.CatalogRefresh{}, err
	}
	if err := validateUUID("connection_id", input.ConnectionID); err != nil {
		return domain.CatalogRefresh{}, err
	}
	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		return domain.CatalogRefresh{}, err
	}

	reason := input.Reason
	if reason == "" {
		reason = "manual"
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_MODEL_CATALOG_REFRESH,
	); err != nil {
		return domain.CatalogRefresh{}, err
	}

	connection, err := service.repository.GetConnection(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.ConnectionID,
	)
	if err != nil {
		return domain.CatalogRefresh{}, err
	}

	if connection.Status != domain.ConnectionStatusActive {
		return domain.CatalogRefresh{}, domain.ErrConnectionArchived
	}

	now := service.now().UTC()
	return service.repository.EnqueueCatalogRefresh(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.ConnectionID,
		input.IdempotencyKey,
		reason,
		now,
	)
}

// GetModelCatalogRefresh retrieves the status of a catalog refresh job.
func (service *Service) GetModelCatalogRefresh(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	refreshID string,
) (domain.CatalogRefresh, error) {
	if err := validateUUID("actor_user_id", actorUserID); err != nil {
		return domain.CatalogRefresh{}, err
	}
	if err := validateUUID("tenant_id", tenantID); err != nil {
		return domain.CatalogRefresh{}, err
	}
	if err := validateUUID("refresh_id", refreshID); err != nil {
		return domain.CatalogRefresh{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_MODEL_CATALOG_READ,
	); err != nil {
		return domain.CatalogRefresh{}, err
	}

	return service.repository.GetCatalogRefresh(
		ctx,
		actorUserID,
		tenantID,
		refreshID,
	)
}

// ExecuteCatalogRefresh performs provider/static discovery and updates offerings in the database.
func (service *Service) ExecuteCatalogRefresh(
	ctx context.Context,
	job domain.CatalogRefreshJob,
) error {
	connection, err := service.repository.GetConnection(
		ctx,
		job.ActorUserID,
		job.TenantID,
		job.ConnectionID,
	)
	if err != nil {
		return err
	}

	if connection.Status != domain.ConnectionStatusActive {
		// Connection no longer active; mark offerings unavailable and fail refresh
		now := service.now().UTC()
		_ = service.repository.MarkConnectionOfferingsUnavailable(
			ctx,
			job.ActorUserID,
			job.TenantID,
			job.ConnectionID,
			now,
		)
		return service.repository.FailCatalogRefresh(
			ctx,
			job.RefreshID,
			job.TenantID,
			job.ActorUserID,
			job.ConnectionID,
			"connection_inactive",
			now,
		)
	}

	now := service.now().UTC()
	var models []domain.DiscoveredModel
	var source domain.OfferingSource

	if connection.ConnectionType == domain.ConnectionTypePlatformManaged {
		source = domain.OfferingSourcePlatformCatalog
		if service.staticCatalog == nil {
			return service.repository.FailCatalogRefresh(
				ctx,
				job.RefreshID,
				job.TenantID,
				job.ActorUserID,
				job.ConnectionID,
				"static_catalog_unavailable",
				now,
			)
		}
		poolKey := "global"
		if connection.ProviderPoolKey != nil && *connection.ProviderPoolKey != "" {
			poolKey = *connection.ProviderPoolKey
		}
		discovered, err := service.staticCatalog.LoadPlatformOfferings(
			connection.ProviderKey,
			poolKey,
		)
		if err != nil {
			return service.repository.FailCatalogRefresh(
				ctx,
				job.RefreshID,
				job.TenantID,
				job.ActorUserID,
				job.ConnectionID,
				"static_catalog_load_failed",
				now,
			)
		}
		models = discovered
	} else {
		source = domain.OfferingSourceProviderDiscovered
		if service.catalogClient == nil {
			return service.repository.FailCatalogRefresh(
				ctx,
				job.RefreshID,
				job.TenantID,
				job.ActorUserID,
				job.ConnectionID,
				"catalog_client_unavailable",
				now,
			)
		}

		credential, err := service.repository.GetBYOKCredential(
			ctx,
			job.ActorUserID,
			job.TenantID,
			job.ConnectionID,
		)
		if err != nil {
			return err
		}

		if credential.State != domain.CredentialStateActive {
			return service.repository.FailCatalogRefresh(
				ctx,
				job.RefreshID,
				job.TenantID,
				job.ActorUserID,
				job.ConnectionID,
				"credential_inactive",
				now,
			)
		}

		rawSecret, err := service.secretStore.ReadVersion(
			ctx,
			credential.SecretRef,
			credential.ActiveVaultVersion,
		)
		if err != nil {
			return err
		}
		defer zeroBytes(rawSecret)

		discovered, err := service.catalogClient.DiscoverModels(
			ctx,
			connection.ProviderKey,
			rawSecret,
		)
		if err != nil {
			return service.repository.FailCatalogRefresh(
				ctx,
				job.RefreshID,
				job.TenantID,
				job.ActorUserID,
				job.ConnectionID,
				fmt.Sprintf("discovery_failed: %v", err),
				now,
			)
		}
		models = discovered

		// Enrich discovered models with static catalog curated metadata if available
		if service.staticCatalog != nil {
			for i, m := range models {
				if curated := service.staticCatalog.FindCuratedMetadata(m.ProviderKey, m.ProviderModelID); curated != nil {
					if curated.DisplayName != "" {
						models[i].DisplayName = curated.DisplayName
					}
					if curated.Description != "" {
						models[i].Description = curated.Description
					}
					if len(curated.Capabilities) > 0 {
						models[i].Capabilities = curated.Capabilities
					}
					if len(curated.InputModalities) > 0 {
						models[i].InputModalities = curated.InputModalities
					}
					if len(curated.OutputModalities) > 0 {
						models[i].OutputModalities = curated.OutputModalities
					}
					if curated.ContextWindowTokens > 0 {
						models[i].ContextWindowTokens = curated.ContextWindowTokens
					}
					if curated.MaxOutputTokens > 0 {
						models[i].MaxOutputTokens = curated.MaxOutputTokens
					}
				}
			}
		}
	}

	result, err := service.repository.ApplyCatalogRefresh(
		ctx,
		ports.ApplyCatalogRefreshParams{
			ActorUserID:  job.ActorUserID,
			TenantID:     job.TenantID,
			ConnectionID: job.ConnectionID,
			RefreshID:    job.RefreshID,
			Discovered:   models,
			Source:       source,
			RefreshedAt:  now,
		},
	)
	if err != nil {
		return err
	}

	// Publish outbox event
	_ = service.publishCatalogRefreshedEvent(ctx, job, result)

	return nil
}

func (service *Service) publishCatalogRefreshedEvent(
	ctx context.Context,
	job domain.CatalogRefreshJob,
	result domain.CatalogRefreshResult,
) error {
	_, err := service.catalogRefreshedEvent(
		ctx,
		job.TenantID,
		&modelv1.ModelCatalogRefreshed{
			TenantId:          job.TenantID,
			ConnectionId:      job.ConnectionID,
			CatalogGeneration: result.Generation,
			DiscoveredCount:   int32(result.DiscoveredCount),
			AvailableCount:    int32(result.AvailableCount),
			UnavailableCount:  int32(result.UnavailableCount),
			RefreshedAt:       timestamppb.New(result.RefreshedAt),
		},
		result.RefreshedAt,
	)
	return err
}
