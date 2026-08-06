package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreateTenant creates a tenant in the provisioning state, its owner
// membership, entitlements, an onboarding operation, and an outbox event
// atomically.
func (repository *Repository) CreateTenant(
	ctx context.Context,
	params ports.CreateTenantParams,
) (domain.CreateTenantResult, error) {
	actorUserID :=
		params.Context.Tenant.CreatedByUserID

	// First use principal scope so a retry can locate the tenant created by
	// this user and creation request before the new tenant ID is known.
	transaction, err := repository.beginPrincipal(
		ctx,
		actorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var existingTenantID string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT tenant_id::text
			FROM tenant_tenants
			WHERE created_by_user_id = $1::uuid
			  AND creation_request_id = $2
		`,
		actorUserID,
		params.RequestID,
	).Scan(&existingTenantID)

	switch {
	case err == nil:
		contextValue, queryErr := queryContext(
			ctx,
			transaction,
			existingTenantID,
			actorUserID,
			false,
		)
		if queryErr != nil {
			return domain.CreateTenantResult{}, queryErr
		}

		operation, queryErr := queryOperationByTenant(
			ctx,
			transaction,
			existingTenantID,
		)
		if queryErr != nil {
			return domain.CreateTenantResult{}, queryErr
		}

		if err := commit(ctx, transaction); err != nil {
			return domain.CreateTenantResult{}, err
		}

		return domain.CreateTenantResult{
			Context:   contextValue,
			Operation: operation,
		}, nil

	case !errors.Is(err, pgx.ErrNoRows):
		return domain.CreateTenantResult{}, fmt.Errorf(
			"check tenant idempotency key: %w",
			err,
		)
	}

	tenant := params.Context.Tenant
	membership := params.Context.Membership
	entitlements := params.Context.Entitlements
	operation := params.Operation

	// From this point onward, all writes are restricted to the newly
	// allocated tenant.
	if err := repository.applyTenantScope(
		ctx,
		transaction,
		tenant.ID,
		actorUserID,
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO tenant_tenants (
				tenant_id,
				slug,
				display_name,
				status,
				region,
				retention_days,
				version,
				created_by_user_id,
				creation_request_id,
				created_at,
				updated_at
			)
			VALUES (
				$1::uuid,
				$2,
				$3,
				'provisioning',
				$4,
				$5,
				$6,
				$7::uuid,
				$8,
				$9,
				$10
			)
		`,
		tenant.ID,
		tenant.Slug,
		tenant.DisplayName,
		tenant.Region,
		tenant.RetentionDays,
		tenant.Version,
		tenant.CreatedByUserID,
		params.RequestID,
		tenant.CreatedAt,
		tenant.UpdatedAt,
	)
	if err != nil {
		return domain.CreateTenantResult{},
			mapDatabaseError(err)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO tenant_memberships (
				tenant_id,
				user_id,
				role,
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
				$5::uuid,
				$6,
				$7
			)
		`,
		membership.TenantID,
		membership.UserID,
		string(membership.Role),
		membership.Version,
		membership.CreatedBy,
		membership.CreatedAt,
		membership.UpdatedAt,
	)
	if err != nil {
		return domain.CreateTenantResult{},
			mapDatabaseError(err)
	}

	features, err := json.Marshal(
		entitlements.Features,
	)
	if err != nil {
		return domain.CreateTenantResult{}, fmt.Errorf(
			"marshal tenant features: %w",
			err,
		)
	}

	limits, err := json.Marshal(
		entitlements.Limits,
	)
	if err != nil {
		return domain.CreateTenantResult{}, fmt.Errorf(
			"marshal tenant limits: %w",
			err,
		)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO tenant_entitlements (
				tenant_id,
				plan_key,
				features,
				limits,
				version,
				updated_at
			)
			VALUES (
				$1::uuid,
				$2,
				$3::jsonb,
				$4::jsonb,
				$5,
				$6
			)
		`,
		entitlements.TenantID,
		entitlements.PlanKey,
		features,
		limits,
		entitlements.Version,
		entitlements.UpdatedAt,
	)
	if err != nil {
		return domain.CreateTenantResult{},
			mapDatabaseError(err)
	}

	if err := insertOperation(
		ctx,
		transaction,
		operation,
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		tenant.ID,
		params.Event,
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	contextValue, err := queryContext(
		ctx,
		transaction,
		tenant.ID,
		membership.UserID,
		false,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	operation, err = queryOperationByTenant(
		ctx,
		transaction,
		tenant.ID,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.CreateTenantResult{}, err
	}

	return domain.CreateTenantResult{
		Context:   contextValue,
		Operation: operation,
	}, nil
}

// GetTenantContext returns trusted context for one active membership.
func (repository *Repository) GetTenantContext(
	ctx context.Context,
	tenantID string,
	userID string,
) (domain.TenantContext, error) {
	transaction, err := repository.beginTenant(
		ctx,
		tenantID,
		userID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := queryContext(
		ctx,
		transaction,
		tenantID,
		userID,
		false,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TenantContext{}, err
	}

	return result, nil
}

// ListTenantContexts lists memberships using UUID keyset pagination.
func (repository *Repository) ListTenantContexts(
	ctx context.Context,
	userID string,
	limit int,
	cursor *ports.TenantCursor,
) ([]domain.TenantContext, error) {
	transaction, err := repository.beginPrincipal(
		ctx,
		userID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

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
		WHERE m.user_id = $1::uuid
	`

	args := []any{userID}

	if cursor != nil {
		query += `
			AND t.tenant_id < $2::uuid
		`

		args = append(args, cursor.TenantID)
	}

	args = append(args, limit)

	query += fmt.Sprintf(
		`
			ORDER BY t.tenant_id DESC
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(
		ctx,
		query,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list tenant contexts: %w",
			err,
		)
	}

	defer rows.Close()

	contexts := make(
		[]domain.TenantContext,
		0,
		limit,
	)

	for rows.Next() {
		contextValue, scanErr :=
			scanTenantContext(rows)

		if scanErr != nil {
			return nil, scanErr
		}

		contexts = append(
			contexts,
			contextValue,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate tenant contexts: %w",
			err,
		)
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return nil, err
	}

	return contexts, nil
}

// UpdateTenant updates mutable tenant settings under optimistic locking.
func (repository *Repository) UpdateTenant(
	ctx context.Context,
	params ports.UpdateTenantParams,
) (domain.TenantContext, error) {
	transaction, err := repository.beginTenant(
		ctx,
		params.Tenant.ID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	current, err := queryContext(
		ctx,
		transaction,
		params.Tenant.ID,
		params.ActorUserID,
		true,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if current.Tenant.Status != domain.StatusActive {
		return domain.TenantContext{}, domain.ErrArchived
	}

	if !domain.CanUpdateTenant(
		current.Membership.Role,
	) {
		return domain.TenantContext{}, domain.ErrForbidden
	}

	if current.Tenant.Version != params.ExpectedVersion {
		return domain.TenantContext{},
			domain.ErrVersionConflict
	}

	commandTag, err := transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET
				display_name = $3,
				region = $4,
				retention_days = $5,
				version = $6,
				updated_at = $7
			WHERE tenant_id = $1::uuid
			  AND version = $2
		`,
		params.Tenant.ID,
		params.ExpectedVersion,
		params.Tenant.DisplayName,
		params.Tenant.Region,
		params.Tenant.RetentionDays,
		params.Tenant.Version,
		params.Tenant.UpdatedAt,
	)
	if err != nil {
		return domain.TenantContext{}, fmt.Errorf(
			"update tenant: %w",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		return domain.TenantContext{},
			domain.ErrVersionConflict
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Tenant.ID,
		params.Event,
	); err != nil {
		return domain.TenantContext{}, err
	}

	result, err := queryContext(
		ctx,
		transaction,
		params.Tenant.ID,
		params.ActorUserID,
		false,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TenantContext{}, err
	}

	return result, nil
}

// ArchiveTenant archives a tenant under owner authorization.
func (repository *Repository) ArchiveTenant(
	ctx context.Context,
	params ports.ArchiveTenantParams,
) (domain.TenantContext, error) {
	transaction, err := repository.beginTenant(
		ctx,
		params.Tenant.ID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	current, err := queryContext(
		ctx,
		transaction,
		params.Tenant.ID,
		params.ActorUserID,
		true,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if !domain.CanArchiveTenant(
		current.Membership.Role,
	) {
		return domain.TenantContext{}, domain.ErrForbidden
	}

	if current.Tenant.Version != params.ExpectedVersion {
		return domain.TenantContext{},
			domain.ErrVersionConflict
	}

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET
				status = 'archived',
				version = $3,
				updated_at = $4,
				archived_at = $5
			WHERE tenant_id = $1::uuid
			  AND version = $2
		`,
		params.Tenant.ID,
		params.ExpectedVersion,
		params.Tenant.Version,
		params.Tenant.UpdatedAt,
		params.Tenant.ArchivedAt,
	)
	if err != nil {
		return domain.TenantContext{}, fmt.Errorf(
			"archive tenant: %w",
			err,
		)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Tenant.ID,
		params.Event,
	); err != nil {
		return domain.TenantContext{}, err
	}

	result, err := queryContext(
		ctx,
		transaction,
		params.Tenant.ID,
		params.ActorUserID,
		false,
	)
	if err != nil {
		return domain.TenantContext{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TenantContext{}, err
	}

	return result, nil
}
