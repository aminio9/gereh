package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func scanCredential(
	row rowScanner,
) (domain.BYOKCredential, error) {
	var result domain.BYOKCredential

	var pendingFingerprintDisplay *string

	err := row.Scan(
		&result.TenantID,
		&result.ConnectionID,
		&result.SecretRef,
		&result.Fingerprint,
		&result.FingerprintDisplay,
		&result.FingerprintKeyID,
		&result.State,
		&result.ActiveVaultVersion,
		&result.VaultLatestVersion,
		&result.PendingVaultVersion,
		&result.PendingFingerprint,
		&pendingFingerprintDisplay,
		&result.CredentialVersion,
		&result.VerifiedAt,
		&result.DestroyedAt,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		return domain.BYOKCredential{},
			mapDatabaseError(err)
	}

	if pendingFingerprintDisplay != nil {
		result.PendingFingerprintDisplay = *pendingFingerprintDisplay
	}

	return result, nil
}

const credentialColumns = `
	tenant_id::text,
	connection_id::text,
	secret_ref,
	credential_fingerprint,
	fingerprint_display,
	fingerprint_key_id,
	state,
	active_vault_version,
	vault_latest_version,
	pending_vault_version,
	pending_fingerprint,
	pending_fingerprint_display,
	credential_version,
	verified_at,
	destroyed_at,
	created_at,
	updated_at
`

// EnsureBYOKCredential creates credential metadata for a BYOK connection.
func (
	repository *Repository,
) EnsureBYOKCredential(
	ctx context.Context,
	params ports.EnsureBYOKCredentialParams,
) (domain.BYOKCredential, error) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			params.TenantID,
			params.ActorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return domain.BYOKCredential{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var connectionType string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT connection_type
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
			FOR UPDATE
		`,
		params.TenantID,
		params.ConnectionID,
	).Scan(
		&connectionType,
	)
	if errors.Is(
		err,
		pgx.ErrNoRows,
	) {
		return domain.BYOKCredential{},
			domain.ErrNotFound
	}

	if err != nil {
		return domain.BYOKCredential{},
			err
	}

	if domain.ConnectionType(
		connectionType,
	) != domain.ConnectionTypeBYOK {
		return domain.BYOKCredential{},
			domain.ErrUnsupportedConnectionType
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO
				model_access_connection_credentials (
					tenant_id,
					connection_id,
					secret_ref,
					credential_fingerprint,
					fingerprint_display,
					fingerprint_key_id,
					state,
					active_vault_version,
					vault_latest_version,
					pending_vault_version,
					pending_fingerprint,
					pending_fingerprint_display,
					credential_version,
					created_at,
					updated_at
				)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4,
				$5,
				$6,
				'pending_store',
				0,
				0,
				0,
				$4,
				$5,
				1,
				$7,
				$7
			)
			ON CONFLICT (
				tenant_id,
				connection_id
			)
			DO NOTHING
		`,
		params.TenantID,
		params.ConnectionID,
		params.SecretRef,
		params.Fingerprint,
		params.FingerprintDisplay,
		params.FingerprintKeyID,
		params.Now,
	)
	if err != nil {
		return domain.BYOKCredential{},
			mapDatabaseError(err)
	}

	credential, err :=
		scanCredential(
			transaction.QueryRow(
				ctx,
				`
					SELECT `+
					credentialColumns+
					`
					FROM
						model_access_connection_credentials
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
					FOR UPDATE
				`,
				params.TenantID,
				params.ConnectionID,
			),
		)
	if err != nil {
		return domain.BYOKCredential{},
			err
	}

	if !bytes.Equal(
		credential.Fingerprint,
		params.Fingerprint,
	) {
		return domain.BYOKCredential{},
			domain.ErrIdempotencyConflict
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return domain.BYOKCredential{},
			err
	}

	return credential, nil
}

