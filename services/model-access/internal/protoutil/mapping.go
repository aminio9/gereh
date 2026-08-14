// Package protoutil maps Model Access domain values to Protobuf.
package protoutil

import (
	"fmt"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConnectionType maps a domain connection type to Protobuf.
func ConnectionType(
	value domain.ConnectionType,
) modelv1.ModelConnectionType {
	switch value {
	case domain.ConnectionTypePlatformManaged:
		return modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_PLATFORM_MANAGED

	case domain.ConnectionTypeBYOK:
		return modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_BYOK

	case domain.ConnectionTypePrivateEndpoint:
		return modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_PRIVATE_ENDPOINT

	default:
		return modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_UNSPECIFIED
	}
}

// DomainConnectionType maps a Protobuf connection type to the domain.
func DomainConnectionType(
	value modelv1.ModelConnectionType,
) (domain.ConnectionType, error) {
	switch value {
	case modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_PLATFORM_MANAGED:
		return domain.ConnectionTypePlatformManaged, nil

	case modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_BYOK:
		return domain.ConnectionTypeBYOK, nil

	case modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_PRIVATE_ENDPOINT:
		return domain.ConnectionTypePrivateEndpoint, nil

	default:
		return "", fmt.Errorf(
			"%w: invalid connection type",
			domain.ErrInvalidArgument,
		)
	}
}

// ConnectionStatus maps a domain status to Protobuf.
func ConnectionStatus(
	value domain.ConnectionStatus,
) modelv1.ModelConnectionStatus {
	switch value {
	case domain.ConnectionStatusDraft:
		return modelv1.ModelConnectionStatus_MODEL_CONNECTION_STATUS_DRAFT

	case domain.ConnectionStatusPendingVerification:
		return modelv1.ModelConnectionStatus_MODEL_CONNECTION_STATUS_PENDING_VERIFICATION

	case domain.ConnectionStatusActive:
		return modelv1.ModelConnectionStatus_MODEL_CONNECTION_STATUS_ACTIVE

	case domain.ConnectionStatusVerificationFailed:
		return modelv1.ModelConnectionStatus_MODEL_CONNECTION_STATUS_VERIFICATION_FAILED

	case domain.ConnectionStatusDisabled:
		return modelv1.ModelConnectionStatus_MODEL_CONNECTION_STATUS_DISABLED

	case domain.ConnectionStatusArchived:
		return modelv1.ModelConnectionStatus_MODEL_CONNECTION_STATUS_ARCHIVED

	default:
		return modelv1.ModelConnectionStatus_MODEL_CONNECTION_STATUS_UNSPECIFIED
	}
}

// Connection maps a domain connection to Protobuf.
func Connection(
	value domain.Connection,
) *modelv1.ModelConnection {
	result := &modelv1.ModelConnection{
		TenantId:              value.TenantID,
		ConnectionId:          value.ID,
		ProviderKey:           value.ProviderKey,
		ConnectionType:        ConnectionType(value.ConnectionType),
		DisplayName:           value.DisplayName,
		Status:                ConnectionStatus(value.Status),
		CredentialFingerprint: value.CredentialFingerprint,
		Version:               value.Version,
		CreatedByUserId:       value.CreatedByUserID,
		CreatedAt:             timestamppb.New(value.CreatedAt),
		UpdatedAt:             timestamppb.New(value.UpdatedAt),
	}

	if value.ArchivedAt != nil {
		result.ArchivedAt = timestamppb.New(*value.ArchivedAt)
	}

	return result
}

// Provider maps a domain provider to Protobuf.
func Provider(
	value domain.Provider,
) *modelv1.ModelProvider {
	connectionTypes := make(
		[]modelv1.ModelConnectionType,
		0,
		len(value.SupportedConnectionTypes),
	)

	for _, item := range value.SupportedConnectionTypes {
		connectionTypes = append(connectionTypes, ConnectionType(item))
	}

	return &modelv1.ModelProvider{
		ProviderKey:              value.Key,
		DisplayName:              value.DisplayName,
		Description:              value.Description,
		SupportedConnectionTypes: connectionTypes,
		Enabled:                  value.Enabled,
	}
}

func OfferingStatus(value domain.OfferingStatus) modelv1.ModelOfferingStatus {
	switch value {
	case domain.OfferingStatusAvailable:
		return modelv1.ModelOfferingStatus_MODEL_OFFERING_STATUS_AVAILABLE
	case domain.OfferingStatusUnavailable:
		return modelv1.ModelOfferingStatus_MODEL_OFFERING_STATUS_UNAVAILABLE
	default:
		return modelv1.ModelOfferingStatus_MODEL_OFFERING_STATUS_UNSPECIFIED
	}
}

func OfferingSource(value domain.OfferingSource) modelv1.ModelOfferingSource {
	switch value {
	case domain.OfferingSourceProviderDiscovered:
		return modelv1.ModelOfferingSource_MODEL_OFFERING_SOURCE_PROVIDER_DISCOVERED
	case domain.OfferingSourcePlatformCatalog:
		return modelv1.ModelOfferingSource_MODEL_OFFERING_SOURCE_PLATFORM_CATALOG
	default:
		return modelv1.ModelOfferingSource_MODEL_OFFERING_SOURCE_UNSPECIFIED
	}
}

func ModelOffering(value domain.ModelOffering) *modelv1.ModelOffering {
	result := &modelv1.ModelOffering{
		TenantId:            value.TenantID,
		OfferingId:          value.ID,
		ConnectionId:        value.ConnectionID,
		ProviderKey:         value.ProviderKey,
		ProviderModelId:     value.ProviderModelID,
		DisplayName:         value.DisplayName,
		Description:         value.Description,
		Status:              OfferingStatus(value.Status),
		Source:              OfferingSource(value.Source),
		AgentUsable:         value.AgentUsable,
		Capabilities:        value.Capabilities,
		InputModalities:     value.InputModalities,
		OutputModalities:    value.OutputModalities,
		ContextWindowTokens: value.ContextWindowTokens,
		MaxOutputTokens:     value.MaxOutputTokens,
		Version:             value.Version,
		FirstSeenAt:         timestamppb.New(value.FirstSeenAt),
		LastSeenAt:          timestamppb.New(value.LastSeenAt),
		RefreshedAt:         timestamppb.New(value.RefreshedAt),
	}
	if value.UnavailableAt != nil {
		result.UnavailableAt = timestamppb.New(*value.UnavailableAt)
	}

	return result
}

func CatalogRefreshStatus(value domain.CatalogRefreshStatus) modelv1.ModelCatalogRefreshStatus {
	switch value {
	case domain.CatalogRefreshPending:
		return modelv1.ModelCatalogRefreshStatus_MODEL_CATALOG_REFRESH_STATUS_PENDING
	case domain.CatalogRefreshRunning:
		return modelv1.ModelCatalogRefreshStatus_MODEL_CATALOG_REFRESH_STATUS_RUNNING
	case domain.CatalogRefreshSucceeded:
		return modelv1.ModelCatalogRefreshStatus_MODEL_CATALOG_REFRESH_STATUS_SUCCEEDED
	case domain.CatalogRefreshFailed:
		return modelv1.ModelCatalogRefreshStatus_MODEL_CATALOG_REFRESH_STATUS_FAILED
	default:
		return modelv1.ModelCatalogRefreshStatus_MODEL_CATALOG_REFRESH_STATUS_UNSPECIFIED
	}
}

func ModelCatalogRefresh(value domain.CatalogRefresh) *modelv1.ModelCatalogRefresh {
	result := &modelv1.ModelCatalogRefresh{
		TenantId:         value.TenantID,
		RefreshId:        value.ID,
		ConnectionId:     value.ConnectionID,
		Status:           CatalogRefreshStatus(value.Status),
		CatalogGeneration: value.Generation,
		DiscoveredCount:  int32(value.DiscoveredCount),
		AvailableCount:   int32(value.AvailableCount),
		UnavailableCount: int32(value.UnavailableCount),
		ErrorCode:        value.ErrorCode,
		RequestedAt:      timestamppb.New(value.RequestedAt),
	}
	if value.StartedAt != nil {
		result.StartedAt = timestamppb.New(*value.StartedAt)
	}
	if value.CompletedAt != nil {
		result.CompletedAt = timestamppb.New(*value.CompletedAt)
	}

	return result
}

func BindingStatus(value domain.BindingStatus) modelv1.AgentModelBindingStatus {
	switch value {
	case domain.BindingStatusActive:
		return modelv1.AgentModelBindingStatus_AGENT_MODEL_BINDING_STATUS_ACTIVE
	case domain.BindingStatusRemoved:
		return modelv1.AgentModelBindingStatus_AGENT_MODEL_BINDING_STATUS_REMOVED
	default:
		return modelv1.AgentModelBindingStatus_AGENT_MODEL_BINDING_STATUS_UNSPECIFIED
	}
}

func FallbackPolicy(value domain.FallbackPolicy) modelv1.ModelFallbackPolicy {
	switch value {
	case domain.FallbackPolicyOrdered:
		return modelv1.ModelFallbackPolicy_MODEL_FALLBACK_POLICY_ORDERED
	case domain.FallbackPolicyNone:
		return modelv1.ModelFallbackPolicy_MODEL_FALLBACK_POLICY_NONE
	default:
		return modelv1.ModelFallbackPolicy_MODEL_FALLBACK_POLICY_UNSPECIFIED
	}
}

func DomainFallbackPolicy(value modelv1.ModelFallbackPolicy) domain.FallbackPolicy {
	switch value {
	case modelv1.ModelFallbackPolicy_MODEL_FALLBACK_POLICY_ORDERED:
		return domain.FallbackPolicyOrdered
	case modelv1.ModelFallbackPolicy_MODEL_FALLBACK_POLICY_NONE:
		return domain.FallbackPolicyNone
	default:
		return domain.FallbackPolicyNone
	}
}

func AgentModelBinding(value domain.AgentModelBinding) *modelv1.AgentModelBinding {
	result := &modelv1.AgentModelBinding{
		TenantId:             value.TenantID,
		AgentId:              value.AgentID,
		CompanyId:            value.CompanyID,
		Status:               BindingStatus(value.Status),
		PrimaryOfferingId:    value.PrimaryOfferingID,
		FallbackOfferingIds:  value.FallbackOfferingIDs,
		FallbackPolicy:       FallbackPolicy(value.FallbackPolicy),
		MaxModelCostMicrousd: value.MaxModelCostMicroUSD,
		Version:              value.Version,
		CreatedByUserId:      value.CreatedByUserID,
		UpdatedByUserId:      value.UpdatedByUserID,
		CreatedAt:             timestamppb.New(value.CreatedAt),
		UpdatedAt:             timestamppb.New(value.UpdatedAt),
	}
	if value.FastOfferingID != nil {
		result.FastOfferingId = value.FastOfferingID
	}
	if value.RemovedAt != nil {
		result.RemovedAt = timestamppb.New(*value.RemovedAt)
	}

	return result
}

