package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

// ClaimOutbox claims up to limit due events for the relay lease window.
func (repository *Repository) ClaimOutbox(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]domain.OutboxRecord, error) {
	rows, err := repository.pool.Query(
		ctx,
		`
			WITH candidates AS (
				SELECT outbox_id
				FROM model_access_outbox
				WHERE published_at IS NULL
				  AND available_at <= clock_timestamp()
				  AND (
					claimed_at IS NULL
					OR claimed_at < clock_timestamp() - $2::interval
				  )
				ORDER BY outbox_id
				FOR UPDATE SKIP LOCKED
				LIMIT $1
			)
			UPDATE model_access_outbox AS event
			SET
				claimed_at = clock_timestamp(),
				attempts = event.attempts + 1
			FROM candidates
			WHERE event.outbox_id = candidates.outbox_id
			RETURNING
				event.outbox_id,
				event.tenant_id::text,
				event.event_id::text,
				event.topic,
				event.partition_key,
				event.envelope,
				event.occurred_at,
				event.attempts
		`,
		limit,
		lease.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("claim Model Access outbox: %w", err)
	}
	defer rows.Close()

	result := make([]domain.OutboxRecord, 0, limit)

	for rows.Next() {
		var item domain.OutboxRecord

		if err := rows.Scan(
			&item.OutboxID,
			&item.TenantID,
			&item.EventID,
			&item.Topic,
			&item.Key,
			&item.Envelope,
			&item.OccurredAt,
			&item.Attempts,
		); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

// MarkOutboxPublished records a successful Kafka publish.
func (repository *Repository) MarkOutboxPublished(
	ctx context.Context,
	outboxID int64,
) error {
	_, err := repository.pool.Exec(
		ctx,
		`
			UPDATE model_access_outbox
			SET
				published_at = clock_timestamp(),
				claimed_at = NULL,
				last_error = NULL
			WHERE outbox_id = $1
			  AND published_at IS NULL
		`,
		outboxID,
	)
	if err != nil {
		return fmt.Errorf(
			"mark Model Access outbox published: %w",
			err,
		)
	}

	return nil
}

// ReleaseOutbox schedules a failed event for retry.
func (repository *Repository) ReleaseOutbox(
	ctx context.Context,
	outboxID int64,
	retryAt time.Time,
	publishError string,
) error {
	_, err := repository.pool.Exec(
		ctx,
		`
			UPDATE model_access_outbox
			SET
				claimed_at = NULL,
				available_at = $2,
				last_error = left($3, 2000)
			WHERE outbox_id = $1
			  AND published_at IS NULL
		`,
		outboxID,
		retryAt,
		publishError,
	)
	if err != nil {
		return fmt.Errorf(
			"release Model Access outbox: %w",
			err,
		)
	}

	return nil
}