// GetBYOKCredential returns BYOK credential metadata.
func (
	repository *Repository,
) GetBYOKCredential(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
) (domain.BYOKCredential, error) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{
				AccessMode: pgx.ReadOnly,
			},
		)
	if err != nil {
		return domain.BYOKCredential{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err :=
		scanCredential(
			transaction.QueryRow(
				ctx,
				`
					SELECT `+
					credentialColumns+
					`
					FROM
						model_access_connection_credentials
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
				`,
				tenantID,
				connectionID,
			),
		)
	if err != nil {
		return domain.BYOKCredential{},
			err
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return domain.BYOKCredential{},
			err
	}

	return result, nil
}

// MarkBYOKSecretStored records that the secret was persisted to Vault.
func (
	repository *Repository,
) MarkBYOKSecretStored(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
	vaultVersion int64,
	now time.Time,
) (domain.BYOKCredential, error) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return domain.BYOKCredential{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err :=
		scanCredential(
			transaction.QueryRow(
				ctx,
				`
					UPDATE
						model_access_connection_credentials
					SET
						state =
							'pending_verification',
						vault_latest_version =
							GREATEST(
								vault_latest_version,
								$3
							),
						pending_vault_version =
							$3,
						credential_version =
							credential_version + 1,
						updated_at =
							$4
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
					RETURNING `+
					credentialColumns,
				tenantID,
				connectionID,
				vaultVersion,
				now,
			),
		)
	if err != nil {
		return domain.BYOKCredential{},
			err
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return domain.BYOKCredential{},
			err
	}

	return result, nil
}

// insertVerification appends an immutable credential verification event.
func insertVerification(
	ctx context.Context,
	transaction pgx.Tx,
	value domain.CredentialVerification,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO
				model_access_connection_verification_events (
					tenant_id,
					verification_event_id,
					connection_id,
					actor_user_id,
					operation,
					outcome,
					reason_code,
					provider_http_status,
					fingerprint_display,
					request_id,
					correlation_id,
					occurred_at
				)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11,
				$12
			)
		`,
		value.TenantID,
		value.EventID,
		value.ConnectionID,
		value.ActorUserID,
		value.Operation,
		string(value.Outcome),
		value.ReasonCode,
		value.ProviderHTTPStatus,
		value.FingerprintDisplay,
		value.RequestID,
		value.CorrelationID,
		value.OccurredAt,
	)

	if err != nil {
		return fmt.Errorf(
			"insert credential verification event: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}

// insertSecretCleanup enqueues a durable secret-store operation.
func insertSecretCleanup(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	secretRef string,
	version *int64,
	action string,
	now time.Time,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO
				model_access_secret_cleanup (
					tenant_id,
					secret_ref,
					secret_version,
					action,
					created_at
				)
			VALUES (
				$1::uuid,
				$2,
				$3,
				$4,
				$5
			)
		`,
		tenantID,
		secretRef,
		version,
		action,
		now,
	)

	return err
}

// purgeArchivedBYOKSecret marks an archived BYOK credential destroyed and
// enqueues a full Vault purge.
func purgeArchivedBYOKSecret(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	connectionID string,
	now time.Time,
) error {
	var secretRef string

	err := transaction.QueryRow(
		ctx,
		`
			UPDATE
				model_access_connection_credentials
			SET
				state = 'destroyed',
				destroyed_at = $3,
				updated_at = $3,
				credential_version =
					credential_version + 1
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
			RETURNING secret_ref
		`,
		tenantID,
		connectionID,
		now,
	).Scan(&secretRef)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}

	if err != nil {
		return err
	}

	return insertSecretCleanup(
		ctx,
		transaction,
		tenantID,
		secretRef,
		nil,
		"purge_secret",
		now,
	)
}

