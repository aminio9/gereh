package postgres

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// AssignTask commits a task assignment and its outbox event atomically.
func (repository *Repository) AssignTask(
	ctx context.Context,
	params ports.AssignTaskParams,
) (domain.TaskAssignment, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Assignment.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TaskAssignment{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := taskExists(
		ctx,
		transaction,
		params.Assignment.TenantID,
		params.Assignment.TaskID,
	); err != nil {
		return domain.TaskAssignment{}, err
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO work_task_assignments (
				tenant_id,
				task_id,
				assignment_id,
				assignee_type,
				user_id,
				agent_id,
				assignment_role,
				assigned_by_user_id,
				assigned_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4,
				$5::uuid,
				$6::uuid,
				$7,
				$8::uuid,
				$9
			)
		`,
		params.Assignment.TenantID,
		params.Assignment.TaskID,
		params.Assignment.ID,
		string(params.Assignment.AssigneeType),
		params.Assignment.UserID,
		params.Assignment.AgentID,
		string(params.Assignment.Role),
		params.Assignment.AssignedByUserID,
		params.Assignment.AssignedAt,
	)
	if err != nil {
		return domain.TaskAssignment{}, mapDatabaseError(err)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Assignment.TenantID,
		params.Event,
	); err != nil {
		return domain.TaskAssignment{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskAssignment{}, err
	}

	return params.Assignment, nil
}

// GetAssignment returns one task assignment by identity.
func (repository *Repository) GetAssignment(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	assignmentID string,
) (domain.TaskAssignment, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.TaskAssignment{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var assignment domain.TaskAssignment
	var assigneeType string
	var role string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				task_id::text,
				assignment_id::text,
				assignee_type,
				user_id::text,
				agent_id::text,
				assignment_role,
				assigned_by_user_id::text,
				assigned_at
			FROM work_task_assignments
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND assignment_id = $3::uuid
		`,
		tenantID,
		taskID,
		assignmentID,
	).Scan(
		&assignment.TenantID,
		&assignment.TaskID,
		&assignment.ID,
		&assigneeType,
		&assignment.UserID,
		&assignment.AgentID,
		&role,
		&assignment.AssignedByUserID,
		&assignment.AssignedAt,
	)
	if err != nil {
		return domain.TaskAssignment{}, mapDatabaseError(err)
	}

	assignment.AssigneeType = domain.AssigneeType(assigneeType)
	assignment.Role = domain.AssignmentRole(role)

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskAssignment{}, err
	}

	return assignment, nil
}

// UnassignTask removes a task assignment.
func (repository *Repository) UnassignTask(
	ctx context.Context,
	params ports.UnassignTaskParams,
) error {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Assignment.TenantID,
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
			DELETE FROM work_task_assignments
			WHERE tenant_id = $1::uuid
			  AND assignment_id = $2::uuid
			  AND task_id = $3::uuid
		`,
		params.Assignment.TenantID,
		params.Assignment.ID,
		params.Assignment.TaskID,
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
		params.Assignment.TenantID,
		params.Event,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}

func taskExists(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	taskID string,
) error {
	var exists bool

	err := transaction.QueryRow(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM work_tasks
				WHERE tenant_id = $1::uuid
				  AND task_id = $2::uuid
			)
		`,
		tenantID,
		taskID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf(
			"check task exists: %w",
			err,
		)
	}

	if !exists {
		return domain.ErrNotFound
	}

	return nil
}
