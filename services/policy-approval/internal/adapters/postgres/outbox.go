package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
)

// ClaimOutbox leases pending outbox rows using SKIP LOCKED.
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
				FROM policy_outbox
				WHERE published_at IS NULL
				  AND available_at <= clock_timestamp()
				  AND (
					claimed_at IS NULL
					OR claimed_at <
						clock_timestamp()
						- ($2 * interval '1 millisecond')
				  )
				ORDER BY outbox_id
				FOR UPDATE SKIP LOCKED
				LIMIT $1
			)
			UPDATE policy_outbox AS outbox
			SET
				claimed_at = clock_timestamp(),
				attempts = outbox.attempts + 1
			FROM candidates
			WHERE outbox.outbox_id =
				candidates.outbox_id
			RETURNING
				outbox.outbox_id,
				outbox.event_id::text,
				outbox.topic,
				outbox.partition_key,
				outbox.envelope,
				outbox.occurred_at,
				outbox.attempts
		`,
		limit,
		lease.Milliseconds(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"claim policy outbox: %w",
			err,
		)
	}
	defer rows.Close()

	var records []domain.OutboxRecord

	for rows.Next() {
		var record domain.OutboxRecord

		if err := rows.Scan(
			&record.OutboxID,
			&record.Event.ID,
			&record.Event.Topic,
			&record.Event.Key,
			&record.Event.Envelope,
			&record.Event.OccurredAt,
			&record.Attempts,
		); err != nil {
			return nil, fmt.Errorf(
				"scan policy outbox row: %w",
				err,
			)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate policy outbox rows: %w",
			err,
		)
	}

	return records, nil
}

// MarkOutboxPublished marks an event as published.
func (repository *Repository) MarkOutboxPublished(
	ctx context.Context,
	outboxID int64,
) error {
	_, err := repository.pool.Exec(
		ctx,
		`
			UPDATE policy_outbox
			SET
				published_at = clock_timestamp(),
				claimed_at = NULL,
				last_error = NULL
			WHERE outbox_id = $1
		`,
		outboxID,
	)
	if err != nil {
		return fmt.Errorf(
			"mark policy outbox published: %w",
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
	if len(publishError) > 2000 {
		publishError = publishError[:2000]
	}

	_, err := repository.pool.Exec(
		ctx,
		`
			UPDATE policy_outbox
			SET
				claimed_at = NULL,
				available_at = $2,
				last_error = $3
			WHERE outbox_id = $1
			  AND published_at IS NULL
		`,
		outboxID,
		retryAt,
		publishError,
	)
	if err != nil {
		return fmt.Errorf(
			"release policy outbox row: %w",
			err,
		)
	}

	return nil
}
