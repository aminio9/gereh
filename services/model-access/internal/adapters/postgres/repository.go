// Package postgres implements Model Access persistence.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/platform/go/grpcx"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	uniqueViolationCode      = "23505"
	foreignKeyViolationCode  = "23503"
	checkViolationCode       = "23514"
	serializationFailureCode = "40001"
)

type rowScanner interface {
	Scan(dest ...any) error
}

// Repository implements ports.Repository over PostgreSQL.
type Repository struct {
	database *platformpostgres.Database

	// Direct pool access is intentionally restricted to
	// the cross-tenant operational outbox.
	pool *pgxpool.Pool
}

// New constructs the repository.
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
	requestID, correlationID := requestIdentifiers(ctx)

	transaction, err := repository.database.Begin(
		ctx,
		platformpostgres.TenantScope(
			tenantID,
			actorUserID,
			requestID,
			correlationID,
		),
		options,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"begin Model Access transaction: %w",
			err,
		)
	}

	return transaction, nil
}

func commit(ctx context.Context, transaction pgx.Tx) error {
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf(
			"commit Model Access transaction: %w",
			err,
		)
	}

	return nil
}

// mapDatabaseError converts PostgreSQL errors into domain errors without
// leaking constraint names to gRPC/HTTP clients.
func mapDatabaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return err
	}

	switch postgresError.Code {
	case uniqueViolationCode:
		return domain.ErrConflict

	case foreignKeyViolationCode,
		checkViolationCode:
		return domain.ErrInvalidArgument

	case serializationFailureCode:
		return domain.ErrVersionConflict

	default:
		return err
	}
}

func insertOutbox(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	event domain.OutboxEvent,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_outbox (
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
		tenantID,
		event.ID,
		event.Topic,
		event.Key,
		event.Envelope,
		event.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf(
			"insert Model Access outbox event: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}
