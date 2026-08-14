// Package postgres implements PostgreSQL request journaling and outbox management.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository manages PostgreSQL request journaling and the transactional outbox with RLS.
type Repository struct {
	database *platformpostgres.Database
	pool     *pgxpool.Pool
}

// New creates a new Model Gateway repository.
func New(database *platformpostgres.Database) *Repository {
	return &Repository{
		database: database,
		pool:     database.Pool(),
	}
}

// AdmitRequest records the incoming request into the journal under tenant RLS.
// Fails if the request ID has already been admitted or completed for this tenant.
func (r *Repository) AdmitRequest(
	ctx context.Context,
	record domain.JournalRecord,
) error {
	scope := platformpostgres.TenantScope(
		record.TenantID,
		uuid.Nil.String(),
		record.RequestID,
		record.RequestID,
	)

	tx, err := r.database.Begin(ctx, scope, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin admit request tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(
		ctx,
		`
			INSERT INTO model_gateway_request_journal (
				tenant_id,
				request_id,
				agent_id,
				execution_id,
				workflow_id,
				run_id,
				step_id,
				connection_id,
				offering_id,
				provider_key,
				provider_model_id,
				status,
				admitted_at
			)
			VALUES (
				$1::uuid,
				$2,
				$3::uuid,
				$4::uuid,
				$5,
				$6,
				$7,
				$8::uuid,
				$9::uuid,
				$10,
				$11,
				'admitted',
				$12
			)
		`,
		record.TenantID,
		record.RequestID,
		record.AgentID,
		record.ExecutionID,
		record.WorkflowID,
		record.RunID,
		record.StepID,
		record.ConnectionID,
		record.OfferingID,
		record.ProviderKey,
		record.ProviderModelID,
		record.AdmittedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // Unique violation
			return domain.ErrDuplicateRequestID
		}
		return fmt.Errorf("insert journal record: %w", err)
	}

	return tx.Commit(ctx)
}

// CompleteRequest updates the journal with final metrics and transactionally enqueues the outbox event.
func (r *Repository) CompleteRequest(
	ctx context.Context,
	record domain.JournalRecord,
	outboxEvent *domain.OutboxEvent,
) error {
	scope := platformpostgres.TenantScope(
		record.TenantID,
		uuid.Nil.String(),
		record.RequestID,
		record.RequestID,
	)

	tx, err := r.database.Begin(ctx, scope, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete request tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	completedAt := record.CompletedAt
	if completedAt == nil {
		now := time.Now().UTC()
		completedAt = &now
	}

	_, err = tx.Exec(
		ctx,
		`
			UPDATE model_gateway_request_journal
			SET
				connection_id = $3::uuid,
				offering_id = $4::uuid,
				provider_key = $5,
				provider_model_id = $6,
				status = $7,
				prompt_tokens = $8,
				completion_tokens = $9,
				total_tokens = $10,
				cached_prompt_tokens = $11,
				reasoning_tokens = $12,
				estimated_cost_microusd = $13,
				error_code = $14,
				streamed = $15,
				retry_count = $16,
				fallback_from_offering_id = $17::uuid,
				duration_ms = $18,
				time_to_first_token_ms = $19,
				completed_at = $20
			WHERE tenant_id = $1::uuid
			  AND request_id = $2
		`,
		record.TenantID,
		record.RequestID,
		record.ConnectionID,
		record.OfferingID,
		record.ProviderKey,
		record.ProviderModelID,
		string(record.Status),
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
		record.CachedPromptTokens,
		record.ReasoningTokens,
		record.EstimatedCostMicroUSD,
		record.ErrorCode,
		record.Streamed,
		record.RetryCount,
		record.FallbackFromOfferingID,
		record.DurationMS,
		record.TimeToFirstTokenMS,
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("update journal record: %w", err)
	}

	if outboxEvent != nil {
		_, err = tx.Exec(
			ctx,
			`
				INSERT INTO model_gateway_outbox (
					tenant_id,
					event_id,
					topic,
					partition_key,
					envelope,
					occurred_at
				)
				VALUES (
					$1::uuid,
					$2::uuid,
					$3,
					$4,
					$5,
					$6
				)
			`,
			record.TenantID,
			outboxEvent.EventID,
			outboxEvent.Topic,
			outboxEvent.PartitionKey,
			outboxEvent.Payload,
			outboxEvent.OccurredAt,
		)
		if err != nil {
			return fmt.Errorf("insert outbox record: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// OutboxRecord represents an item claimed from the transactional outbox table.
type OutboxRecord struct {
	OutboxID     int64
	TenantID     string
	EventID      string
	Topic        string
	PartitionKey string
	Envelope     []byte
	Attempts     int
}

// ClaimOutbox claims pending outbox messages for publication.
func (r *Repository) ClaimOutbox(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]OutboxRecord, error) {
	rows, err := r.pool.Query(
		ctx,
		`
			WITH candidates AS (
				SELECT outbox_id
				FROM model_gateway_outbox
				WHERE available_at <= clock_timestamp()
				  AND published_at IS NULL
				  AND (claimed_at IS NULL OR claimed_at < clock_timestamp() - $2::interval)
				ORDER BY available_at ASC, outbox_id ASC
				FOR UPDATE SKIP LOCKED
				LIMIT $1
			)
			UPDATE model_gateway_outbox AS o
			SET
				claimed_at = clock_timestamp(),
				attempts = o.attempts + 1
			FROM candidates
			WHERE o.outbox_id = candidates.outbox_id
			RETURNING
				o.outbox_id,
				o.tenant_id::text,
				o.event_id::text,
				o.topic,
				o.partition_key,
				o.envelope,
				o.attempts
		`,
		limit,
		lease.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("claim model gateway outbox: %w", err)
	}
	defer rows.Close()

	var records []OutboxRecord
	for rows.Next() {
		var rec OutboxRecord
		if err := rows.Scan(
			&rec.OutboxID,
			&rec.TenantID,
			&rec.EventID,
			&rec.Topic,
			&rec.PartitionKey,
			&rec.Envelope,
			&rec.Attempts,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// MarkOutboxPublished flags an outbox message as published.
func (r *Repository) MarkOutboxPublished(
	ctx context.Context,
	outboxID int64,
) error {
	_, err := r.pool.Exec(
		ctx,
		`
			UPDATE model_gateway_outbox
			SET published_at = clock_timestamp()
			WHERE outbox_id = $1
		`,
		outboxID,
	)
	return err
}

// ReleaseOutbox releases a failed outbox message with backoff.
func (r *Repository) ReleaseOutbox(
	ctx context.Context,
	outboxID int64,
	retryAt time.Time,
	publishError string,
) error {
	_, err := r.pool.Exec(
		ctx,
		`
			UPDATE model_gateway_outbox
			SET
				claimed_at = NULL,
				available_at = $2,
				last_error = left($3, 2000)
			WHERE outbox_id = $1
		`,
		outboxID,
		retryAt,
		publishError,
	)
	return err
}
