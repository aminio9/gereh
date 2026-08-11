package postgres

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreateConnection atomically persists the connection, its first revision,
// the outbox event and the idempotency record.
func (repository *Repository) CreateConnection(
	ctx context.Context,
	params ports.CreateConnectionParams,
) (domain.Connection, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Connection.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Connection{}, err
	}

	defer func() { _ = transaction.Rollback(ctx) }()

	const operation = "create_connection"

	if err := lockIdempotency(
		ctx,
		transaction,
		params.Connection.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
	); err != nil {
		return domain.Connection{}, err
	}

	existing, found, err := lookupIdempotency(
		ctx,
		transaction,
		params.Connection.TenantID,
		params.ActorUserID,
		operation,
		params.IdempotencyKey,
		params.RequestHash,
		params.Connection.CreatedAt,
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

	if err := ensureProviderSupport(
		ctx,
		transaction,
		params.Connection.ProviderKey,
		params.Connection.ConnectionType,
	); err != nil {
		return domain.Connection{}, err
	}

	result, err := scanConnection(
		transaction.QueryRow(
			ctx,
			`
				INSERT INTO model_access_connections (
					tenant_id,
					connection_id,
					provider_key,
					connection_type,
					display_name,
					status,
					version,
					created_by_user_id,
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
					$7,
					$8::uuid,
					$9,
					$10
				)
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
			params.Connection.TenantID,
			params.Connection.ID,
			params.Connection.ProviderKey,
			string(params.Connection.ConnectionType),
			params.Connection.DisplayName,
			string(params.Connection.Status),
			params.Connection.Version,
			params.Connection.CreatedByUserID,
			params.Connection.CreatedAt,
			params.Connection.UpdatedAt,
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
		"created",
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

// GetConnection returns one connection for the tenant.
func (repository *Repository) GetConnection(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
) (domain.Connection, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	if err != nil {
		return domain.Connection{}, err
	}

	defer func() { _ = transaction.Rollback(ctx) }()

	result, err := scanConnection(
		transaction.QueryRow(
			ctx,
			`
				SELECT
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
				FROM model_access_connections
				WHERE tenant_id = $1::uuid
				  AND connection_id = $2::uuid
			`,
			tenantID,
			connectionID,
		),
	)
	if err != nil {
		return domain.Connection{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Connection{}, err
	}

	return result, nil
}

// ListConnections pages connections by connection ID.
func (repository *Repository) ListConnections(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	limit int,
	cursor *ports.ConnectionCursor,
	includeArchived bool,
) ([]domain.Connection, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	if err != nil {
		return nil, err
	}

	defer func() { _ = transaction.Rollback(ctx) }()

	var cursorID *string

	if cursor != nil {
		cursorID = &cursor.ConnectionID
	}

	rows, err := transaction.Query(
		ctx,
		`
			SELECT
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
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
			  AND ($2 OR status <> 'archived')
			  AND ($3::uuid IS NULL OR connection_id > $3::uuid)
			ORDER BY connection_id
			LIMIT $4
		`,
		tenantID,
		includeArchived,
		cursorID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list model connections: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Connection, 0, limit)

	for rows.Next() {
		value, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, value)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return result, nil
}
