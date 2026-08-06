package postgres

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/ports"
	"github.com/jackc/pgx/v5"
)

func queryMembership(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	userID string,
	lock bool,
) (domain.Membership, error) {
	query := `
		SELECT
			tenant_id::text,
			user_id::text,
			role,
			version,
			created_by_user_id::text,
			created_at,
			updated_at
		FROM tenant_memberships
		WHERE tenant_id = $1::uuid
		  AND user_id = $2::uuid
	`

	if lock {
		query += ` FOR UPDATE`
	}

	var membership domain.Membership
	var role string

	err := querier.QueryRow(
		ctx,
		query,
		tenantID,
		userID,
	).Scan(
		&membership.TenantID,
		&membership.UserID,
		&role,
		&membership.Version,
		&membership.CreatedBy,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	)
	if err != nil {
		return domain.Membership{},
			mapDatabaseError(err)
	}

	membership.Role = domain.Role(role)

	return membership, nil
}

// GetMembership returns a tenant membership.
func (repository *Repository) GetMembership(
	ctx context.Context,
	tenantID string,
	userID string,
) (domain.Membership, error) {
	transaction, err := repository.beginTenant(
		ctx,
		tenantID,
		userID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Membership{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := queryMembership(
		ctx,
		transaction,
		tenantID,
		userID,
		false,
	)
	if err != nil {
		return domain.Membership{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Membership{}, err
	}

	return result, nil
}

// GetMembershipAsActor returns one membership using actorUserID as the
// security principal and targetUserID only as the row predicate.
func (repository *Repository) GetMembershipAsActor(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	targetUserID string,
) (domain.Membership, error) {
	transaction, err := repository.beginTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Membership{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := queryMembership(
		ctx,
		transaction,
		tenantID,
		targetUserID,
		false,
	)
	if err != nil {
		return domain.Membership{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Membership{}, err
	}

	return result, nil
}

// ListMembers lists memberships after validating actor membership.
func (repository *Repository) ListMembers(
	ctx context.Context,
	tenantID string,
	actorUserID string,
	limit int,
	cursor *ports.MemberCursor,
) ([]domain.Membership, error) {
	transaction, err := repository.beginTenant(
		ctx,
		tenantID,
		actorUserID,
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

	if _, err := queryContext(
		ctx,
		transaction,
		tenantID,
		actorUserID,
		false,
	); err != nil {
		return nil, err
	}

	query := `
		SELECT
			tenant_id::text,
			user_id::text,
			role,
			version,
			created_by_user_id::text,
			created_at,
			updated_at
		FROM tenant_memberships
		WHERE tenant_id = $1::uuid
	`

	args := []any{tenantID}

	if cursor != nil {
		query += `
			AND user_id > $2::uuid
		`

		args = append(args, cursor.UserID)
	}

	args = append(args, limit)

	query += fmt.Sprintf(
		`
			ORDER BY user_id
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
			"list tenant memberships: %w",
			err,
		)
	}

	defer rows.Close()

	memberships := make(
		[]domain.Membership,
		0,
		limit,
	)

	for rows.Next() {
		var membership domain.Membership
		var role string

		if err := rows.Scan(
			&membership.TenantID,
			&membership.UserID,
			&role,
			&membership.Version,
			&membership.CreatedBy,
			&membership.CreatedAt,
			&membership.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf(
				"scan tenant membership: %w",
				err,
			)
		}

		membership.Role = domain.Role(role)

		memberships = append(
			memberships,
			membership,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate tenant memberships: %w",
			err,
		)
	}

	if err := commit(
		ctx,
		transaction,
	); err != nil {
		return nil, err
	}

	return memberships, nil
}

// AddMember creates a membership and increments the tenant version.
func (repository *Repository) AddMember(
	ctx context.Context,
	params ports.AddMemberParams,
) (domain.Membership, error) {
	transaction, err := repository.beginTenant(
		ctx,
		params.Membership.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Membership{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	actorContext, err := queryContext(
		ctx,
		transaction,
		params.Membership.TenantID,
		params.ActorUserID,
		true,
	)
	if err != nil {
		return domain.Membership{}, err
	}

	if actorContext.Tenant.Status != domain.StatusActive {
		return domain.Membership{}, domain.ErrArchived
	}

	if actorContext.Tenant.Version !=
		params.ExpectedTenantVersion {
		return domain.Membership{},
			domain.ErrVersionConflict
	}

	if !domain.CanManageMember(
		actorContext.Membership.Role,
		"",
		params.Membership.Role,
	) {
		return domain.Membership{},
			domain.ErrForbidden
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
		params.Membership.TenantID,
		params.Membership.UserID,
		string(params.Membership.Role),
		params.Membership.Version,
		params.Membership.CreatedBy,
		params.Membership.CreatedAt,
		params.Membership.UpdatedAt,
	)
	if err != nil {
		return domain.Membership{},
			mapDatabaseError(err)
	}

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET
				version = $3,
				updated_at = $4
			WHERE tenant_id = $1::uuid
			  AND version = $2
		`,
		params.Membership.TenantID,
		params.ExpectedTenantVersion,
		params.NewTenantVersion,
		params.Membership.UpdatedAt,
	)
	if err != nil {
		return domain.Membership{}, fmt.Errorf(
			"increment tenant version: %w",
			err,
		)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Membership.TenantID,
		params.Event,
	); err != nil {
		return domain.Membership{}, err
	}

	result, err := queryMembership(
		ctx,
		transaction,
		params.Membership.TenantID,
		params.Membership.UserID,
		false,
	)
	if err != nil {
		return domain.Membership{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Membership{}, err
	}

	return result, nil
}

func lockOwners(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
) ([]string, error) {
	rows, err := transaction.Query(
		ctx,
		`
			SELECT user_id::text
			FROM tenant_memberships
			WHERE tenant_id = $1::uuid
			  AND role = 'owner'
			ORDER BY user_id
			FOR UPDATE
		`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"lock tenant owners: %w",
			err,
		)
	}
	defer rows.Close()

	var owners []string

	for rows.Next() {
		var userID string

		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf(
				"scan tenant owner: %w",
				err,
			)
		}

		owners = append(owners, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate tenant owners: %w",
			err,
		)
	}

	return owners, nil
}

// UpdateMemberRole updates a membership and protects the last owner.
func (repository *Repository) UpdateMemberRole(
	ctx context.Context,
	params ports.UpdateMemberRoleParams,
) (domain.Membership, error) {
	transaction, err := repository.beginTenant(
		ctx,
		params.Membership.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Membership{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	actorContext, err := queryContext(
		ctx,
		transaction,
		params.Membership.TenantID,
		params.ActorUserID,
		true,
	)
	if err != nil {
		return domain.Membership{}, err
	}

	target, err := queryMembership(
		ctx,
		transaction,
		params.Membership.TenantID,
		params.Membership.UserID,
		true,
	)
	if err != nil {
		return domain.Membership{}, err
	}

	if actorContext.Tenant.Status != domain.StatusActive {
		return domain.Membership{}, domain.ErrArchived
	}

	if actorContext.Tenant.Version !=
		params.ExpectedTenantVersion ||
		target.Version !=
			params.ExpectedMembershipVersion {
		return domain.Membership{},
			domain.ErrVersionConflict
	}

	if !domain.CanManageMember(
		actorContext.Membership.Role,
		target.Role,
		params.Membership.Role,
	) {
		return domain.Membership{},
			domain.ErrForbidden
	}

	if target.Role == domain.RoleOwner &&
		params.Membership.Role != domain.RoleOwner {
		owners, err := lockOwners(
			ctx,
			transaction,
			params.Membership.TenantID,
		)
		if err != nil {
			return domain.Membership{}, err
		}

		if len(owners) <= 1 {
			return domain.Membership{},
				domain.ErrLastOwner
		}
	}

	commandTag, err := transaction.Exec(
		ctx,
		`
			UPDATE tenant_memberships
			SET
				role = $4,
				version = $5,
				updated_at = $6
			WHERE tenant_id = $1::uuid
			  AND user_id = $2::uuid
			  AND version = $3
		`,
		params.Membership.TenantID,
		params.Membership.UserID,
		params.ExpectedMembershipVersion,
		string(params.Membership.Role),
		params.Membership.Version,
		params.Membership.UpdatedAt,
	)
	if err != nil {
		return domain.Membership{}, fmt.Errorf(
			"update tenant membership role: %w",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		return domain.Membership{},
			domain.ErrVersionConflict
	}

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET
				version = $3,
				updated_at = $4
			WHERE tenant_id = $1::uuid
			  AND version = $2
		`,
		params.Membership.TenantID,
		params.ExpectedTenantVersion,
		params.NewTenantVersion,
		params.Membership.UpdatedAt,
	)
	if err != nil {
		return domain.Membership{}, fmt.Errorf(
			"increment tenant version: %w",
			err,
		)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Membership.TenantID,
		params.Event,
	); err != nil {
		return domain.Membership{}, err
	}

	result, err := queryMembership(
		ctx,
		transaction,
		params.Membership.TenantID,
		params.Membership.UserID,
		false,
	)
	if err != nil {
		return domain.Membership{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Membership{}, err
	}

	return result, nil
}

// RemoveMember removes a membership while protecting the last owner.
func (repository *Repository) RemoveMember(
	ctx context.Context,
	params ports.RemoveMemberParams,
) error {
	transaction, err := repository.beginTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	actorContext, err := queryContext(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		true,
	)
	if err != nil {
		return err
	}

	target, err := queryMembership(
		ctx,
		transaction,
		params.TenantID,
		params.UserID,
		true,
	)
	if err != nil {
		return err
	}

	if actorContext.Tenant.Status != domain.StatusActive {
		return domain.ErrArchived
	}

	if actorContext.Tenant.Version !=
		params.ExpectedTenantVersion ||
		target.Version !=
			params.ExpectedMembershipVersion {
		return domain.ErrVersionConflict
	}

	if !domain.CanManageMember(
		actorContext.Membership.Role,
		target.Role,
		target.Role,
	) {
		return domain.ErrForbidden
	}

	if target.Role == domain.RoleOwner {
		owners, err := lockOwners(
			ctx,
			transaction,
			params.TenantID,
		)
		if err != nil {
			return err
		}

		if len(owners) <= 1 {
			return domain.ErrLastOwner
		}
	}

	commandTag, err := transaction.Exec(
		ctx,
		`
			DELETE FROM tenant_memberships
			WHERE tenant_id = $1::uuid
			  AND user_id = $2::uuid
			  AND version = $3
		`,
		params.TenantID,
		params.UserID,
		params.ExpectedMembershipVersion,
	)
	if err != nil {
		return fmt.Errorf(
			"remove tenant membership: %w",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		return domain.ErrVersionConflict
	}

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET
				version = $3,
				updated_at = $4
			WHERE tenant_id = $1::uuid
			  AND version = $2
		`,
		params.TenantID,
		params.ExpectedTenantVersion,
		params.NewTenantVersion,
		params.Event.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf(
			"increment tenant version: %w",
			err,
		)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.TenantID,
		params.Event,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}
