package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/jackc/pgx/v5"
)

func scanConnection(row rowScanner) (domain.Connection, error) {
	var result domain.Connection

	var connectionType string
	var status string

	err := row.Scan(
		&result.TenantID,
		&result.ID,
		&result.ProviderKey,
		&connectionType,
		&result.DisplayName,
		&status,
		&result.Version,
		&result.CreatedByUserID,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.ArchivedAt,
	)
	if err != nil {
		return domain.Connection{}, mapDatabaseError(err)
	}

	result.ConnectionType = domain.ConnectionType(connectionType)
	result.Status = domain.ConnectionStatus(status)

	return result, nil
}

// ListProviders returns the enabled provider catalog in sort order.
func (repository *Repository) ListProviders(
	ctx context.Context,
	actorUserID string,
	tenantID string,
) ([]domain.Provider, error) {
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

	rows, err := transaction.Query(
		ctx,
		`
			SELECT
				provider_key,
				display_name,
				description,
				supported_connection_types,
				enabled,
				version,
				created_at,
				updated_at
			FROM model_access_providers
			WHERE enabled
			ORDER BY
				sort_order,
				provider_key
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list model providers: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Provider, 0)

	for rows.Next() {
		var provider domain.Provider

		var connectionTypes []string

		if err := rows.Scan(
			&provider.Key,
			&provider.DisplayName,
			&provider.Description,
			&connectionTypes,
			&provider.Enabled,
			&provider.Version,
			&provider.CreatedAt,
			&provider.UpdatedAt,
		); err != nil {
			return nil, err
		}

		for _, value := range connectionTypes {
			provider.SupportedConnectionTypes = append(
				provider.SupportedConnectionTypes,
				domain.ConnectionType(value),
			)
		}

		result = append(result, provider)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return result, nil
}

// ensureProviderSupport verifies the provider exists, is enabled and supports
// the requested connection type.
func ensureProviderSupport(
	ctx context.Context,
	transaction pgx.Tx,
	providerKey string,
	connectionType domain.ConnectionType,
) error {
	var enabled bool
	var supported bool

	err := transaction.QueryRow(
		ctx,
		`
			SELECT
				enabled,
				$2 = ANY(
					supported_connection_types
				)
			FROM model_access_providers
			WHERE provider_key = $1
		`,
		providerKey,
		string(connectionType),
	).Scan(&enabled, &supported)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrProviderNotFound
	}

	if err != nil {
		return fmt.Errorf("read model provider: %w", err)
	}

	if !enabled {
		return domain.ErrProviderDisabled
	}

	if !supported {
		return domain.ErrUnsupportedConnectionType
	}

	return nil
}

// insertRevision appends an immutable connection revision.
func insertRevision(
	ctx context.Context,
	transaction pgx.Tx,
	value domain.Connection,
	actorUserID string,
	changeKind string,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO
				model_access_connection_revisions (
					tenant_id,
					connection_id,
					revision,
					provider_key,
					connection_type,
					display_name,
					status,
					change_kind,
					changed_by_user_id,
					created_at
				)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9::uuid,
				$10
			)
		`,
		value.TenantID,
		value.ID,
		value.Version,
		value.ProviderKey,
		string(value.ConnectionType),
		value.DisplayName,
		string(value.Status),
		changeKind,
		actorUserID,
		value.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"insert model connection revision: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}