// ActivateBYOK atomically promotes a verified BYOK credential.
func (
	repository *Repository,
) ActivateBYOK(
	ctx context.Context,
	params ports.ActivateBYOKParams,
) (domain.Connection, error) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			params.TenantID,
			params.ActorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return domain.Connection{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	_, err = scanCredential(
		transaction.QueryRow(
			ctx,
			`
				UPDATE
					model_access_connection_credentials
				SET
					state = 'active',
					credential_fingerprint =
						$3,
					fingerprint_display =
						$4,
					active_vault_version =
						$5,
					vault_latest_version =
						GREATEST(
							vault_latest_version,
							$5
						),
					pending_vault_version = 0,
					pending_fingerprint = NULL,
					pending_fingerprint_display =
						NULL,
					verified_at = $6,
					credential_version =
						credential_version + 1,
					updated_at = $6
				WHERE tenant_id =
					$1::uuid
				  AND connection_id =
					$2::uuid
				RETURNING `+
				credentialColumns,
			params.TenantID,
			params.ConnectionID,
			params.Fingerprint,
			params.FingerprintDisplay,
			params.VaultVersion,
			params.VerifiedAt,
		),
	)
	if err != nil {
		return domain.Connection{}, err
	}

	result, err :=
		scanConnection(
			transaction.QueryRow(
				ctx,
				`
					UPDATE
						model_access_connections
					SET
						status = 'active',
						credential_fingerprint =
							$3,
						version =
							version + 1,
						updated_at =
							$4
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
					  AND connection_type =
						'byok'
					  AND status IN (
						'pending_verification',
						'verification_failed'
					  )
					RETURNING
						`+connectionColumns,
				params.TenantID,
				params.ConnectionID,
				params.FingerprintDisplay,
				params.VerifiedAt,
			),
		)
	if err != nil {
		return domain.Connection{}, err
	}

	if err := insertRevision(
		ctx,
		transaction,
		result,
		params.ActorUserID,
		"updated",
	); err != nil {
		return domain.Connection{}, err
	}

	if err := insertVerification(
		ctx,
		transaction,
		params.Verification,
	); err != nil {
		return domain.Connection{}, err
	}

	event, err :=
		params.EventFactory(
			result,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	refreshUUID, _ := uuid.NewV7()
	refreshID := refreshUUID.String()

	_, _ = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_catalog_refreshes (
				tenant_id,
				refresh_id,
				actor_user_id,
				connection_id,
				status,
				generation,
				requested_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				'pending',
				1,
				$5
			)
		`,
		result.TenantID,
		refreshID,
		params.ActorUserID,
		result.ID,
		params.VerifiedAt,
	)

	_, _ = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_catalog_refresh_queue (
				tenant_id,
				refresh_id,
				connection_id,
				actor_user_id,
				available_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5
			)
		`,
		result.TenantID,
		refreshID,
		result.ID,
		params.ActorUserID,
		params.VerifiedAt,
	)

	if err := insertOutbox(
		ctx,
		transaction,
		result.TenantID,
		event,
	); err != nil {
		return domain.Connection{}, err
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return domain.Connection{}, err
	}

	return result, nil
}

