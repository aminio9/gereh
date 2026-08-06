package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreateGoal commits a new goal and its outbox event atomically.
func (repository *Repository) CreateGoal(
	ctx context.Context,
	params ports.CreateGoalParams,
) (domain.Goal, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Goal.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Goal{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertGoal(
		ctx,
		transaction,
		params.Goal,
	); err != nil {
		return domain.Goal{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Goal.TenantID,
		params.Event,
	); err != nil {
		return domain.Goal{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Goal{}, err
	}

	return params.Goal, nil
}

// GetGoal returns one goal by identity.
func (repository *Repository) GetGoal(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	goalID string,
) (domain.Goal, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Goal{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	goal, err := queryGoal(
		ctx,
		transaction,
		tenantID,
		goalID,
	)
	if err != nil {
		return domain.Goal{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Goal{}, err
	}

	return goal, nil
}

// ListGoals lists a company's goals after the cursor.
func (repository *Repository) ListGoals(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	limit int,
	cursor *ports.GoalCursor,
	includeArchived bool,
) ([]domain.Goal, error) {
	transaction, err := repository.beginUserTenant(
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

	query := `
		SELECT
			tenant_id::text,
			company_id::text,
			goal_id::text,
			title,
			description,
			status,
			version,
			created_by_user_id::text,
			created_at,
			updated_at,
			completed_at,
			archived_at
		FROM work_goals
		WHERE tenant_id = $1::uuid
		  AND company_id = $2::uuid
	`

	args := []any{tenantID, companyID}

	if cursor != nil {
		args = append(args, cursor.GoalID)
		query += fmt.Sprintf(
			" AND goal_id > $%d::uuid",
			len(args),
		)
	}

	if !includeArchived {
		query += ` AND status <> 'archived'`
	}

	args = append(args, limit)
	query += fmt.Sprintf(
		`
			ORDER BY goal_id
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"list work goals: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var goals []domain.Goal

	for rows.Next() {
		goal, err := scanGoal(rows)
		if err != nil {
			return nil, err
		}

		goals = append(goals, goal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate work goals: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return goals, nil
}

// UpdateGoal applies an optimistic-concurrency goal update.
func (repository *Repository) UpdateGoal(
	ctx context.Context,
	params ports.UpdateGoalParams,
) (domain.Goal, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Goal.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Goal{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_goals
			SET
				title = $4,
				description = $5,
				version = version + 1,
				updated_at = $6
			WHERE tenant_id = $1::uuid
			  AND goal_id = $2::uuid
			  AND version = $3
		`,
		params.Goal.TenantID,
		params.Goal.ID,
		params.ExpectedVersion,
		params.Goal.Title,
		params.Goal.Description,
		params.Goal.UpdatedAt,
	)
	if err != nil {
		return domain.Goal{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectGoalVersionOrMissing(
			ctx,
			transaction,
			params.Goal.TenantID,
			params.Goal.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Goal{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Goal.TenantID,
		params.Event,
	); err != nil {
		return domain.Goal{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Goal{}, err
	}

	return params.Goal, nil
}

// ChangeGoalStatus applies an optimistic-concurrency goal status change.
func (repository *Repository) ChangeGoalStatus(
	ctx context.Context,
	params ports.UpdateGoalParams,
) (domain.Goal, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Goal.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Goal{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if params.Goal.Status == domain.GoalStatusCompleted ||
		params.Goal.Status == domain.GoalStatusArchived {
		var hasOpenProjects bool

		err := transaction.QueryRow(
			ctx,
			`
				SELECT EXISTS (
					SELECT 1
					FROM work_projects
					WHERE tenant_id = $1::uuid
					  AND company_id = $2::uuid
					  AND status NOT IN (
							'completed',
							'canceled',
							'archived'
					  )
				)
			`,
			params.Goal.TenantID,
			params.Goal.CompanyID,
		).Scan(&hasOpenProjects)
		if err != nil {
			return domain.Goal{}, fmt.Errorf(
				"check goal open projects: %w",
				err,
			)
		}

		if hasOpenProjects {
			return domain.Goal{}, domain.ErrGoalOpenProjects
		}
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_goals
			SET
				status = $4,
				version = version + 1,
				updated_at = $5,
				completed_at = $6,
				archived_at = $7
			WHERE tenant_id = $1::uuid
			  AND goal_id = $2::uuid
			  AND version = $3
		`,
		params.Goal.TenantID,
		params.Goal.ID,
		params.ExpectedVersion,
		string(params.Goal.Status),
		params.Goal.UpdatedAt,
		params.Goal.CompletedAt,
		params.Goal.ArchivedAt,
	)
	if err != nil {
		return domain.Goal{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectGoalVersionOrMissing(
			ctx,
			transaction,
			params.Goal.TenantID,
			params.Goal.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Goal{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Goal.TenantID,
		params.Event,
	); err != nil {
		return domain.Goal{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Goal{}, err
	}

	return params.Goal, nil
}

func rejectGoalVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	goalID string,
	expectedVersion int64,
) error {
	var currentVersion int64

	err := transaction.QueryRow(
		ctx,
		`
			SELECT version
			FROM work_goals
			WHERE tenant_id = $1::uuid
			  AND goal_id = $2::uuid
		`,
		tenantID,
		goalID,
	).Scan(&currentVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query goal version: %w",
			err,
		)
	}

	return fmt.Errorf(
		"%w: expected %d, found %d",
		domain.ErrVersionConflict,
		expectedVersion,
		currentVersion,
	)
}

func insertGoal(
	ctx context.Context,
	transaction pgx.Tx,
	goal domain.Goal,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO work_goals (
				tenant_id,
				company_id,
				goal_id,
				title,
				description,
				status,
				version,
				created_by_user_id,
				created_at,
				updated_at,
				completed_at,
				archived_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4,
				$5,
				$6,
				$7,
				$8::uuid,
				$9,
				$10,
				$11,
				$12
			)
		`,
		goal.TenantID,
		goal.CompanyID,
		goal.ID,
		goal.Title,
		goal.Description,
		string(goal.Status),
		goal.Version,
		goal.CreatedByUserID,
		goal.CreatedAt,
		goal.UpdatedAt,
		goal.CompletedAt,
		goal.ArchivedAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return nil
}

func queryGoal(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	goalID string,
) (domain.Goal, error) {
	return scanGoal(querier.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				company_id::text,
				goal_id::text,
				title,
				description,
				status,
				version,
				created_by_user_id::text,
				created_at,
				updated_at,
				completed_at,
				archived_at
			FROM work_goals
			WHERE tenant_id = $1::uuid
			  AND goal_id = $2::uuid
		`,
		tenantID,
		goalID,
	))
}

func scanGoal(
	scanner rowScanner,
) (domain.Goal, error) {
	var goal domain.Goal
	var status string

	err := scanner.Scan(
		&goal.TenantID,
		&goal.CompanyID,
		&goal.ID,
		&goal.Title,
		&goal.Description,
		&status,
		&goal.Version,
		&goal.CreatedByUserID,
		&goal.CreatedAt,
		&goal.UpdatedAt,
		&goal.CompletedAt,
		&goal.ArchivedAt,
	)
	if err != nil {
		return domain.Goal{}, mapDatabaseError(err)
	}

	goal.Status = domain.GoalStatus(status)

	return goal, nil
}
