// Package postgres implements the Projection Service repository.
package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/platform/go/grpcx"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/aminio9/gereh/services/projection/internal/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is a pgx-backed projection repository.
type Repository struct {
	database *platformpostgres.Database
	pool     *pgxpool.Pool
}

var _ ports.Repository = (*Repository)(nil)

// New creates a PostgreSQL projection repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		database: platformpostgres.Wrap(pool),
		pool:     pool,
	}
}

func requestIdentifiers(
	ctx context.Context,
) (string, string) {
	metadata, ok := grpcx.RequestMetadataFromContext(ctx)

	if !ok {
		return "", ""
	}

	return metadata.RequestID, metadata.CorrelationID
}

func (repository *Repository) beginUserTenant(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	options pgx.TxOptions,
) (pgx.Tx, error) {
	requestID, correlationID :=
		requestIdentifiers(ctx)

	return repository.database.Begin(
		ctx,
		platformpostgres.TenantScope(
			tenantID,
			actorUserID,
			requestID,
			correlationID,
		),
		options,
	)
}

func (repository *Repository) beginServiceTenant(
	ctx context.Context,
	tenantID string,
	servicePrincipalID string,
	options pgx.TxOptions,
) (pgx.Tx, error) {
	requestID, correlationID :=
		requestIdentifiers(ctx)

	return repository.database.Begin(
		ctx,
		platformpostgres.ServiceTenantScope(
			tenantID,
			servicePrincipalID,
			requestID,
			correlationID,
		),
		options,
	)
}

type projectionTransaction struct {
	tx pgx.Tx
}

// ApplyEvent applies one event inside a single tenant-scoped transaction.
//
// The consumed-event inbox provides idempotency: a redelivered event that was
// already committed is detected without mutation and safely checkpointed.
// Returns true when the event applied for the first time.
func (repository *Repository) ApplyEvent(
	ctx context.Context,
	servicePrincipalID string,
	event domain.EventMeta,
	apply ports.ApplyFunc,
) (bool, error) {
	transaction, err :=
		repository.beginServiceTenant(
			ctx,
			event.TenantID,
			servicePrincipalID,
			pgx.TxOptions{},
		)
	if err != nil {
		return false, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var insertedID string

	err = transaction.QueryRow(
		ctx,
		`
			INSERT INTO projection_consumed_events (
				event_id,
				tenant_id,
				topic,
				partition,
				offset_value,
				event_type,
				event_version,
				aggregate_type,
				aggregate_id,
				aggregate_version,
				event_hash,
				occurred_at,
				consumed_at
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
				$9,
				$10,
				$11,
				$12,
				$13
			)
			ON CONFLICT (event_id)
			DO NOTHING
			RETURNING event_id::text
		`,
		event.EventID,
		event.TenantID,
		event.Topic,
		event.Partition,
		event.Offset,
		event.EventType,
		event.EventVersion,
		event.AggregateType,
		event.AggregateID,
		event.AggregateVersion,
		event.EventHash,
		event.OccurredAt,
		event.ProcessedAt,
	).Scan(&insertedID)

	switch {
	case err == nil:
		// New event. Continue applying.

	case errors.Is(err, pgx.ErrNoRows):
		var existingHash []byte

		queryErr := transaction.QueryRow(
			ctx,
			`
				SELECT event_hash
				FROM projection_consumed_events
				WHERE event_id = $1::uuid
			`,
			event.EventID,
		).Scan(&existingHash)
		if queryErr != nil {
			return false, queryErr
		}

		if !bytes.Equal(
			existingHash,
			event.EventHash,
		) {
			return false,
				domain.ErrEventIdentityConflict
		}

		if err := updateCheckpoint(
			ctx,
			transaction,
			event,
		); err != nil {
			return false, err
		}

		if err := transaction.Commit(ctx); err != nil {
			return false, fmt.Errorf(
				"commit duplicate projection event: %w",
				err,
			)
		}

		return false, nil

	default:
		return false, fmt.Errorf(
			"insert projection inbox event: %w",
			err,
		)
	}

	if apply != nil {
		if err := apply(
			ctx,
			&projectionTransaction{
				tx: transaction,
			},
		); err != nil {
			return false, err
		}
	}

	if err := updateWatermark(
		ctx,
		transaction,
		event,
	); err != nil {
		return false, err
	}

	if err := updateCheckpoint(
		ctx,
		transaction,
		event,
	); err != nil {
		return false, err
	}

	if err := transaction.Commit(ctx); err != nil {
		return false, fmt.Errorf(
			"commit projection event: %w",
			err,
		)
	}

	return true, nil
}

func updateWatermark(
	ctx context.Context,
	tx pgx.Tx,
	event domain.EventMeta,
) error {
	_, err := tx.Exec(
		ctx,
		`
			INSERT INTO projection_tenant_watermarks (
				tenant_id,
				projected_through_event_time,
				last_processed_at
			)
			VALUES (
				$1::uuid,
				$2,
				$3
			)
			ON CONFLICT (tenant_id)
			DO UPDATE SET
				projected_through_event_time =
					GREATEST(
						projection_tenant_watermarks.
							projected_through_event_time,
						EXCLUDED.projected_through_event_time
					),
				last_processed_at =
					GREATEST(
						projection_tenant_watermarks.
							last_processed_at,
						EXCLUDED.last_processed_at
					)
		`,
		event.TenantID,
		event.OccurredAt,
		event.ProcessedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"update tenant projection watermark: %w",
			err,
		)
	}

	return nil
}

func updateCheckpoint(
	ctx context.Context,
	tx pgx.Tx,
	event domain.EventMeta,
) error {
	_, err := tx.Exec(
		ctx,
		`
			INSERT INTO projection_partition_checkpoints (
				topic,
				partition,
				last_offset,
				last_event_id,
				last_event_at,
				processed_at
			)
			VALUES (
				$1,
				$2,
				$3,
				$4::uuid,
				$5,
				$6
			)
			ON CONFLICT (
				topic,
				partition
			)
			DO UPDATE SET
				last_offset =
					EXCLUDED.last_offset,
				last_event_id =
					EXCLUDED.last_event_id,
				last_event_at =
					EXCLUDED.last_event_at,
				processed_at =
					EXCLUDED.processed_at
			WHERE
				projection_partition_checkpoints.
					last_offset
				<
				EXCLUDED.last_offset
		`,
		event.Topic,
		event.Partition,
		event.Offset,
		event.EventID,
		event.OccurredAt,
		event.ProcessedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"update projection partition checkpoint: %w",
			err,
		)
	}

	return nil
}
