package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreateProject commits a new project and its outbox event atomically.
func (repository *Repository) CreateProject(
	ctx context.Context,
	params ports.CreateProjectParams,
) (domain.Project, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Project.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Project{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertProject(
		ctx,
		transaction,
		params.Project,
	); err != nil {
		return domain.Project{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Project.TenantID,
		params.Event,
	); err != nil {
		return domain.Project{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Project{}, err
	}

	return params.Project, nil
}

// GetProject returns one project by identity.
func (repository *Repository) GetProject(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	projectID string,
) (domain.Project, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Project{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	project, err := queryProject(
		ctx,
		transaction,
		tenantID,
		projectID,
	)
	if err != nil {
		return domain.Project{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Project{}, err
	}

	return project, nil
}

// ListProjects lists a company's projects after the cursor, optionally
// filtered by goal.
func (repository *Repository) ListProjects(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	goalID string,
	limit int,
	cursor *ports.ProjectCursor,
	includeArchived bool,
) ([]domain.Project, error) {
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
			project_id::text,
			title,
			description,
			status,
			version,
			created_by_user_id::text,
			created_at,
			updated_at,
			completed_at,
			archived_at
		FROM work_projects
		WHERE tenant_id = $1::uuid
		  AND company_id = $2::uuid
	`

	args := []any{tenantID, companyID}

	if goalID != "" {
		args = append(args, goalID)
		query += fmt.Sprintf(
			" AND goal_id = $%d::uuid",
			len(args),
		)
	}

	if cursor != nil {
		args = append(args, cursor.ProjectID)
		query += fmt.Sprintf(
			" AND project_id > $%d::uuid",
			len(args),
		)
	}

	if !includeArchived {
		query += ` AND status <> 'archived'`
	}

	args = append(args, limit)
	query += fmt.Sprintf(
		`
			ORDER BY project_id
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"list work projects: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var projects []domain.Project

	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate work projects: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return projects, nil
}

// UpdateProject applies an optimistic-concurrency project update.
func (repository *Repository) UpdateProject(
	ctx context.Context,
	params ports.UpdateProjectParams,
) (domain.Project, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Project.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Project{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_projects
			SET
				title = $4,
				description = $5,
				version = version + 1,
				updated_at = $6
			WHERE tenant_id = $1::uuid
			  AND project_id = $2::uuid
			  AND version = $3
		`,
		params.Project.TenantID,
		params.Project.ID,
		params.ExpectedVersion,
		params.Project.Title,
		params.Project.Description,
		params.Project.UpdatedAt,
	)
	if err != nil {
		return domain.Project{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectProjectVersionOrMissing(
			ctx,
			transaction,
			params.Project.TenantID,
			params.Project.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Project{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Project.TenantID,
		params.Event,
	); err != nil {
		return domain.Project{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Project{}, err
	}

	return params.Project, nil
}

// ChangeProjectStatus applies an optimistic-concurrency project status change.
func (repository *Repository) ChangeProjectStatus(
	ctx context.Context,
	params ports.UpdateProjectParams,
) (domain.Project, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Project.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Project{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if params.Project.Status == domain.ProjectStatusCompleted ||
		params.Project.Status == domain.ProjectStatusArchived {
		var hasOpenTasks bool

		err := transaction.QueryRow(
			ctx,
			`
				SELECT EXISTS (
					SELECT 1
					FROM work_tasks
					WHERE tenant_id = $1::uuid
					  AND project_id = $2::uuid
					  AND status NOT IN (
							'completed',
							'canceled'
					  )
				)
			`,
			params.Project.TenantID,
			params.Project.ID,
		).Scan(&hasOpenTasks)
		if err != nil {
			return domain.Project{}, fmt.Errorf(
				"check project open tasks: %w",
				err,
			)
		}

		if hasOpenTasks {
			return domain.Project{}, domain.ErrProjectOpenTasks
		}
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_projects
			SET
				status = $4,
				version = version + 1,
				updated_at = $5,
				completed_at = $6,
				archived_at = $7
			WHERE tenant_id = $1::uuid
			  AND project_id = $2::uuid
			  AND version = $3
		`,
		params.Project.TenantID,
		params.Project.ID,
		params.ExpectedVersion,
		string(params.Project.Status),
		params.Project.UpdatedAt,
		params.Project.CompletedAt,
		params.Project.ArchivedAt,
	)
	if err != nil {
		return domain.Project{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectProjectVersionOrMissing(
			ctx,
			transaction,
			params.Project.TenantID,
			params.Project.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Project{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Project.TenantID,
		params.Event,
	); err != nil {
		return domain.Project{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Project{}, err
	}

	return params.Project, nil
}

func rejectProjectVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	projectID string,
	expectedVersion int64,
) error {
	var currentVersion int64

	err := transaction.QueryRow(
		ctx,
		`
			SELECT version
			FROM work_projects
			WHERE tenant_id = $1::uuid
			  AND project_id = $2::uuid
		`,
		tenantID,
		projectID,
	).Scan(&currentVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query project version: %w",
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

func insertProject(
	ctx context.Context,
	transaction pgx.Tx,
	project domain.Project,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO work_projects (
				tenant_id,
				company_id,
				goal_id,
				project_id,
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
				$4::uuid,
				$5,
				$6,
				$7,
				$8,
				$9::uuid,
				$10,
				$11,
				$12,
				$13
			)
		`,
		project.TenantID,
		project.CompanyID,
		project.GoalID,
		project.ID,
		project.Title,
		project.Description,
		string(project.Status),
		project.Version,
		project.CreatedByUserID,
		project.CreatedAt,
		project.UpdatedAt,
		project.CompletedAt,
		project.ArchivedAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return nil
}

func queryProject(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	projectID string,
) (domain.Project, error) {
	return scanProject(querier.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				company_id::text,
				goal_id::text,
				project_id::text,
				title,
				description,
				status,
				version,
				created_by_user_id::text,
				created_at,
				updated_at,
				completed_at,
				archived_at
			FROM work_projects
			WHERE tenant_id = $1::uuid
			  AND project_id = $2::uuid
		`,
		tenantID,
		projectID,
	))
}

func scanProject(
	scanner rowScanner,
) (domain.Project, error) {
	var project domain.Project
	var status string

	err := scanner.Scan(
		&project.TenantID,
		&project.CompanyID,
		&project.GoalID,
		&project.ID,
		&project.Title,
		&project.Description,
		&status,
		&project.Version,
		&project.CreatedByUserID,
		&project.CreatedAt,
		&project.UpdatedAt,
		&project.CompletedAt,
		&project.ArchivedAt,
	)
	if err != nil {
		return domain.Project{}, mapDatabaseError(err)
	}

	project.Status = domain.ProjectStatus(status)

	return project, nil
}
