package postgres

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/projection/internal/domain"
)

func (transaction *projectionTransaction) UpsertTenant(
	ctx context.Context,
	value domain.Tenant,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_tenants (
				tenant_id,
				slug,
				display_name,
				status,
				region,
				retention_days,
				source_version,
				source_event_id,
				source_event_at,
				projected_at
			)
			VALUES (
				$1::uuid,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8::uuid,
				$9,
				$10
			)
			ON CONFLICT (tenant_id)
			DO UPDATE SET
				slug = EXCLUDED.slug,
				display_name =
					EXCLUDED.display_name,
				status = EXCLUDED.status,
				region = EXCLUDED.region,
				retention_days =
					EXCLUDED.retention_days,
				source_version =
					EXCLUDED.source_version,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
			WHERE
				projection_tenants.source_version
				<
				EXCLUDED.source_version
		`,
		value.ID,
		value.Slug,
		value.DisplayName,
		value.Status,
		value.Region,
		value.RetentionDays,
		value.SourceVersion,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert tenant projection: %w",
			err,
		)
	}

	return nil
}

func (transaction *projectionTransaction) UpsertCompany(
	ctx context.Context,
	value domain.Company,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_companies (
				tenant_id,
				company_id,
				slug,
				display_name,
				description,
				status,
				is_default,
				source_version,
				source_event_id,
				source_event_at,
				projected_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9::uuid,
				$10,
				$11
			)
			ON CONFLICT (
				tenant_id,
				company_id
			)
			DO UPDATE SET
				slug = EXCLUDED.slug,
				display_name =
					EXCLUDED.display_name,
				description =
					EXCLUDED.description,
				status = EXCLUDED.status,
				is_default =
					EXCLUDED.is_default,
				source_version =
					EXCLUDED.source_version,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
			WHERE
				projection_companies.source_version
				<
				EXCLUDED.source_version
		`,
		value.TenantID,
		value.ID,
		value.Slug,
		value.DisplayName,
		value.Description,
		value.Status,
		value.IsDefault,
		value.SourceVersion,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert company projection: %w",
			err,
		)
	}

	return nil
}

func (transaction *projectionTransaction) UpsertAgent(
	ctx context.Context,
	value domain.Agent,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_agents (
				tenant_id,
				company_id,
				agent_id,
				slug,
				display_name,
				role_title,
				objective,
				manager_agent_id,
				status,
				execution_profile,
				autonomy_level,
				source_version,
				source_event_id,
				source_event_at,
				projected_at
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
				$12,
				$13::uuid,
				$14,
				$15
			)
			ON CONFLICT (
				tenant_id,
				agent_id
			)
			DO UPDATE SET
				company_id =
					EXCLUDED.company_id,
				slug = EXCLUDED.slug,
				display_name =
					EXCLUDED.display_name,
				role_title =
					EXCLUDED.role_title,
				objective =
					EXCLUDED.objective,
				manager_agent_id =
					EXCLUDED.manager_agent_id,
				status =
					EXCLUDED.status,
				execution_profile =
					EXCLUDED.execution_profile,
				autonomy_level =
					EXCLUDED.autonomy_level,
				source_version =
					EXCLUDED.source_version,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
			WHERE
				projection_agents.source_version
				<
				EXCLUDED.source_version
		`,
		value.TenantID,
		value.CompanyID,
		value.ID,
		value.Slug,
		value.DisplayName,
		value.RoleTitle,
		value.Objective,
		value.ManagerAgentID,
		value.Status,
		value.ExecutionProfile,
		value.AutonomyLevel,
		value.SourceVersion,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert agent projection: %w",
			err,
		)
	}

	return nil
}

func (transaction *projectionTransaction) UpsertGoal(
	ctx context.Context,
	value domain.Goal,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_goals (
				tenant_id,
				company_id,
				goal_id,
				title,
				description,
				status,
				source_version,
				source_event_id,
				source_event_at,
				projected_at
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
				$10
			)
			ON CONFLICT (
				tenant_id,
				goal_id
			)
			DO UPDATE SET
				company_id =
					EXCLUDED.company_id,
				title =
					EXCLUDED.title,
				description =
					EXCLUDED.description,
				status =
					EXCLUDED.status,
				source_version =
					EXCLUDED.source_version,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
			WHERE
				projection_goals.source_version
				<
				EXCLUDED.source_version
		`,
		value.TenantID,
		value.CompanyID,
		value.ID,
		value.Title,
		value.Description,
		value.Status,
		value.SourceVersion,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert goal projection: %w",
			err,
		)
	}

	return nil
}

func (transaction *projectionTransaction) UpsertProject(
	ctx context.Context,
	value domain.Project,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_projects (
				tenant_id,
				company_id,
				goal_id,
				project_id,
				title,
				description,
				status,
				source_version,
				source_event_id,
				source_event_at,
				projected_at
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
				$11
			)
			ON CONFLICT (
				tenant_id,
				project_id
			)
			DO UPDATE SET
				company_id =
					EXCLUDED.company_id,
				goal_id =
					EXCLUDED.goal_id,
				title =
					EXCLUDED.title,
				description =
					EXCLUDED.description,
				status =
					EXCLUDED.status,
				source_version =
					EXCLUDED.source_version,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
			WHERE
				projection_projects.source_version
				<
				EXCLUDED.source_version
		`,
		value.TenantID,
		value.CompanyID,
		value.GoalID,
		value.ID,
		value.Title,
		value.Description,
		value.Status,
		value.SourceVersion,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert project projection: %w",
			err,
		)
	}

	return nil
}

func (transaction *projectionTransaction) UpsertTask(
	ctx context.Context,
	value domain.Task,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_tasks (
				tenant_id,
				company_id,
				project_id,
				task_id,
				parent_task_id,
				title,
				description,
				status,
				priority,
				created_by_user_id,
				source_version,
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
				$7,
				$8,
				$9,
				$10::uuid,
				$11,
				$12::uuid,
				$13,
				$14
			)
			ON CONFLICT (
				tenant_id,
				task_id
			)
			DO UPDATE SET
				company_id =
					EXCLUDED.company_id,
				project_id =
					EXCLUDED.project_id,
				parent_task_id =
					EXCLUDED.parent_task_id,
				title =
					EXCLUDED.title,
				description =
					EXCLUDED.description,
				status =
					EXCLUDED.status,
				priority =
					EXCLUDED.priority,
				created_by_user_id =
					EXCLUDED.created_by_user_id,
				source_version =
					EXCLUDED.source_version,
				source_event_id =
					EXCLUDED.source_event_id,
				source_event_at =
					EXCLUDED.source_event_at,
				projected_at =
					EXCLUDED.projected_at
			WHERE
				projection_tasks.source_version
				<
				EXCLUDED.source_version
		`,
		value.TenantID,
		value.CompanyID,
		value.ProjectID,
		value.ID,
		value.ParentTaskID,
		value.Title,
		value.Description,
		value.Status,
		value.Priority,
		value.CreatedByUserID,
		value.SourceVersion,
		value.SourceEventID,
		value.SourceEventAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert task projection: %w",
			err,
		)
	}

	return nil
}