// FailInitialBYOKVerification marks a rejected initial credential.
func (
	repository *Repository,
) FailInitialBYOKVerification(
	ctx context.Context,
	params ports.FailInitialBYOKParams,
) (domain.Connection, error) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			params.TenantID,
			params.ActorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return domain.Connection{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var secretRef string

	err = transaction.QueryRow(
		ctx,
		`
			UPDATE
				model_access_connection_credentials
			SET
				state =
					'verification_failed',
				vault_latest_version =
					GREATEST(
						vault_latest_version,
						$3
					),
				pending_vault_version = 0,
				pending_fingerprint = NULL,
				pending_fingerprint_display =
					NULL,
				credential_version =
					credential_version + 1,
				updated_at = $4
			WHERE tenant_id =
				$1::uuid
			  AND connection_id =
				$2::uuid
			RETURNING secret_ref
		`,
		params.TenantID,
		params.ConnectionID,
		params.VaultVersion,
		params.FailedAt,
	).Scan(
		&secretRef,
	)
	if err != nil {
		return domain.Connection{}, err
	}

	result, err :=
		scanConnection(
			transaction.QueryRow(
				ctx,
				`
					UPDATE
						model_access_connections
					SET
						status =
							'verification_failed',
						version =
							version + 1,
						updated_at =
							$3
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
					  AND connection_type =
						'byok'
					RETURNING
						`+connectionColumns,
				params.TenantID,
				params.ConnectionID,
				params.FailedAt,
			),
		)
	if err != nil {
		return domain.Connection{}, err
	}

	if err := insertRevision(
		ctx,
		transaction,
		result,
		params.ActorUserID,
		"updated",
	); err != nil {
		return domain.Connection{}, err
	}

	if err := insertVerification(
		ctx,
		transaction,
		params.Verification,
	); err != nil {
		return domain.Connection{}, err
	}

	version :=
		params.VaultVersion

	if err := insertSecretCleanup(
		ctx,
		transaction,
		params.TenantID,
		secretRef,
		&version,
		"destroy_version",
		params.FailedAt,
	); err != nil {
		return domain.Connection{}, err
	}

	event, err :=
		params.EventFactory(result)
	if err != nil {
		return domain.Connection{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		result.TenantID,
		event,
	); err != nil {
		return domain.Connection{}, err
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return domain.Connection{}, err
	}

	return result, nil
}

// RecordTransientVerification records a transient verification outcome.
func (
	repository *Repository,
) RecordTransientVerification(
	ctx context.Context,
	verification domain.CredentialVerification,
) error {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			verification.TenantID,
			verification.ActorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertVerification(
		ctx,
		transaction,
		verification,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}

func scanCredentialOperation(
	row rowScanner,
) (domain.CredentialOperation, error) {
	var result domain.CredentialOperation

	err := row.Scan(
		&result.TenantID,
		&result.ActorUserID,
		&result.Operation,
		&result.IdempotencyKey,
		&result.RequestHash,
		&result.ConnectionID,
		&result.State,
		&result.ResponseConnectionVersion,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.ExpiresAt,
	)
	if err != nil {
		return domain.CredentialOperation{},
			mapDatabaseError(err)
	}

	return result, nil
}

const credentialOperationColumns = `
	tenant_id::text,
	actor_user_id::text,
	operation,
	idempotency_key::text,
	request_hash,
	connection_id::text,
	state,
	response_connection_version,
	created_at,
	updated_at,
	expires_at
`

// PrepareBYOKRotation prepares a rotation with credential-operation idempotency.
func (
	repository *Repository,
) PrepareBYOKRotation(
	ctx context.Context,
	params ports.PrepareRotationParams,
) (ports.RotationPreparation, error) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			params.TenantID,
			params.ActorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return ports.RotationPreparation{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	lockKey := fmt.Sprintf(
		"%s:%s:rotate_byok_credential:%s",
		params.TenantID,
		params.ActorUserID,
		params.IdempotencyKey,
	)

	if _, err := transaction.Exec(
		ctx,
		`
			SELECT pg_advisory_xact_lock(
				hashtextextended(
					$1,
					0
				)
			)
		`,
		lockKey,
	); err != nil {
		return ports.RotationPreparation{}, err
	}

	_, err = transaction.Exec(
		ctx,
		`
			DELETE
			FROM model_access_credential_operations
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = 'rotate_byok_credential'
			  AND idempotency_key = $3::uuid
			  AND expires_at <= $4
		`,
		params.TenantID,
		params.ActorUserID,
		params.IdempotencyKey,
		params.Now,
	)
	if err != nil {
		return ports.RotationPreparation{}, err
	}

	var existingHash []byte

	err = transaction.QueryRow(
		ctx,
		`
			SELECT request_hash
			FROM model_access_credential_operations
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = 'rotate_byok_credential'
			  AND idempotency_key = $3::uuid
		`,
		params.TenantID,
		params.ActorUserID,
		params.IdempotencyKey,
	).Scan(&existingHash)

	replayed := false

	if err == nil {
		if !bytes.Equal(existingHash, params.RequestHash) {
			return ports.RotationPreparation{},
				domain.ErrIdempotencyConflict
		}

		replayed = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ports.RotationPreparation{}, err
	}

	connection, err :=
		scanConnection(
			transaction.QueryRow(
				ctx,
				`
					SELECT
						`+connectionColumns+`
					FROM model_access_connections
					WHERE tenant_id = $1::uuid
					  AND connection_id = $2::uuid
					  AND connection_type = 'byok'
					  AND status IN (
						'active',
						'verification_failed'
					  )
					FOR UPDATE
				`,
				params.TenantID,
				params.ConnectionID,
			),
		)
	if err != nil {
		return ports.RotationPreparation{}, err
	}

	if connection.Version != params.ExpectedVersion {
		return ports.RotationPreparation{},
			domain.ErrVersionConflict
	}

	credential, err :=
		scanCredential(
			transaction.QueryRow(
				ctx,
				`
					SELECT `+
					credentialColumns+
					`
					FROM
						model_access_connection_credentials
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
					FOR UPDATE
				`,
				params.TenantID,
				params.ConnectionID,
			),
		)
	if err != nil {
		return ports.RotationPreparation{}, err
	}

	if !replayed {
		if _, err := transaction.Exec(
			ctx,
			`
				INSERT INTO
					model_access_credential_operations (
						tenant_id,
						actor_user_id,
						operation,
						idempotency_key,
						request_hash,
						connection_id,
						state,
						created_at,
						updated_at,
						expires_at
					)
				VALUES (
					$1::uuid,
					$2::uuid,
					'rotate_byok_credential',
					$3::uuid,
					$4,
					$5::uuid,
					'prepared',
					$6,
					$6,
					$7
				)
			`,
			params.TenantID,
			params.ActorUserID,
			params.IdempotencyKey,
			params.RequestHash,
			params.ConnectionID,
			params.Now,
			params.ExpiresAt,
		); err != nil {
			return ports.RotationPreparation{}, err
		}
	}

	operation, err :=
		scanCredentialOperation(
			transaction.QueryRow(
				ctx,
				`
					SELECT `+
					credentialOperationColumns+
					`
					FROM model_access_credential_operations
					WHERE tenant_id = $1::uuid
					  AND actor_user_id = $2::uuid
					  AND operation = 'rotate_byok_credential'
					  AND idempotency_key = $3::uuid
					FOR UPDATE
				`,
				params.TenantID,
				params.ActorUserID,
				params.IdempotencyKey,
			),
		)
	if err != nil {
		return ports.RotationPreparation{}, err
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return ports.RotationPreparation{}, err
	}

	return ports.RotationPreparation{
		Connection: connection,
		Credential: credential,
		Operation:  operation,
	}, nil
}

// MarkBYOKRotationSecretStored records that the rotated secret is in Vault.
func (
	repository *Repository,
) MarkBYOKRotationSecretStored(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
	idempotencyKey string,
	vaultVersion int64,
	now time.Time,
) error {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if _, err := transaction.Exec(
		ctx,
		`
			UPDATE model_access_credential_operations
			SET
				state = 'secret_stored',
				updated_at = $4
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = 'rotate_byok_credential'
			  AND idempotency_key = $3::uuid
			  AND state IN ('prepared', 'secret_stored')
		`,
		tenantID,
		actorUserID,
		idempotencyKey,
		now,
	); err != nil {
		return err
	}

	if _, err := transaction.Exec(
		ctx,
		`
			UPDATE model_access_connection_credentials
			SET
				vault_latest_version =
					GREATEST(
						vault_latest_version,
						$3
					),
				pending_vault_version =
					$3,
				updated_at = $4
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
		`,
		tenantID,
		connectionID,
		vaultVersion,
		now,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}

// CompleteBYOKRotation atomically promotes a verified rotation.
func (
	repository *Repository,
) CompleteBYOKRotation(
	ctx context.Context,
	params ports.CompleteRotationParams,
) (domain.Connection, error) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			params.TenantID,
			params.ActorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return domain.Connection{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	credential, err :=
		scanCredential(
			transaction.QueryRow(
				ctx,
				`
					SELECT `+
					credentialColumns+
					`
					FROM
						model_access_connection_credentials
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
					FOR UPDATE
				`,
				params.TenantID,
				params.ConnectionID,
			),
		)
	if err != nil {
		return domain.Connection{}, err
	}

	oldActiveVersion :=
		credential.ActiveVaultVersion

	_, err = scanCredential(
		transaction.QueryRow(
			ctx,
			`
				UPDATE
					model_access_connection_credentials
				SET
					state = 'active',
					credential_fingerprint =
						$3,
					fingerprint_display =
						$4,
					active_vault_version =
						$5,
					vault_latest_version =
						GREATEST(
							vault_latest_version,
							$5
						),
					pending_vault_version = 0,
					pending_fingerprint = NULL,
					pending_fingerprint_display =
						NULL,
					verified_at = $6,
					credential_version =
						credential_version + 1,
					updated_at = $6
				WHERE tenant_id = $1::uuid
				  AND connection_id = $2::uuid
				RETURNING `+
				credentialColumns,
			params.TenantID,
			params.ConnectionID,
			params.NewFingerprint,
			params.NewFingerprintDisplay,
			params.NewVaultVersion,
			params.VerifiedAt,
		),
	)
	if err != nil {
		return domain.Connection{}, err
	}

	result, err :=
		scanConnection(
			transaction.QueryRow(
				ctx,
				`
					UPDATE
						model_access_connections
					SET
						status = 'active',
						credential_fingerprint =
							$3,
						version =
							version + 1,
						updated_at =
							$4
					WHERE tenant_id =
						$1::uuid
					  AND connection_id =
						$2::uuid
					  AND connection_type =
						'byok'
					RETURNING
						`+connectionColumns,
				params.TenantID,
				params.ConnectionID,
				params.NewFingerprintDisplay,
				params.VerifiedAt,
			),
		)
	if err != nil {
		return domain.Connection{}, err
	}

	if oldActiveVersion > 0 &&
		oldActiveVersion != params.NewVaultVersion {
		if err := insertSecretCleanup(
			ctx,
			transaction,
			params.TenantID,
			credential.SecretRef,
			&oldActiveVersion,
			"destroy_version",
			params.VerifiedAt,
		); err != nil {
			return domain.Connection{}, err
		}
	}

	if err := insertRevision(
		ctx,
		transaction,
		result,
		params.ActorUserID,
		"updated",
	); err != nil {
		return domain.Connection{}, err
	}

	if err := insertVerification(
		ctx,
		transaction,
		params.Verification,
	); err != nil {
		return domain.Connection{}, err
	}

	if _, err := transaction.Exec(
		ctx,
		`
			UPDATE model_access_credential_operations
			SET
				state = 'succeeded',
				response_connection_version = $4,
				updated_at = $3
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = 'rotate_byok_credential'
			  AND idempotency_key = $5::uuid
		`,
		params.TenantID,
		params.ActorUserID,
		params.VerifiedAt,
		result.Version,
		params.IdempotencyKey,
	); err != nil {
		return domain.Connection{}, err
	}

	event, err :=
		params.EventFactory(
			result,
		)
	if err != nil {
		return domain.Connection{}, err
	}

	refreshUUID, _ := uuid.NewV7()
	refreshID := refreshUUID.String()

	_, _ = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_catalog_refreshes (
				tenant_id,
				refresh_id,
				actor_user_id,
				connection_id,
				status,
				generation,
				requested_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				'pending',
				1,
				$5
			)
		`,
		result.TenantID,
		refreshID,
		params.ActorUserID,
		result.ID,
		params.VerifiedAt,
	)

	_, _ = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_catalog_refresh_queue (
				tenant_id,
				refresh_id,
				connection_id,
				actor_user_id,
				available_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5
			)
		`,
		result.TenantID,
		refreshID,
		result.ID,
		params.ActorUserID,
		params.VerifiedAt,
	)

	if err := insertOutbox(
		ctx,
		transaction,
		result.TenantID,
		event,
	); err != nil {
		return domain.Connection{}, err
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return domain.Connection{}, err
	}

	return result, nil
}

// RejectBYOKRotation records a rejected rotation without touching the active
// credential, and queues the rejected Vault version for destruction.
func (
	repository *Repository,
) RejectBYOKRotation(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
	idempotencyKey string,
	vaultVersion int64,
	verification domain.CredentialVerification,
	now time.Time,
) error {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{},
		)
	if err != nil {
		return err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var secretRef string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT secret_ref
			FROM model_access_connection_credentials
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
			FOR UPDATE
		`,
		tenantID,
		connectionID,
	).Scan(&secretRef)
	if err != nil {
		return err
	}

	if err := insertVerification(
		ctx,
		transaction,
		verification,
	); err != nil {
		return err
	}

	if err := insertSecretCleanup(
		ctx,
		transaction,
		tenantID,
		secretRef,
		&vaultVersion,
		"destroy_version",
		now,
	); err != nil {
		return err
	}

	if _, err := transaction.Exec(
		ctx,
		`
			UPDATE model_access_credential_operations
			SET
				state = 'rejected',
				updated_at = $3
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = 'rotate_byok_credential'
			  AND idempotency_key = $4::uuid
		`,
		tenantID,
		actorUserID,
		now,
		idempotencyKey,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}

// ClaimSecretCleanup claims due cleanup items for the worker lease window.
func (
	repository *Repository,
) ClaimSecretCleanup(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]domain.SecretCleanup, error) {
	rows, err := repository.pool.Query(
		ctx,
		`
			WITH candidates AS (
				SELECT cleanup_id
				FROM model_access_secret_cleanup
				WHERE completed_at IS NULL
				  AND available_at <= clock_timestamp()
				  AND (
					  claimed_at IS NULL
					  OR claimed_at <
						 clock_timestamp()
						 - $2::interval
				  )
				ORDER BY cleanup_id
				FOR UPDATE SKIP LOCKED
				LIMIT $1
			)
			UPDATE model_access_secret_cleanup AS cleanup
			SET
				claimed_at = clock_timestamp(),
				attempts = cleanup.attempts + 1
			FROM candidates
			WHERE cleanup.cleanup_id = candidates.cleanup_id
			RETURNING
				cleanup.cleanup_id,
				cleanup.tenant_id::text,
				cleanup.secret_ref,
				cleanup.secret_version,
				cleanup.action,
				cleanup.attempts
		`,
		limit,
		lease.String(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"claim Model Access secret cleanup: %w",
			err,
		)
	}
	defer rows.Close()

	result := make([]domain.SecretCleanup, 0, limit)

	for rows.Next() {
		var item domain.SecretCleanup

		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.SecretRef,
			&item.Version,
			&item.Action,
			&item.Attempts,
		); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

// CompleteSecretCleanup marks a cleanup item completed.
func (
	repository *Repository,
) CompleteSecretCleanup(
	ctx context.Context,
	cleanupID int64,
) error {
	_, err := repository.pool.Exec(
		ctx,
		`
			UPDATE model_access_secret_cleanup
			SET
				completed_at = clock_timestamp(),
				claimed_at = NULL,
				last_error = NULL
			WHERE cleanup_id = $1
		`,
		cleanupID,
	)
	if err != nil {
		return fmt.Errorf(
			"complete Model Access secret cleanup: %w",
			err,
		)
	}

	return nil
}

// ReleaseSecretCleanup re-queues a failed cleanup item with backoff.
func (
	repository *Repository,
) ReleaseSecretCleanup(
	ctx context.Context,
	cleanupID int64,
	retryAt time.Time,
	message string,
) error {
	_, err := repository.pool.Exec(
		ctx,
		`
			UPDATE model_access_secret_cleanup
			SET
				claimed_at = NULL,
				available_at = $2,
				last_error = left($3, 2000)
			WHERE cleanup_id = $1
			  AND completed_at IS NULL
		`,
		cleanupID,
		retryAt,
		message,
	)
	if err != nil {
		return fmt.Errorf(
			"release Model Access secret cleanup: %w",
			err,
		)
	}

	return nil
}
