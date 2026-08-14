package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const offeringColumns = `
	tenant_id::text,
	offering_id::text,
	connection_id::text,
	provider_key,
	provider_model_id,
	display_name,
	description,
	status,
	source,
	agent_usable,
	capabilities,
	input_modalities,
	output_modalities,
	context_window_tokens,
	max_output_tokens,
	version,
	first_seen_at,
	last_seen_at,
	refreshed_at,
	unavailable_at
`

func scanOffering(row rowScanner) (domain.ModelOffering, error) {
	var result domain.ModelOffering
	var status string
	var source string

	err := row.Scan(
		&result.TenantID,
		&result.ID,
		&result.ConnectionID,
		&result.ProviderKey,
		&result.ProviderModelID,
		&result.DisplayName,
		&result.Description,
		&status,
		&source,
		&result.AgentUsable,
		&result.Capabilities,
		&result.InputModalities,
		&result.OutputModalities,
		&result.ContextWindowTokens,
		&result.MaxOutputTokens,
		&result.Version,
		&result.FirstSeenAt,
		&result.LastSeenAt,
		&result.RefreshedAt,
		&result.UnavailableAt,
	)
	if err != nil {
		return domain.ModelOffering{}, mapDatabaseError(err)
	}

	result.Status = domain.OfferingStatus(status)
	result.Source = domain.OfferingSource(source)

	return result, nil
}

