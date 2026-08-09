package postgres

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/projection/internal/domain"
)

func (transaction *projectionTransaction) UpsertDependency(
	ctx context.Context,
	value domain.Dependency,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_task_dependencies (
				tenant_id,
				project_id,
				task_id,
				depends_on_task_id,
				source_event_id,
				source_event_at,
				projected_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5::uuid,
				$6,
				$7
			)
			ON CONFLICT (
				tenant_id,
				task_id,
				depends_on_task_id
			)
			DO UPDATE SET
				project_id =
					EXCLUDED.project_id,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
		`,
		value.TenantID,
		value.ProjectID,
		value.TaskID,
		value.DependsOnTaskID,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert task dependency projection: %w",
			err,
		)
	}

	return nil
}

func (
	transaction *projectionTransaction,
) DeleteDependency(
	ctx context.Context,
	tenantID string,
	taskID string,
	dependsOnTaskID string,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			DELETE
			FROM projection_task_dependencies
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND depends_on_task_id = $3::uuid
		`,
		tenantID,
		taskID,
		dependsOnTaskID,
	)
	if err != nil {
		return fmt.Errorf(
			"delete task dependency projection: %w",
			err,
		)
	}

	return nil
}

func (transaction *projectionTransaction) UpsertAssignment(
	ctx context.Context,
	value domain.Assignment,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_task_assignments (
				tenant_id,
				task_id,
				assignment_id,
				assignee_type,
				user_id,
				agent_id,
				assignment_role,
				assigned_at,
				source_event_id,
				source_event_at,
				projected_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4,
				$5::uuid,
				$6::uuid,
				$7,
				$8,
				$9::uuid,
				$10,
				$11
			)
			ON CONFLICT (
				tenant_id,
				assignment_id
			)
			DO UPDATE SET
				task_id =
					EXCLUDED.task_id,
				assignee_type =
					EXCLUDED.assignee_type,
				user_id =
					EXCLUDED.user_id,
				agent_id =
					EXCLUDED.agent_id,
				assignment_role =
					EXCLUDED.assignment_role,
				assigned_at =
					EXCLUDED.assigned_at,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
		`,
		value.TenantID,
		value.TaskID,
		value.ID,
		value.AssigneeType,
		value.UserID,
		value.AgentID,
		value.Role,
		value.AssignedAt,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert task assignment projection: %w",
			err,
		)
	}

	return nil
}

func (
	transaction *projectionTransaction,
) DeleteAssignment(
	ctx context.Context,
	tenantID string,
	assignmentID string,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			DELETE
			FROM projection_task_assignments
			WHERE tenant_id = $1::uuid
			  AND assignment_id = $2::uuid
		`,
		tenantID,
		assignmentID,
	)
	if err != nil {
		return fmt.Errorf(
			"delete task assignment projection: %w",
			err,
		)
	}

	return nil
}
