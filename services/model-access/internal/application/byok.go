package application

import (
	"context"
	"errors"
	"fmt"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/aminio9/gereh/services/model-access/internal/protoutil"
	"github.com/aminio9/gereh/services/model-access/internal/security"
	"github.com/google/uuid"
)

// CreateBYOKConnectionInput is a BYOK create command.
type CreateBYOKConnectionInput struct {
	ActorUserID string
	TenantID    string

	IdempotencyKey string

	ProviderKey string
	DisplayName string

	APIKey string
}

// RotateBYOKCredentialInput is a credential rotation command.
type RotateBYOKCredentialInput struct {
	ActorUserID string
	TenantID    string

	ConnectionID string

	IdempotencyKey string

	ExpectedVersion int64

	APIKey string
}

// CreateBYOKConnection stores, verifies and activates a BYOK credential.
func (
	service *Service,
) CreateBYOKConnection(
	ctx context.Context,
	input CreateBYOKConnectionInput,
) (
	domain.Connection,
	error,
) {
	if err :=
		service.authorizer.Require(
			ctx,
			input.ActorUserID,
			input.TenantID,
			tenantv1.Permission_PERMISSION_MODEL_CONNECTION_CREATE,
		); err != nil {
		return domain.Connection{}, err
	}

	rawCredential, err :=
		normalizeCredential(
			input.APIKey,
		)
	if err != nil {
		return domain.Connection{}, err
	}
	defer zeroBytes(rawCredential)

	fingerprint :=
		service.fingerprinter.Sum(
			rawCredential,
		)

	// The generic connection idempotency hash deliberately excludes
	// the secret. The credential metadata check below compares its HMAC
	// fingerprint, so reusing the same Idempotency-Key with a different
	// API key is rejected without persisting the raw key.
	connection, err :=
		service.createBYOKMetadata(
			ctx,
			input,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	// The generic create idempotency snapshot may predate activation.
	// Always re-read authoritative aggregate state.
	connection, err =
		service.repository.GetConnection(
			ctx,
			input.ActorUserID,
			input.TenantID,
			connection.ID,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	secretRef :=
		service.secretStore.Reference(
			input.TenantID,
			connection.ID,
		)

	now :=
		service.now().UTC()

	credential, err :=
		service.repository.
			EnsureBYOKCredential(
				ctx,
				ports.EnsureBYOKCredentialParams{
					ActorUserID:        input.ActorUserID,
					TenantID:           input.TenantID,
					ConnectionID:       connection.ID,
					SecretRef:          secretRef,
					Fingerprint:        fingerprint.Full,
					FingerprintDisplay: fingerprint.Display,
					FingerprintKeyID:   fingerprint.KeyID,
					Now:                now,
				},
			)
	if err != nil {
		return domain.Connection{}, err
	}

	// A replay after full success returns the final sanitized aggregate.
	if credential.State ==
		domain.CredentialStateActive &&
		security.Equal(
			credential.Fingerprint,
			fingerprint.Full,
		) {
		return service.repository.
			GetConnection(
				ctx,
				input.ActorUserID,
				input.TenantID,
				connection.ID,
			)
	}

	if credential.State ==
		domain.CredentialStateVerificationFailed {
		return domain.Connection{},
			domain.ErrCredentialRejected
	}

	vaultVersion, err :=
		service.writeCredentialRecoverably(
			ctx,
			credential,
			rawCredential,
			fingerprint.Full,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	credential, err =
		service.repository.
			MarkBYOKSecretStored(
				ctx,
				input.ActorUserID,
				input.TenantID,
				connection.ID,
				vaultVersion,
				service.now().UTC(),
			)
	if err != nil {
		return domain.Connection{}, err
	}

	// Verify what Vault holds, not the original request buffer.
	storedCredential, err :=
		service.secretStore.ReadVersion(
			ctx,
			secretRef,
			vaultVersion,
		)
	if err != nil {
		return domain.Connection{}, err
	}
	defer zeroBytes(storedCredential)

	verification, verifyErr :=
		service.verifier.Verify(
			ctx,
			connection.ProviderKey,
			storedCredential,
		)

	event :=
		service.newCredentialVerification(
			ctx,
			input.ActorUserID,
			input.TenantID,
			connection.ID,
			"create",
			fingerprint.Display,
			verification.HTTPStatus,
			verifyErr,
		)

	switch {
	case verifyErr == nil:
		activatedAt :=
			service.now().UTC()

		return service.repository.
			ActivateBYOK(
				ctx,
				ports.ActivateBYOKParams{
					ActorUserID:        input.ActorUserID,
					TenantID:           input.TenantID,
					ConnectionID:       connection.ID,
					VaultVersion:       vaultVersion,
					Fingerprint:        fingerprint.Full,
					FingerprintDisplay: fingerprint.Display,
					VerifiedAt:         activatedAt,
					Verification:       event,
					EventFactory: func(
						result domain.Connection,
					) (
						domain.OutboxEvent,
						error,
					) {
						return service.connectionEvent(
							ctx,
							"model.connection.activated",
							result,
							&modelv1.ModelConnectionActivated{
								Connection:        protoutil.Connection(result),
								ActivatedByUserId: input.ActorUserID,
							},
							activatedAt,
						)
					},
				},
			)

	case errors.Is(
		verifyErr,
		domain.ErrCredentialRejected,
	):
		failedAt :=
			service.now().UTC()

		_, persistErr :=
			service.repository.
				FailInitialBYOKVerification(
					ctx,
					ports.FailInitialBYOKParams{
						ActorUserID:  input.ActorUserID,
						TenantID:     input.TenantID,
						ConnectionID: connection.ID,
						VaultVersion: vaultVersion,
						Verification: event,
						FailedAt:     failedAt,
						EventFactory: func(
							result domain.Connection,
						) (
							domain.OutboxEvent,
							error,
						) {
							return service.connectionEvent(
								ctx,
								"model.connection.verification_failed",
								result,
								&modelv1.ModelConnectionVerificationFailed{
									Connection:       protoutil.Connection(result),
									VerifiedByUserId: input.ActorUserID,
									ReasonCode:       event.ReasonCode,
								},
								failedAt,
							)
						},
					},
				)
		if persistErr != nil {
			return domain.Connection{},
				persistErr
		}

		return domain.Connection{},
			domain.ErrCredentialRejected

	default:
		_ =
			service.repository.
				RecordTransientVerification(
					ctx,
					event,
				)

		return domain.Connection{},
			verifyErr
	}
}

// createBYOKMetadata creates the PENDING_VERIFICATION BYOK connection.
func (
	service *Service,
) createBYOKMetadata(
	ctx context.Context,
	input CreateBYOKConnectionInput,
) (
	domain.Connection,
	error,
) {
	if err :=
		validateUUID(
			"actor_user_id",
			input.ActorUserID,
		); err != nil {
		return domain.Connection{}, err
	}

	if err :=
		validateUUID(
			"tenant_id",
			input.TenantID,
		); err != nil {
		return domain.Connection{}, err
	}

	if err :=
		validateIdempotencyKey(
			input.IdempotencyKey,
		); err != nil {
		return domain.Connection{}, err
	}

	providerKey, err :=
		normalizeProviderKey(
			input.ProviderKey,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	displayName, err :=
		normalizeDisplayName(
			input.DisplayName,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	requestHash, err :=
		hashCanonical(
			createFingerprint{
				ProviderKey:    providerKey,
				ConnectionType: string(domain.ConnectionTypeBYOK),
				DisplayName:    displayName,
			},
		)
	if err != nil {
		return domain.Connection{}, err
	}

	connectionID, err :=
		uuid.NewV7()
	if err != nil {
		return domain.Connection{},
			fmt.Errorf(
				"generate model connection ID: %w",
				err,
			)
	}

	now :=
		service.now().UTC()

	return service.repository.
		CreateConnection(
			ctx,
			ports.CreateConnectionParams{
				ActorUserID: input.ActorUserID,

				Connection: domain.Connection{
					TenantID:        input.TenantID,
					ID:              connectionID.String(),
					ProviderKey:     providerKey,
					ConnectionType:  domain.ConnectionTypeBYOK,
					DisplayName:     displayName,
					Status:          domain.ConnectionStatusPendingVerification,
					Version:         1,
					CreatedByUserID: input.ActorUserID,
					CreatedAt:       now,
					UpdatedAt:       now,
				},

				IdempotencyKey:       input.IdempotencyKey,
				RequestHash:          requestHash,
				IdempotencyExpiresAt: now.Add(service.config.IdempotencyTTL),

				EventFactory: func(
					result domain.Connection,
				) (
					domain.OutboxEvent,
					error,
				) {
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

// writeCredentialRecoverably writes a credential to the secret store,
// recovering from a lost success via Vault CAS versioning.
func (
	service *Service,
) writeCredentialRecoverably(
	ctx context.Context,
	credential domain.BYOKCredential,
	rawCredential []byte,
	expectedFingerprint []byte,
) (int64, error) {
	version, err :=
		service.secretStore.
			WriteCAS(
				ctx,
				credential.SecretRef,
				rawCredential,
				credential.VaultLatestVersion,
			)

	if err == nil {
		return version, nil
	}

	if !errors.Is(
		err,
		domain.ErrSecretStoreConflict,
	) {
		return 0, err
	}

	// Possible crash-retry:
	// Vault committed a version but PostgreSQL did not record it.
	currentVersion, err :=
		service.secretStore.
			CurrentVersion(
				ctx,
				credential.SecretRef,
			)
	if err != nil {
		return 0, err
	}

	if currentVersion <=
		credential.VaultLatestVersion {
		return 0,
			domain.ErrSecretStoreConflict
	}

	stored, err :=
		service.secretStore.
			ReadVersion(
				ctx,
				credential.SecretRef,
				currentVersion,
			)
	if err != nil {
		return 0, err
	}
	defer zeroBytes(stored)

	actual :=
		service.fingerprinter.
			Sum(stored)

	if !security.Equal(
		actual.Full,
		expectedFingerprint,
	) {
		return 0,
			domain.ErrIdempotencyConflict
	}

	return currentVersion, nil
}

// newCredentialVerification builds an immutable audit event. It never
// captures the provider response body or raw credential.
func (
	service *Service,
) newCredentialVerification(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
	operation string,
	fingerprint string,
	httpStatus int,
	verifyErr error,
) domain.CredentialVerification {
	eventID, _ :=
		uuid.NewV7()

	outcome :=
		domain.VerificationSucceeded

	reasonCode :=
		"credential_valid"

	switch {
	case errors.Is(
		verifyErr,
		domain.ErrCredentialRejected,
	):
		outcome =
			domain.VerificationRejected

		reasonCode =
			"provider_rejected"

	case verifyErr != nil:
		outcome =
			domain.VerificationTransientFailure

		reasonCode =
			"provider_unavailable"
	}

	var providerStatus *int

	if httpStatus > 0 {
		providerStatus =
			&httpStatus
	}

	requestID := ""
	correlationID := ""

	if metadata, ok :=
		grpcx.RequestMetadataFromContext(
			ctx,
		); ok {
		requestID =
			metadata.RequestID

		correlationID =
			metadata.CorrelationID
	}

	return domain.CredentialVerification{
		EventID:            eventID.String(),
		TenantID:           tenantID,
		ConnectionID:       connectionID,
		ActorUserID:        actorUserID,
		Operation:          operation,
		Outcome:            outcome,
		ReasonCode:         reasonCode,
		ProviderHTTPStatus: providerStatus,
		FingerprintDisplay: fingerprint,
		RequestID:          requestID,
		CorrelationID:      correlationID,
		OccurredAt:         service.now().UTC(),
	}
}

// RotateBYOKCredential rotates a BYOK secret without changing the
// connection identity.
func (
	service *Service,
) RotateBYOKCredential(
	ctx context.Context,
	input RotateBYOKCredentialInput,
) (
	domain.Connection,
	error,
) {
	if err :=
		service.authorizer.Require(
			ctx,
			input.ActorUserID,
			input.TenantID,
			tenantv1.Permission_PERMISSION_MODEL_CONNECTION_UPDATE,
		); err != nil {
		return domain.Connection{}, err
	}

	if input.ExpectedVersion <= 0 {
		return domain.Connection{},
			domain.ErrInvalidArgument
	}

	if err :=
		validateUUID(
			"connection_id",
			input.ConnectionID,
		); err != nil {
		return domain.Connection{}, err
	}

	if err :=
		validateIdempotencyKey(
			input.IdempotencyKey,
		); err != nil {
		return domain.Connection{}, err
	}

	rawCredential, err :=
		normalizeCredential(
			input.APIKey,
		)
	if err != nil {
		return domain.Connection{}, err
	}
	defer zeroBytes(rawCredential)

	fingerprint :=
		service.fingerprinter.Sum(
			rawCredential,
		)

	requestHash, err :=
		hashCanonical(
			struct {
				ConnectionID    string `json:"connectionId"`
				ExpectedVersion int64  `json:"expectedVersion"`
				Fingerprint     string `json:"fingerprint"`
			}{
				ConnectionID:    input.ConnectionID,
				ExpectedVersion: input.ExpectedVersion,
				Fingerprint:     fingerprint.Display,
			},
		)
	if err != nil {
		return domain.Connection{}, err
	}

	now :=
		service.now().UTC()

	preparation, err :=
		service.repository.
			PrepareBYOKRotation(
				ctx,
				ports.PrepareRotationParams{
					ActorUserID:           input.ActorUserID,
					TenantID:              input.TenantID,
					ConnectionID:          input.ConnectionID,
					ExpectedVersion:       input.ExpectedVersion,
					IdempotencyKey:        input.IdempotencyKey,
					RequestHash:           requestHash,
					NewFingerprint:        fingerprint.Full,
					NewFingerprintDisplay: fingerprint.Display,
					Now:                   now,
					ExpiresAt:             now.Add(service.config.IdempotencyTTL),
				},
			)
	if err != nil {
		return domain.Connection{}, err
	}

	switch preparation.Operation.State {
	case domain.CredentialOperationSucceeded:
		return service.repository.
			GetConnection(
				ctx,
				input.ActorUserID,
				input.TenantID,
				input.ConnectionID,
			)

	case domain.CredentialOperationRejected:
		return domain.Connection{},
			domain.ErrCredentialRejected
	}

	vaultVersion, err :=
		service.writeCredentialRecoverably(
			ctx,
			preparation.Credential,
			rawCredential,
			fingerprint.Full,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	if err :=
		service.repository.
			MarkBYOKRotationSecretStored(
				ctx,
				input.ActorUserID,
				input.TenantID,
				input.ConnectionID,
				input.IdempotencyKey,
				vaultVersion,
				service.now().UTC(),
			); err != nil {
		return domain.Connection{}, err
	}

	storedCredential, err :=
		service.secretStore.
			ReadVersion(
				ctx,
				preparation.Credential.SecretRef,
				vaultVersion,
			)
	if err != nil {
		return domain.Connection{}, err
	}
	defer zeroBytes(storedCredential)

	verification, verifyErr :=
		service.verifier.Verify(
			ctx,
			preparation.Connection.ProviderKey,
			storedCredential,
		)

	verificationEvent :=
		service.newCredentialVerification(
			ctx,
			input.ActorUserID,
			input.TenantID,
			input.ConnectionID,
			"rotate",
			fingerprint.Display,
			verification.HTTPStatus,
			verifyErr,
		)

	if errors.Is(
		verifyErr,
		domain.ErrCredentialRejected,
	) {
		if err :=
			service.repository.
				RejectBYOKRotation(
					ctx,
					input.ActorUserID,
					input.TenantID,
					input.ConnectionID,
					input.IdempotencyKey,
					vaultVersion,
					verificationEvent,
					service.now().UTC(),
				); err != nil {
			return domain.Connection{}, err
		}

		return domain.Connection{},
			domain.ErrCredentialRejected
	}

	if verifyErr != nil {
		_ =
			service.repository.
				RecordTransientVerification(
					ctx,
					verificationEvent,
				)

		return domain.Connection{},
			verifyErr
	}

	rotatedAt :=
		service.now().UTC()

	return service.repository.
		CompleteBYOKRotation(
			ctx,
			ports.CompleteRotationParams{
				ActorUserID:           input.ActorUserID,
				TenantID:              input.TenantID,
				ConnectionID:          input.ConnectionID,
				IdempotencyKey:        input.IdempotencyKey,
				NewVaultVersion:       vaultVersion,
				NewFingerprint:        fingerprint.Full,
				NewFingerprintDisplay: fingerprint.Display,
				VerifiedAt:            rotatedAt,
				Verification:          verificationEvent,
				EventFactory: func(
					result domain.Connection,
				) (
					domain.OutboxEvent,
					error,
				) {
					return service.connectionEvent(
						ctx,
						"model.connection.credential_rotated",
						result,
						&modelv1.ModelConnectionCredentialRotated{
							Connection:      protoutil.Connection(result),
							RotatedByUserId: input.ActorUserID,
						},
						rotatedAt,
					)
				},
			},
		)
}
