package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolationCode = "23505"

type rowQuerier interface {
	QueryRow(
		ctx context.Context,
		sql string,
		args ...any,
	) pgx.Row
}

// Repository is a pgx-backed tenant repository.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates a PostgreSQL tenant repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func begin(
	ctx context.Context,
	pool *pgxpool.Pool,
) (pgx.Tx, error) {
	transaction, err := pool.BeginTx(
		ctx,
		pgx.TxOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"begin tenant transaction: %w",
			err,
		)
	}

	return transaction, nil
}

func commit(
	ctx context.Context,
	transaction pgx.Tx,
) error {
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf(
			"commit tenant transaction: %w",
			err,
		)
	}

	return nil
}

func mapDatabaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	var postgresError *pgconn.PgError

	if errors.As(err, &postgresError) &&
		postgresError.Code == uniqueViolationCode {
		return fmt.Errorf(
			"%w: %s",
			domain.ErrConflict,
			postgresError.ConstraintName,
		)
	}

	return err
}

func queryContext(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	userID string,
	lock bool,
) (domain.TenantContext, error) {
	query := `
		SELECT
			t.tenant_id::text,
			t.slug,
			t.display_name,
			t.status,
			t.region,
			t.retention_days,
			t.version,
			t.created_by_user_id::text,
			t.created_at,
			t.updated_at,
			t.archived_at,

			m.user_id::text,
			m.role,
			m.version,
			m.created_by_user_id::text,
			m.created_at,
			m.updated_at,

			e.plan_key,
			e.features,
			e.limits,
			e.version,
			e.updated_at
		FROM tenant_tenants AS t
		JOIN tenant_memberships AS m
			ON m.tenant_id = t.tenant_id
		JOIN tenant_entitlements AS e
			ON e.tenant_id = t.tenant_id
		WHERE t.tenant_id = $1::uuid
		  AND m.user_id = $2::uuid
	`

	if lock {
		query += `
			FOR UPDATE OF t, m, e
		`
	}

	var result domain.TenantContext
	var status string
	var role string
	var featuresJSON []byte
	var limitsJSON []byte

	err := querier.QueryRow(
		ctx,
		query,
		tenantID,
		userID,
	).Scan(
		&result.Tenant.ID,
		&result.Tenant.Slug,
		&result.Tenant.DisplayName,
		&status,
		&result.Tenant.Region,
		&result.Tenant.RetentionDays,
		&result.Tenant.Version,
		&result.Tenant.CreatedByUserID,
		&result.Tenant.CreatedAt,
		&result.Tenant.UpdatedAt,
		&result.Tenant.ArchivedAt,

		&result.Membership.UserID,
		&role,
		&result.Membership.Version,
		&result.Membership.CreatedBy,
		&result.Membership.CreatedAt,
		&result.Membership.UpdatedAt,

		&result.Entitlements.PlanKey,
		&featuresJSON,
		&limitsJSON,
		&result.Entitlements.Version,
		&result.Entitlements.UpdatedAt,
	)
	if err != nil {
		return domain.TenantContext{},
			mapDatabaseError(err)
	}

	result.Tenant.Status = domain.Status(status)

	result.Membership.TenantID = result.Tenant.ID
	result.Membership.Role = domain.Role(role)

	result.Entitlements.TenantID = result.Tenant.ID

	if err := json.Unmarshal(
		featuresJSON,
		&result.Entitlements.Features,
	); err != nil {
		return domain.TenantContext{}, fmt.Errorf(
			"decode tenant feature entitlements: %w",
			err,
		)
	}

	if err := json.Unmarshal(
		limitsJSON,
		&result.Entitlements.Limits,
	); err != nil {
		return domain.TenantContext{}, fmt.Errorf(
			"decode tenant limit entitlements: %w",
			err,
		)
	}

	return result, nil
}

func insertOutbox(
	ctx context.Context,
	transaction pgx.Tx,
	event domain.OutboxEvent,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO tenant_outbox (
				event_id,
				topic,
				partition_key,
				envelope,
				occurred_at
			)
			VALUES (
				$1::uuid,
				$2,
				$3,
				$4,
				$5
			)
		`,
		event.ID,
		event.Topic,
		event.Key,
		event.Envelope,
		event.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf(
			"insert tenant outbox event: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}
