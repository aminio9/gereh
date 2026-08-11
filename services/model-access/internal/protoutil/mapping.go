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
