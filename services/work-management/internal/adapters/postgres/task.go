package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreateTask commits a new task and its outbox event atomically.
func (repository *Repository) CreateTask(
	ctx context.Context,
	params ports.CreateTaskParams,
) (domain.TaskView, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Task.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertTask(
		ctx,
		transaction,
		params.Task,
	); err != nil {
		return domain.TaskView{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Task.TenantID,
		params.Event,
	); err != nil {
		return domain.TaskView{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskView{}, err
	}

	return taskView(params.Task), nil
}

// GetTask returns one task with derived dependency state.
func (repository *Repository) GetTask(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) (domain.TaskView, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	task, err := queryTaskView(
		ctx,
		transaction,
		tenantID,
		taskID,
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskView{}, err
	}

	return task, nil
}

// ListTasks lists a project's tasks after the cursor.
func (repository *Repository) ListTasks(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	projectID string,
	limit int,
	cursor *ports.TaskCursor,
	includeCanceled bool,
) ([]domain.TaskView, error) {
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

	query := taskViewSelect + `
		WHERE t.tenant_id = $1::uuid
		  AND t.company_id = $2::uuid
		  AND t.project_id = $3::uuid
	`

	args := []any{tenantID, companyID, projectID}

	if cursor != nil {
		args = append(args, cursor.TaskID)
		query += fmt.Sprintf(
			" AND t.task_id > $%d::uuid",
			len(args),
		)
	}

	if !includeCanceled {
		query += ` AND t.status <> 'canceled'`
	}

	args = append(args, limit)
	query += fmt.Sprintf(
		`
			GROUP BY
				t.tenant_id,
				t.company_id,
				t.project_id,
				t.task_id,
				t.parent_task_id,
				t.title,
				t.description,
				t.status,
				t.priority,
				t.version,
				t.created_by_user_id,
				t.created_at,
				t.updated_at,
				t.completed_at,
				t.canceled_at
			ORDER BY t.task_id
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"list work tasks: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var tasks []domain.TaskView

	for rows.Next() {
		task, err := scanTaskView(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate work tasks: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return tasks, nil
}

// UpdateTask applies an optimistic-concurrency task update.
func (repository *Repository) UpdateTask(
	ctx context.Context,
	params ports.UpdateTaskParams,
) (domain.TaskView, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Task.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_tasks
			SET
				title = $4,
				description = $5,
				priority = $6,
				version = version + 1,
				updated_at = $7
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND version = $3
		`,
		params.Task.TenantID,
		params.Task.ID,
		params.ExpectedVersion,
		params.Task.Title,
		params.Task.Description,
		string(params.Task.Priority),
		params.Task.UpdatedAt,
	)
	if err != nil {
		return domain.TaskView{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectTaskVersionOrMissing(
			ctx,
			transaction,
			params.Task.TenantID,
			params.Task.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.TaskView{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Task.TenantID,
		params.Event,
	); err != nil {
		return domain.TaskView{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskView{}, err
	}

	return taskView(params.Task), nil
}

// ChangeTaskStatus applies an optimistic-concurrency task status change.
func (repository *Repository) ChangeTaskStatus(
	ctx context.Context,
	params ports.TaskChangeParams,
) (domain.TaskView, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Task.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TaskView{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if params.Task.Status == domain.TaskStatusCompleted ||
		params.Task.Status == domain.TaskStatusInProgress ||
		params.Task.Status == domain.TaskStatusWaitingApproval {
		var hasIncompleteDependencies bool

		err := transaction.QueryRow(
			ctx,
			`
				SELECT EXISTS (
					SELECT 1
					FROM work_task_dependencies AS dependency
					JOIN work_tasks AS prerequisite
					  ON prerequisite.tenant_id =
							dependency.tenant_id
					 AND prerequisite.task_id =
							dependency.depends_on_task_id
					WHERE dependency.tenant_id = $1::uuid
					  AND dependency.task_id = $2::uuid
					  AND prerequisite.status <> 'completed'
				)
			`,
			params.Task.TenantID,
			params.Task.ID,
		).Scan(&hasIncompleteDependencies)
		if err != nil {
			return domain.TaskView{}, fmt.Errorf(
				"check task incomplete dependencies: %w",
				err,
			)
		}

		if hasIncompleteDependencies {
			return domain.TaskView{}, domain.ErrTaskBlocked
		}
	}

	if params.Task.Status == domain.TaskStatusCompleted {
		var hasOpenChecklist bool

		err := transaction.QueryRow(
			ctx,
			`
				SELECT EXISTS (
					SELECT 1
					FROM work_checklist_items
					WHERE tenant_id = $1::uuid
					  AND task_id = $2::uuid
					  AND NOT completed
				)
			`,
			params.Task.TenantID,
			params.Task.ID,
		).Scan(&hasOpenChecklist)
		if err != nil {
			return domain.TaskView{}, fmt.Errorf(
				"check task open checklist: %w",
				err,
			)
		}

		if hasOpenChecklist {
			return domain.TaskView{}, domain.ErrChecklistOpen
		}
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_tasks
			SET
				status = $4,
				version = version + 1,
				updated_at = $5,
				completed_at = $6,
				canceled_at = $7
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND version = $3
		`,
		params.Task.TenantID,
		params.Task.ID,
		params.ExpectedVersion,
		string(params.Task.Status),
		params.Task.UpdatedAt,
		params.Task.CompletedAt,
		params.Task.CanceledAt,
	)
	if err != nil {
		return domain.TaskView{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectTaskVersionOrMissing(
			ctx,
			transaction,
			params.Task.TenantID,
			params.Task.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.TaskView{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Task.TenantID,
		params.Event,
	); err != nil {
		return domain.TaskView{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskView{}, err
	}

	return taskView(params.Task), nil
}

func rejectTaskVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	taskID string,
	expectedVersion int64,
) error {
	var currentVersion int64

	err := transaction.QueryRow(
		ctx,
		`
			SELECT version
			FROM work_tasks
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
		`,
		tenantID,
		taskID,
	).Scan(&currentVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query task version: %w",
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

func insertTask(
	ctx context.Context,
	transaction pgx.Tx,
	task domain.Task,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO work_tasks (
				tenant_id,
				company_id,
				project_id,
				task_id,
				parent_task_id,
				title,
				description,
				status,
				priority,
				version,
				created_by_user_id,
				created_at,
				updated_at,
				completed_at,
				canceled_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5::uuid,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11::uuid,
				$12,
				$13,
				$14,
				$15
			)
		`,
		task.TenantID,
		task.CompanyID,
		task.ProjectID,
		task.ID,
		task.ParentTaskID,
		task.Title,
		task.Description,
		string(task.Status),
		string(task.Priority),
		task.Version,
		task.CreatedByUserID,
		task.CreatedAt,
		task.UpdatedAt,
		task.CompletedAt,
		task.CanceledAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return nil
}

const taskViewSelect = `
	SELECT
		t.tenant_id::text,
		t.company_id::text,
		t.project_id::text,
		t.task_id::text,
		t.parent_task_id::text,
		t.title,
		t.description,
		t.status,
		t.priority,
		t.version,
		t.created_by_user_id::text,
		t.created_at,
		t.updated_at,
		t.completed_at,
		t.canceled_at,
		(
			SELECT count(*)::int
			FROM work_task_dependencies AS incoming
			WHERE incoming.tenant_id = t.tenant_id
			  AND incoming.task_id = t.task_id
		) AS dependency_count,
		(
			SELECT count(*)::int
			FROM work_task_dependencies AS incoming
			JOIN work_tasks AS prerequisite
			  ON prerequisite.tenant_id = incoming.tenant_id
			 AND prerequisite.task_id = incoming.depends_on_task_id
			WHERE incoming.tenant_id = t.tenant_id
			  AND incoming.task_id = t.task_id
			  AND prerequisite.status <> 'completed'
		) AS incomplete_dependency_count
	FROM work_tasks AS t
`

func queryTaskView(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	taskID string,
) (domain.TaskView, error) {
	return scanTaskView(querier.QueryRow(
		ctx,
		taskViewSelect+`
			WHERE t.tenant_id = $1::uuid
			  AND t.task_id = $2::uuid
			GROUP BY
				t.tenant_id,
				t.company_id,
				t.project_id,
				t.task_id,
				t.parent_task_id,
				t.title,
				t.description,
				t.status,
				t.priority,
				t.version,
				t.created_by_user_id,
				t.created_at,
				t.updated_at,
				t.completed_at,
				t.canceled_at
		`,
		tenantID,
		taskID,
	))
}

func scanTaskView(
	scanner rowScanner,
) (domain.TaskView, error) {
	var view domain.TaskView
	var status string
	var priority string

	err := scanner.Scan(
		&view.TenantID,
		&view.CompanyID,
		&view.ProjectID,
		&view.ID,
		&view.ParentTaskID,
		&view.Title,
		&view.Description,
		&status,
		&priority,
		&view.Version,
		&view.CreatedByUserID,
		&view.CreatedAt,
		&view.UpdatedAt,
		&view.CompletedAt,
		&view.CanceledAt,
		&view.DependencyCount,
		&view.IncompleteDependencyCount,
	)
	if err != nil {
		return domain.TaskView{}, mapDatabaseError(err)
	}

	view.Status = domain.TaskStatus(status)
	view.Priority = domain.TaskPriority(priority)
	view.Blocked = deriveBlocked(view)

	return view, nil
}

// taskView derives blocked state from the current task without dependencies.
func taskView(task domain.Task) domain.TaskView {
	view := domain.TaskView{
		Task: task,
	}

	view.DependencyCount = 0
	view.IncompleteDependencyCount = 0
	view.Blocked = deriveBlocked(view)

	return view
}

func deriveBlocked(view domain.TaskView) bool {
	if view.Status == domain.TaskStatusCompleted ||
		view.Status == domain.TaskStatusCanceled {
		return false
	}

	return view.IncompleteDependencyCount > 0
}
