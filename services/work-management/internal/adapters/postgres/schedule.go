package postgres

import (
	"context"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// UpsertSchedule inserts or replaces a task schedule and its outbox event.
func (repository *Repository) UpsertSchedule(
	ctx context.Context,
	params ports.UpsertScheduleParams,
) (domain.TaskSchedule, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Schedule.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.TaskSchedule{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := taskExists(
		ctx,
		transaction,
		params.Schedule.TenantID,
		params.Schedule.TaskID,
	); err != nil {
		return domain.TaskSchedule{}, err
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO work_task_schedules (
				tenant_id,
				task_id,
				not_before,
				due_at,
				timezone,
				version,
				updated_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4,
				$5,
				$6,
				$7
			)
			ON CONFLICT (tenant_id, task_id)
			DO UPDATE SET
				not_before = EXCLUDED.not_before,
				due_at = EXCLUDED.due_at,
				timezone = EXCLUDED.timezone,
				version = work_task_schedules.version + 1,
				updated_at = EXCLUDED.updated_at
		`,
		params.Schedule.TenantID,
		params.Schedule.TaskID,
		params.Schedule.NotBefore,
		params.Schedule.DueAt,
		params.Schedule.Timezone,
		params.Schedule.Version,
		params.Schedule.UpdatedAt,
	)
	if err != nil {
		return domain.TaskSchedule{}, mapDatabaseError(err)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Schedule.TenantID,
		params.Event,
	); err != nil {
		return domain.TaskSchedule{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskSchedule{}, err
	}

	return params.Schedule, nil
}

// DeleteSchedule removes a task schedule.
func (repository *Repository) DeleteSchedule(
	ctx context.Context,
	params ports.DeleteScheduleParams,
) error {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Schedule.TenantID,
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
			DELETE FROM work_task_schedules
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
		`,
		params.Schedule.TenantID,
		params.Schedule.TaskID,
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
		params.Schedule.TenantID,
		params.Event,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}

// GetSchedule returns one task schedule.
func (repository *Repository) GetSchedule(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) (domain.TaskSchedule, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.TaskSchedule{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var schedule domain.TaskSchedule

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				task_id::text,
				not_before,
				due_at,
				timezone,
				version,
				updated_at
			FROM work_task_schedules
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
		`,
		tenantID,
		taskID,
	).Scan(
		&schedule.TenantID,
		&schedule.TaskID,
		&schedule.NotBefore,
		&schedule.DueAt,
		&schedule.Timezone,
		&schedule.Version,
		&schedule.UpdatedAt,
	)
	if err != nil {
		return domain.TaskSchedule{}, mapDatabaseError(err)
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.TaskSchedule{}, err
	}

	return schedule, nil
}
