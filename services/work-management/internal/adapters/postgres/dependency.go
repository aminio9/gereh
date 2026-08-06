package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// AddDependency inserts a prerequisite edge after serializing dependency
// mutation per project. Both endpoints must belong to the same project.
func (repository *Repository) AddDependency(
	ctx context.Context,
	params ports.AddDependencyParams,
) (domain.TaskDependency, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Dependency.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TaskDependency{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	projectID, err := sharedProject(
		ctx,
		transaction,
		params.Dependency.TenantID,
		params.Dependency.TaskID,
		params.Dependency.DependsOnTaskID,
	)
	if err != nil {
		return domain.TaskDependency{}, err
	}

	// Serialize dependency mutations for the project so concurrent edge
	// insertions cannot race the cycle check.
	_, err = transaction.Exec(
		ctx,
		`
			SELECT 1
			FROM work_projects
			WHERE tenant_id = $1::uuid
			  AND project_id = $2::uuid
			FOR UPDATE
		`,
		params.Dependency.TenantID,
		projectID,
	)
	if err != nil {
		return domain.TaskDependency{}, fmt.Errorf(
			"lock work project for dependency: %w",
			mapDatabaseError(err),
		)
	}

	cycle, err := dependencyCycleExists(
		ctx,
		transaction,
		params.Dependency.TenantID,
		params.Dependency.DependsOnTaskID,
		params.Dependency.TaskID,
	)
	if err != nil {
		return domain.TaskDependency{}, err
	}

	if cycle {
		return domain.TaskDependency{}, domain.ErrDependencyCycle
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO work_task_dependencies (
				tenant_id,
				project_id,
				task_id,
				depends_on_task_id,
				created_by_user_id,
				created_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5::uuid,
				$6
			)
		`,
		params.Dependency.TenantID,
		projectID,
		params.Dependency.TaskID,
		params.Dependency.DependsOnTaskID,
		params.Dependency.CreatedByUserID,
		params.Dependency.CreatedAt,
	)
	if err != nil {
		return domain.TaskDependency{}, mapDatabaseError(err)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Dependency.TenantID,
		params.Event,
	); err != nil {
		return domain.TaskDependency{}, err
	}

	dependency := params.Dependency
	dependency.ProjectID = projectID

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskDependency{}, err
	}

	return dependency, nil
}

// RemoveDependency deletes one prerequisite edge.
func (repository *Repository) RemoveDependency(
	ctx context.Context,
	params ports.RemoveDependencyParams,
) error {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Dependency.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			DELETE FROM work_task_dependencies
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND depends_on_task_id = $3::uuid
		`,
		params.Dependency.TenantID,
		params.Dependency.TaskID,
		params.Dependency.DependsOnTaskID,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Dependency.TenantID,
		params.Event,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}

// sharedProject resolves both task endpoints and verifies they share one
// project.
func sharedProject(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	taskID string,
	otherTaskID string,
) (string, error) {
	var projectID string
	var otherProjectID string

	err := transaction.QueryRow(
		ctx,
		`
			SELECT project_id::text
			FROM work_tasks
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
		`,
		tenantID,
		taskID,
	).Scan(&projectID)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf(
			"query dependency task project: %w",
			err,
		)
	}

	err = transaction.QueryRow(
		ctx,
		`
			SELECT project_id::text
			FROM work_tasks
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
		`,
		tenantID,
		otherTaskID,
	).Scan(&otherProjectID)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf(
			"query dependency prerequisite project: %w",
			err,
		)
	}

	if projectID != otherProjectID {
		return "", fmt.Errorf(
			"%w: dependencies must be within one project",
			domain.ErrInvalidArgument,
		)
	}

	return projectID, nil
}

// dependencyCycleExists reports whether adding taskID as a dependent of
// prerequisiteID would create a cycle.
func dependencyCycleExists(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	prerequisiteID string,
	taskID string,
) (bool, error) {
	var cycle bool

	err := transaction.QueryRow(
		ctx,
		`
			WITH RECURSIVE chain (depends_on_task_id) AS (
				SELECT dependency.depends_on_task_id
				FROM work_task_dependencies AS dependency
				WHERE dependency.tenant_id = $1::uuid
				  AND dependency.task_id = $2::uuid

				UNION

				SELECT dependency.depends_on_task_id
				FROM work_task_dependencies AS dependency
				JOIN chain
				  ON dependency.task_id =
						chain.depends_on_task_id
				WHERE dependency.tenant_id = $1::uuid
			)
			SELECT EXISTS (
				SELECT 1
				FROM chain
				WHERE chain.depends_on_task_id = $3::uuid
			)
		`,
		tenantID,
		prerequisiteID,
		taskID,
	).Scan(&cycle)
	if err != nil {
		return false, fmt.Errorf(
			"detect dependency cycle: %w",
			err,
		)
	}

	return cycle, nil
}
