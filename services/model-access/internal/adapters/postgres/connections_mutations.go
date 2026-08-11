package postgres

import (
	"context"
	"errors"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/jackc/pgx/v5"
)

// resolveMutationMiss distinguishes not-found, archived and version conflicts
// after an UPDATE affected no rows.
func resolveMutationMiss(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	connectionID string,
) error {
	var version int64
	var status string

	err := transaction.QueryRow(
		ctx,
		`
			SELECT
				version,
				status
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
		`,
		tenantID,
		connectionID,
	).Scan(&version, &status)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return err
	}

	if status == string(domain.ConnectionStatusArchived) {
		return domain.ErrConnectionArchived
	}

	return domain.ErrVersionConflict
}

// UpdateConnection renames a connection under optimistic concurrency.
func (repository *Repository) UpdateConnection(
	ctx context.Context,
	params ports.UpdateConnectionParams,
) (domain.Connection, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Connection{}, err
	}

	defer func() { _ = transaction.Rollback(ctx) }()

	const operation = "update_connection"

	if err := lockIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
	); err != nil {
		return domain.Connection{}, err
	}

	existing, found, err := lookupIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
		params.RequestHash,
		params.UpdatedAt,
	)
	if err != nil {
		return domain.Connection{}, err
	}

	if found {
		if err := commit(ctx, transaction); err != nil {
			return domain.Connection{}, err
		}

		return existing, nil
	}

	result, err := scanConnection(
		transaction.QueryRow(
			ctx,
			`
				UPDATE model_access_connections
				SET
					display_name = $4,
					version = version + 1,
					updated_at = $5
				WHERE tenant_id = $1::uuid
				  AND connection_id = $2::uuid
				  AND version = $3
				  AND status <> 'archived'
				RETURNING
					tenant_id::text,
					connection_id::text,
					provider_key,
					connection_type,
					display_name,
					status,
					version,
					created_by_user_id::text,
					created_at,
					updated_at,
					archived_at
			`,
			params.TenantID,
			params.ConnectionID,
			params.ExpectedVersion,
			params.DisplayName,
			params.UpdatedAt,
		),
	)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Connection{}, resolveMutationMiss(
			ctx,
			transaction,
			params.TenantID,
			params.ConnectionID,
		)
	}

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

	event, err := params.EventFactory(result)
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

	if err := storeIdempotency(
		ctx,
		transaction,
		result.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
		params.RequestHash,
		result,
		params.IdempotencyExpiresAt,
	); err != nil {
		return domain.Connection{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Connection{}, err
	}

	return result, nil
}

// ArchiveConnection archives a connection under optimistic concurrency.
func (repository *Repository) ArchiveConnection(
	ctx context.Context,
	params ports.ArchiveConnectionParams,
) (domain.Connection, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Connection{}, err
	}

	defer func() { _ = transaction.Rollback(ctx) }()

	const operation = "archive_connection"

	if err := lockIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
	); err != nil {
		return domain.Connection{}, err
	}

	existing, found, err := lookupIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
		params.RequestHash,
		params.ArchivedAt,
	)
	if err != nil {
		return domain.Connection{}, err
	}

	if found {
		if err := commit(ctx, transaction); err != nil {
			return domain.Connection{}, err
		}

		return existing, nil
	}

	result, err := scanConnection(
		transaction.QueryRow(
			ctx,
			`
				UPDATE model_access_connections
				SET
					status = 'archived',
					version = version + 1,
					updated_at = $4,
					archived_at = $4
				WHERE tenant_id = $1::uuid
				  AND connection_id = $2::uuid
				  AND version = $3
				  AND status <> 'archived'
				RETURNING
					tenant_id::text,
					connection_id::text,
					provider_key,
					connection_type,
					display_name,
					status,
					version,
					created_by_user_id::text,
					created_at,
					updated_at,
					archived_at
			`,
			params.TenantID,
			params.ConnectionID,
			params.ExpectedVersion,
			params.ArchivedAt,
		),
	)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Connection{}, resolveMutationMiss(
			ctx,
			transaction,
			params.TenantID,
			params.ConnectionID,
		)
	}

	if err != nil {
		return domain.Connection{}, err
	}

	if err := insertRevision(
		ctx,
		transaction,
		result,
		params.ActorUserID,
		"archived",
	); err != nil {
		return domain.Connection{}, err
	}

	event, err := params.EventFactory(result)
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

	if err := storeIdempotency(
		ctx,
		transaction,
		result.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
		params.RequestHash,
		result,
		params.IdempotencyExpiresAt,
	); err != nil {
		return domain.Connection{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Connection{}, err
	}

	return result, nil
}
