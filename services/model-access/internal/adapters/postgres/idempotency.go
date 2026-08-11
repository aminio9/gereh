package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/jackc/pgx/v5"
)

type connectionSnapshot struct {
	TenantID string `json:"tenantId"`

	ID string `json:"connectionId"`

	ProviderKey string `json:"providerKey"`

	ProviderPoolKey *string `json:"providerPoolKey,omitempty"`

	ConnectionType string `json:"connectionType"`

	DisplayName string `json:"displayName"`

	Status string `json:"status"`

	Version int64 `json:"version"`

	CreatedByUserID string `json:"createdByUserId"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	ArchivedAt *time.Time `json:"archivedAt,omitempty"`
}

func snapshotConnection(value domain.Connection) connectionSnapshot {
	return connectionSnapshot{
		TenantID:        value.TenantID,
		ID:              value.ID,
		ProviderKey:     value.ProviderKey,
		ProviderPoolKey: value.ProviderPoolKey,
		ConnectionType:  string(value.ConnectionType),
		DisplayName:     value.DisplayName,
		Status:          string(value.Status),
		Version:         value.Version,
		CreatedByUserID: value.CreatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
		ArchivedAt:      value.ArchivedAt,
	}
}

func domainConnection(value connectionSnapshot) domain.Connection {
	return domain.Connection{
		TenantID:        value.TenantID,
		ID:              value.ID,
		ProviderKey:     value.ProviderKey,
		ProviderPoolKey: value.ProviderPoolKey,
		ConnectionType:  domain.ConnectionType(value.ConnectionType),
		DisplayName:     value.DisplayName,
		Status:          domain.ConnectionStatus(value.Status),
		Version:         value.Version,
		CreatedByUserID: value.CreatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
		ArchivedAt:      value.ArchivedAt,
	}
}

// lockIdempotency serializes concurrent mutations for the same logical
// idempotency key inside the transaction.
func lockIdempotency(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	actorUserID string,
	operation string,
	idempotencyKey string,
) error {
	lockKey := fmt.Sprintf(
		"%s:%s:%s:%s",
		tenantID,
		actorUserID,
		operation,
		idempotencyKey,
	)

	_, err := transaction.Exec(
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
	)
	if err != nil {
		return fmt.Errorf(
			"acquire Model Access idempotency lock: %w",
			err,
		)
	}

	return nil
}

// lookupIdempotency returns the stored result when the key was already used
// with the same request hash. Expired keys are deleted first.
func lookupIdempotency(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	actorUserID string,
	operation string,
	idempotencyKey string,
	requestHash []byte,
	now time.Time,
) (domain.Connection, bool, error) {
	_, err := transaction.Exec(
		ctx,
		`
			DELETE
			FROM model_access_idempotency
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = $3
			  AND idempotency_key = $4::uuid
			  AND expires_at <= $5
		`,
		tenantID,
		actorUserID,
		operation,
		idempotencyKey,
		now,
	)
	if err != nil {
		return domain.Connection{}, false, fmt.Errorf(
			"expire Model Access idempotency record: %w",
			err,
		)
	}

	var existingHash []byte
	var rawSnapshot []byte

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				request_hash,
				response_snapshot
			FROM model_access_idempotency
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = $3
			  AND idempotency_key = $4::uuid
		`,
		tenantID,
		actorUserID,
		operation,
		idempotencyKey,
	).Scan(&existingHash, &rawSnapshot)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Connection{}, false, nil
	}

	if err != nil {
		return domain.Connection{}, false, fmt.Errorf(
			"read Model Access idempotency record: %w",
			err,
		)
	}

	if !bytes.Equal(existingHash, requestHash) {
		return domain.Connection{}, false, domain.ErrIdempotencyConflict
	}

	var snapshot connectionSnapshot

	if err := json.Unmarshal(rawSnapshot, &snapshot); err != nil {
		return domain.Connection{}, false, fmt.Errorf(
			"decode Model Access idempotency snapshot: %w",
			err,
		)
	}

	return domainConnection(snapshot), true, nil
}

// storeIdempotency records a committed mutation result.
func storeIdempotency(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	actorUserID string,
	operation string,
	idempotencyKey string,
	requestHash []byte,
	result domain.Connection,
	expiresAt time.Time,
) error {
	snapshotBytes, err := json.Marshal(snapshotConnection(result))
	if err != nil {
		return fmt.Errorf(
			"encode Model Access idempotency snapshot: %w",
			err,
		)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_idempotency (
				tenant_id,
				actor_user_id,
				operation,
				idempotency_key,
				request_hash,
				response_snapshot,
				created_at,
				expires_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4::uuid,
				$5,
				$6::jsonb,
				clock_timestamp(),
				$7
			)
		`,
		tenantID,
		actorUserID,
		operation,
		idempotencyKey,
		requestHash,
		snapshotBytes,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf(
			"store Model Access idempotency record: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}