// ListOfferings returns model offerings for a tenant according to the specified filter parameters.
func (repository *Repository) ListOfferings(
	ctx context.Context,
	params ports.ListOfferingsParams,
) ([]domain.ModelOffering, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	query := `
		SELECT ` + offeringColumns + `
		FROM model_access_model_offerings
		WHERE tenant_id = $1::uuid
	`
	args := []any{params.TenantID}
	argIdx := 2

	if params.ConnectionID != "" {
		query += fmt.Sprintf(" AND connection_id = $%d::uuid", argIdx)
		args = append(args, params.ConnectionID)
		argIdx++
	}

	if params.AgentUsableOnly {
		query += " AND agent_usable = true AND status = 'available'"
	}

	if params.Cursor != nil && params.Cursor.OfferingID != "" {
		query += fmt.Sprintf(" AND offering_id > $%d::uuid", argIdx)
		args = append(args, params.Cursor.OfferingID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY offering_id ASC LIMIT $%d", argIdx)
	args = append(args, params.Limit)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list model offerings: %w", mapDatabaseError(err))
	}
	defer rows.Close()

	var results []domain.ModelOffering
	for rows.Next() {
		offering, err := scanOffering(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, offering)
	}

	if err := rows.Err(); err != nil {
		return nil, mapDatabaseError(err)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return results, nil
}

// GetOffering looks up a single offering by tenant and offering ID.
func (repository *Repository) GetOffering(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	offeringID string,
) (domain.ModelOffering, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	if err != nil {
		return domain.ModelOffering{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	row := transaction.QueryRow(
		ctx,
		`
			SELECT `+offeringColumns+`
			FROM model_access_model_offerings
			WHERE tenant_id = $1::uuid
			  AND offering_id = $2::uuid
		`,
		tenantID,
		offeringID,
	)

	offering, err := scanOffering(row)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ModelOffering{}, domain.ErrOfferingNotFound
		}
		return domain.ModelOffering{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.ModelOffering{}, err
	}

	return offering, nil
}

// EnqueueCatalogRefresh records a catalog refresh request and enqueues it for worker processing.
func (repository *Repository) EnqueueCatalogRefresh(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
	idempotencyKey string,
	reason string,
	requestedAt time.Time,
) (domain.CatalogRefresh, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.CatalogRefresh{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	refreshUUID, err := uuid.NewV7()
	if err != nil {
		return domain.CatalogRefresh{}, err
	}

	refreshID := refreshUUID.String()

	tag, err := transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_catalog_refreshes (
				tenant_id,
				refresh_id,
				actor_user_id,
				connection_id,
				idempotency_key,
				reason,
				status,
				catalog_generation,
				requested_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5::uuid,
				$6,
				'pending',
				0,
				$7
			)
			ON CONFLICT (
				tenant_id,
				actor_user_id,
				idempotency_key
			)
			DO NOTHING
		`,
		tenantID,
		refreshID,
		actorUserID,
		connectionID,
		idempotencyKey,
		reason,
		requestedAt,
	)
	if err != nil {
		return domain.CatalogRefresh{}, mapDatabaseError(err)
	}

	if tag.RowsAffected() == 0 {
		// Idempotent conflict: return existing refresh
		var existing domain.CatalogRefresh
		var status string

		err = transaction.QueryRow(
			ctx,
			`
				SELECT
					tenant_id::text,
					refresh_id::text,
					actor_user_id::text,
					connection_id::text,
					idempotency_key::text,
					reason,
					status,
					catalog_generation,
					discovered_count,
					available_count,
					unavailable_count,
					error_code,
					requested_at,
					started_at,
					completed_at
				FROM model_access_catalog_refreshes
				WHERE tenant_id = $1::uuid
				  AND actor_user_id = $2::uuid
				  AND idempotency_key = $3::uuid
			`,
			tenantID,
			actorUserID,
			idempotencyKey,
		).Scan(
			&existing.TenantID,
			&existing.ID,
			&existing.ActorUserID,
			&existing.ConnectionID,
			&existing.IdempotencyKey,
			&existing.Reason,
			&status,
			&existing.Generation,
			&existing.DiscoveredCount,
			&existing.AvailableCount,
			&existing.UnavailableCount,
			&existing.ErrorCode,
			&existing.RequestedAt,
			&existing.StartedAt,
			&existing.CompletedAt,
		)
		if err != nil {
			return domain.CatalogRefresh{}, mapDatabaseError(err)
		}
		existing.Status = domain.CatalogRefreshStatus(status)

		if err := commit(ctx, transaction); err != nil {
			return domain.CatalogRefresh{}, err
		}

		return existing, nil
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_catalog_refresh_queue (
				refresh_id,
				tenant_id,
				actor_user_id,
				connection_id,
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
		refreshID,
		tenantID,
		actorUserID,
		connectionID,
		requestedAt,
	)
	if err != nil {
		return domain.CatalogRefresh{}, mapDatabaseError(err)
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.CatalogRefresh{}, err
	}

	return domain.CatalogRefresh{
		TenantID:       tenantID,
		ID:             refreshID,
		ActorUserID:    actorUserID,
		ConnectionID:   connectionID,
		IdempotencyKey: idempotencyKey,
		Reason:         reason,
		Status:         domain.CatalogRefreshPending,
		Generation:     0,
		RequestedAt:    requestedAt,
	}, nil
}

// GetCatalogRefresh retrieves the catalog refresh state record.
func (repository *Repository) GetCatalogRefresh(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	refreshID string,
) (domain.CatalogRefresh, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	if err != nil {
		return domain.CatalogRefresh{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	var result domain.CatalogRefresh
	var status string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				refresh_id::text,
				actor_user_id::text,
				connection_id::text,
				idempotency_key::text,
				reason,
				status,
				catalog_generation,
				discovered_count,
				available_count,
				unavailable_count,
				error_code,
				requested_at,
				started_at,
				completed_at
			FROM model_access_catalog_refreshes
			WHERE tenant_id = $1::uuid
			  AND refresh_id = $2::uuid
		`,
		tenantID,
		refreshID,
	).Scan(
		&result.TenantID,
		&result.ID,
		&result.ActorUserID,
		&result.ConnectionID,
		&result.IdempotencyKey,
		&result.Reason,
		&status,
		&result.Generation,
		&result.DiscoveredCount,
		&result.AvailableCount,
		&result.UnavailableCount,
		&result.ErrorCode,
		&result.RequestedAt,
		&result.StartedAt,
		&result.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CatalogRefresh{}, domain.ErrCatalogRefreshNotFound
		}
		return domain.CatalogRefresh{}, mapDatabaseError(err)
	}

	result.Status = domain.CatalogRefreshStatus(status)

	if err := commit(ctx, transaction); err != nil {
		return domain.CatalogRefresh{}, err
	}

	return result, nil
}

// ClaimCatalogRefresh locks and claims pending catalog refresh jobs from the queue.
func (repository *Repository) ClaimCatalogRefresh(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]domain.CatalogRefreshJob, error) {
	rows, err := repository.pool.Query(
		ctx,
		`
			WITH candidates AS (
				SELECT refresh_id
				FROM model_access_catalog_refresh_queue
				WHERE available_at <= clock_timestamp()
				  AND (claimed_at IS NULL OR claimed_at < clock_timestamp() - $2::interval)
				ORDER BY available_at ASC, refresh_id ASC
				FOR UPDATE SKIP LOCKED
				LIMIT $1
			)
			UPDATE model_access_catalog_refresh_queue AS q
			SET
				claimed_at = clock_timestamp(),
				attempts = q.attempts + 1
			FROM candidates
			WHERE q.refresh_id = candidates.refresh_id
			RETURNING
				q.refresh_id::text,
				q.tenant_id::text,
				q.actor_user_id::text,
				q.connection_id::text,
				q.attempts
		`,
		limit,
		lease.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("claim catalog refresh queue: %w", err)
	}
	defer rows.Close()

	var jobs []domain.CatalogRefreshJob
	for rows.Next() {
		var job domain.CatalogRefreshJob
		if err := rows.Scan(
			&job.RefreshID,
			&job.TenantID,
			&job.ActorUserID,
			&job.ConnectionID,
			&job.Attempts,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// ApplyCatalogRefresh transactionally updates model offerings and catalog refresh state.
func (repository *Repository) ApplyCatalogRefresh(
	ctx context.Context,
	params ports.ApplyCatalogRefreshParams,
) (domain.CatalogRefreshResult, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.CatalogRefreshResult{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	// Lock catalog state for connection
	var currentGeneration int64
	err = transaction.QueryRow(
		ctx,
		`
			SELECT generation
			FROM model_access_catalog_states
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
			FOR UPDATE
		`,
		params.TenantID,
		params.ConnectionID,
	).Scan(&currentGeneration)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		currentGeneration = 1
		_, err = transaction.Exec(
			ctx,
			`
				INSERT INTO model_access_catalog_states (
					tenant_id,
					connection_id,
					generation,
					last_success_at,
					available_count,
					unavailable_count,
					updated_at
				)
				VALUES (
					$1::uuid,
					$2::uuid,
					1,
					$3,
					0,
					0,
					$3
				)
			`,
			params.TenantID,
			params.ConnectionID,
			params.RefreshedAt,
		)
		if err != nil {
			return domain.CatalogRefreshResult{}, mapDatabaseError(err)
		}
	case err != nil:
		return domain.CatalogRefreshResult{}, mapDatabaseError(err)
	default:
		currentGeneration++
		_, err = transaction.Exec(
			ctx,
			`
				UPDATE model_access_catalog_states
				SET
					generation = $3,
					last_success_at = $4,
					updated_at = $4
				WHERE tenant_id = $1::uuid
				  AND connection_id = $2::uuid
			`,
			params.TenantID,
			params.ConnectionID,
			currentGeneration,
			params.RefreshedAt,
		)
		if err != nil {
			return domain.CatalogRefreshResult{}, mapDatabaseError(err)
		}
	}

	discoveredModelIDs := make([]string, 0, len(params.Discovered))
	for _, m := range params.Discovered {
		discoveredModelIDs = append(discoveredModelIDs, m.ProviderModelID)

		offeringUUID, _ := uuid.NewV7()

		_, err = transaction.Exec(
			ctx,
			`
				INSERT INTO model_access_model_offerings (
					tenant_id,
					offering_id,
					connection_id,
					provider_key,
					provider_model_id,
					display_name,
					description,
					status,
					source,
					agent_usable,
					capabilities,
					input_modalities,
					output_modalities,
					context_window_tokens,
					max_output_tokens,
					version,
					first_seen_at,
					last_seen_at,
					refreshed_at
				)
				VALUES (
					$1::uuid,
					$2::uuid,
					$3::uuid,
					$4,
					$5,
					$6,
					$7,
					'available',
					$8,
					$9,
					$10,
					$11,
					$12,
					$13,
					$14,
					1,
					$15,
					$15,
					$15
				)
				ON CONFLICT (tenant_id, connection_id, provider_model_id)
				DO UPDATE SET
					display_name = EXCLUDED.display_name,
					description = EXCLUDED.description,
					status = 'available',
					source = EXCLUDED.source,
					agent_usable = EXCLUDED.agent_usable,
					capabilities = EXCLUDED.capabilities,
					input_modalities = EXCLUDED.input_modalities,
					output_modalities = EXCLUDED.output_modalities,
					context_window_tokens = EXCLUDED.context_window_tokens,
					max_output_tokens = EXCLUDED.max_output_tokens,
					version = CASE
						WHEN (
							model_access_model_offerings.display_name,
							model_access_model_offerings.description,
							model_access_model_offerings.status,
							model_access_model_offerings.source,
							model_access_model_offerings.agent_usable,
							model_access_model_offerings.capabilities,
							model_access_model_offerings.input_modalities,
							model_access_model_offerings.output_modalities,
							model_access_model_offerings.context_window_tokens,
							model_access_model_offerings.max_output_tokens
						) IS DISTINCT FROM (
							EXCLUDED.display_name,
							EXCLUDED.description,
							'available',
							EXCLUDED.source,
							EXCLUDED.agent_usable,
							EXCLUDED.capabilities,
							EXCLUDED.input_modalities,
							EXCLUDED.output_modalities,
							EXCLUDED.context_window_tokens,
							EXCLUDED.max_output_tokens
						) THEN model_access_model_offerings.version + 1
						ELSE model_access_model_offerings.version
					END,
					last_seen_at = EXCLUDED.last_seen_at,
					refreshed_at = EXCLUDED.refreshed_at,
					unavailable_at = NULL
			`,
			params.TenantID,
			offeringUUID.String(),
			params.ConnectionID,
			m.ProviderKey,
			m.ProviderModelID,
			m.DisplayName,
			m.Description,
			string(params.Source),
			m.AgentUsable,
			m.Capabilities,
			m.InputModalities,
			m.OutputModalities,
			m.ContextWindowTokens,
			m.MaxOutputTokens,
			params.RefreshedAt,
		)
		if err != nil {
			return domain.CatalogRefreshResult{}, mapDatabaseError(err)
		}
	}

	// Mark disappeared models unavailable
	tag, err := transaction.Exec(
		ctx,
		`
			UPDATE model_access_model_offerings
			SET
				status = 'unavailable',
				version = version + 1,
				unavailable_at = $4,
				refreshed_at = $4
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
			  AND status = 'available'
			  AND NOT (provider_model_id = ANY($3))
		`,
		params.TenantID,
		params.ConnectionID,
		discoveredModelIDs,
		params.RefreshedAt,
	)
	if err != nil {
		return domain.CatalogRefreshResult{}, mapDatabaseError(err)
	}

	unavailableCount := int(tag.RowsAffected())
	availableCount := len(params.Discovered)

	// Update catalog state counts
	_, err = transaction.Exec(
		ctx,
		`
			UPDATE model_access_catalog_states
			SET
				available_count = (
					SELECT count(*)
					FROM model_access_model_offerings
					WHERE tenant_id = $1::uuid
					  AND connection_id = $2::uuid
					  AND status = 'available'
				),
				unavailable_count = (
					SELECT count(*)
					FROM model_access_model_offerings
					WHERE tenant_id = $1::uuid
					  AND connection_id = $2::uuid
					  AND status = 'unavailable'
				),
				updated_at = $3
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
		`,
		params.TenantID,
		params.ConnectionID,
		params.RefreshedAt,
	)
	if err != nil {
		return domain.CatalogRefreshResult{}, mapDatabaseError(err)
	}

	// Update refresh record if refreshID was provided
	if params.RefreshID != "" {
		_, err = transaction.Exec(
			ctx,
			`
				UPDATE model_access_catalog_refreshes
				SET
					status = 'succeeded',
					catalog_generation = $3,
					discovered_count = $4,
					available_count = $5,
					unavailable_count = $6,
					completed_at = $7
				WHERE tenant_id = $1::uuid
				  AND refresh_id = $2::uuid
			`,
			params.TenantID,
			params.RefreshID,
			currentGeneration,
			len(params.Discovered),
			availableCount,
			unavailableCount,
			params.RefreshedAt,
		)
		if err != nil {
			return domain.CatalogRefreshResult{}, mapDatabaseError(err)
		}

		// Delete from queue
		_, _ = transaction.Exec(
			ctx,
			`
				DELETE FROM model_access_catalog_refresh_queue
				WHERE tenant_id = $1::uuid
				  AND refresh_id = $2::uuid
			`,
			params.TenantID,
			params.RefreshID,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.CatalogRefreshResult{}, err
	}

	return domain.CatalogRefreshResult{
		Generation:       currentGeneration,
		DiscoveredCount:  len(params.Discovered),
		AvailableCount:   availableCount,
		UnavailableCount: unavailableCount,
		RefreshedAt:      params.RefreshedAt,
	}, nil
}

// FailCatalogRefresh marks a catalog refresh execution as failed in the database.
func (repository *Repository) FailCatalogRefresh(
	ctx context.Context,
	refreshID string,
	tenantID string,
	actorUserID string,
	_ string,
	errorCode string,
	failedAt time.Time,
) error {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE model_access_catalog_refreshes
			SET
				status = 'failed',
				error_code = $3,
				completed_at = $4
			WHERE tenant_id = $1::uuid
			  AND refresh_id = $2::uuid
		`,
		tenantID,
		refreshID,
		errorCode,
		failedAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	_, _ = transaction.Exec(
		ctx,
		`
			DELETE FROM model_access_catalog_refresh_queue
			WHERE tenant_id = $1::uuid
			  AND refresh_id = $2::uuid
		`,
		tenantID,
		refreshID,
	)

	return commit(ctx, transaction)
}

// ReleaseCatalogRefresh unlocks a claimed queue job and schedules its retry.
func (repository *Repository) ReleaseCatalogRefresh(
	ctx context.Context,
	refreshID string,
	retryAt time.Time,
	message string,
) error {
	_, err := repository.pool.Exec(
		ctx,
		`
			UPDATE model_access_catalog_refresh_queue
			SET
				claimed_at = NULL,
				available_at = $2,
				last_error = left($3, 2000)
			WHERE refresh_id = $1::uuid
		`,
		refreshID,
		retryAt,
		message,
	)
	if err != nil {
		return fmt.Errorf("release catalog refresh queue: %w", err)
	}

	return nil
}

// MarkConnectionOfferingsUnavailable updates all active offerings under a connection to unavailable.
func (repository *Repository) MarkConnectionOfferingsUnavailable(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	connectionID string,
	unavailableAt time.Time,
) error {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE model_access_model_offerings
			SET
				status = 'unavailable',
				version = version + 1,
				unavailable_at = $3,
				refreshed_at = $3
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
			  AND status = 'available'
		`,
		tenantID,
		connectionID,
		unavailableAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return commit(ctx, transaction)
}
