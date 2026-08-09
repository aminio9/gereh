package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/aminio9/gereh/services/projection/internal/ports"
	"github.com/jackc/pgx/v5"
)

// GetDashboardSummary returns the tenant dashboard read model together
// with a tenant watermark.
func (repository *Repository) GetDashboardSummary(
	ctx context.Context,
	actorUserID string,
	tenantID string,
) (
	domain.DashboardSummary,
	domain.ProjectionMetadata,
	error,
) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{
				AccessMode: pgx.ReadOnly,
			},
		)
	if err != nil {
		return domain.DashboardSummary{},
			domain.ProjectionMetadata{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var summary domain.DashboardSummary

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				(
					SELECT COUNT(*)
					FROM projection_companies
				) AS companies_total,
				(
					SELECT COUNT(*)
					FROM projection_companies
					WHERE status = 'active'
				) AS companies_active,
				(
					SELECT COUNT(*)
					FROM projection_agents
					WHERE status <> 'deleted'
				) AS agents_total,
				(
					SELECT COUNT(*)
					FROM projection_agents
					WHERE status = 'ready'
				) AS agents_ready,
				(
					SELECT COUNT(*)
					FROM projection_agents
					WHERE status = 'degraded'
				) AS agents_degraded,
				(
					SELECT COUNT(*)
					FROM projection_agents
					WHERE status = 'paused'
				) AS agents_paused,
				(
					SELECT COUNT(*)
					FROM projection_agents
					WHERE status = 'failed'
				) AS agents_failed,
				(
					SELECT COUNT(*)
					FROM projection_goals
					WHERE status = 'active'
				) AS goals_active,
				(
					SELECT COUNT(*)
					FROM projection_goals
					WHERE status = 'completed'
				) AS goals_completed,
				(
					SELECT COUNT(*)
					FROM projection_projects
					WHERE status = 'active'
				) AS projects_active,
				(
					SELECT COUNT(*)
					FROM projection_projects
					WHERE status = 'on_hold'
				) AS projects_on_hold,
				(
					SELECT COUNT(*)
					FROM projection_projects
					WHERE status = 'completed'
				) AS projects_completed,
				(
					SELECT COUNT(*)
					FROM projection_tasks
				) AS tasks_total,
				(
					SELECT COUNT(*)
					FROM projection_tasks
					WHERE status = 'backlog'
				) AS tasks_backlog,
				(
					SELECT COUNT(*)
					FROM projection_tasks
					WHERE status = 'ready'
				) AS tasks_ready,
				(
					SELECT COUNT(*)
					FROM projection_tasks
					WHERE status = 'in_progress'
				) AS tasks_in_progress,
				(
					SELECT COUNT(*)
					FROM projection_tasks
					WHERE status = 'waiting_approval'
				) AS tasks_waiting_approval,
				(
					SELECT COUNT(*)
					FROM projection_tasks
					WHERE status = 'completed'
				) AS tasks_completed,
				(
					SELECT COUNT(*)
					FROM projection_tasks
					WHERE status = 'canceled'
				) AS tasks_canceled,
				(
					SELECT COUNT(DISTINCT
						task.task_id)
					FROM projection_tasks task
					JOIN
						projection_task_dependencies
						dependency
						ON dependency.tenant_id
							= task.tenant_id
						AND dependency.task_id
							= task.task_id
					JOIN projection_tasks
						prerequisite
						ON prerequisite.tenant_id
							= dependency.tenant_id
						AND prerequisite.task_id
							= dependency.depends_on_task_id
					WHERE prerequisite.status
							<> 'completed'
					  AND task.status
							NOT IN (
								'completed',
								'canceled'
							)
				) AS tasks_blocked
		`,
	).Scan(
		&summary.CompaniesTotal,
		&summary.CompaniesActive,
		&summary.AgentsTotal,
		&summary.AgentsReady,
		&summary.AgentsDegraded,
		&summary.AgentsPaused,
		&summary.AgentsFailed,
		&summary.GoalsActive,
		&summary.GoalsCompleted,
		&summary.ProjectsActive,
		&summary.ProjectsOnHold,
		&summary.ProjectsCompleted,
		&summary.TasksTotal,
		&summary.TasksBacklog,
		&summary.TasksReady,
		&summary.TasksInProgress,
		&summary.TasksWaitingApproval,
		&summary.TasksCompleted,
		&summary.TasksCanceled,
		&summary.TasksBlocked,
	)
	if err != nil {
		return domain.DashboardSummary{},
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"query dashboard summary: %w",
				err,
			)
	}

	metadata, err :=
		readTenantWatermark(
			ctx,
			transaction,
			tenantID,
		)
	if err != nil {
		return domain.DashboardSummary{},
			domain.ProjectionMetadata{},
			err
	}

	if err := transaction.Commit(ctx); err != nil {
		return domain.DashboardSummary{},
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"commit dashboard summary query: %w",
				err,
			)
	}

	return summary, metadata, nil
}

// GetCompanyOverview returns the company read model together with a
// tenant watermark.
func (repository *Repository) GetCompanyOverview(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) (
	domain.CompanyOverview,
	domain.ProjectionMetadata,
	error,
) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{
				AccessMode: pgx.ReadOnly,
			},
		)
	if err != nil {
		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var overview domain.CompanyOverview

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				company.tenant_id::text,
				company.company_id::text,
				company.slug,
				company.display_name,
				company.status,
				company.is_default,
				(
					SELECT COUNT(*)
					FROM projection_agents agent
					WHERE agent.company_id = company.company_id
					  AND agent.status <> 'deleted'
				) AS agents_total,
				(
					SELECT COUNT(*)
					FROM projection_agents agent
					WHERE agent.company_id = company.company_id
					  AND agent.status = 'ready'
				) AS agents_ready,
				(
					SELECT COUNT(*)
					FROM projection_agents agent
					WHERE agent.company_id = company.company_id
					  AND agent.status = 'degraded'
				) AS agents_degraded,
				(
					SELECT COUNT(*)
					FROM projection_agents agent
					WHERE agent.company_id = company.company_id
					  AND agent.status = 'paused'
				) AS agents_paused,
				(
					SELECT COUNT(*)
					FROM projection_tasks task
					WHERE task.company_id = company.company_id
				) AS tasks_total,
				(
					SELECT COUNT(*)
					FROM projection_tasks task
					WHERE task.company_id = company.company_id
					  AND task.status = 'ready'
				) AS tasks_ready,
				(
					SELECT COUNT(*)
					FROM projection_tasks task
					WHERE task.company_id = company.company_id
					  AND task.status = 'in_progress'
				) AS tasks_in_progress,
				(
					SELECT COUNT(*)
					FROM projection_tasks task
					WHERE task.company_id = company.company_id
					  AND task.status = 'waiting_approval'
				) AS tasks_waiting_approval,
				(
					SELECT COUNT(*)
					FROM projection_tasks task
					WHERE task.company_id = company.company_id
					  AND task.status = 'completed'
				) AS tasks_completed,
				(
					SELECT COUNT(DISTINCT
						task.task_id)
					FROM projection_tasks task
					JOIN
						projection_task_dependencies
						dependency
						ON dependency.tenant_id
							= task.tenant_id
						AND dependency.task_id
							= task.task_id
					JOIN projection_tasks
						prerequisite
						ON prerequisite.tenant_id
							= dependency.tenant_id
						AND prerequisite.task_id
							= dependency.depends_on_task_id
					WHERE task.company_id
							= company.company_id
					  AND prerequisite.status
							<> 'completed'
					  AND task.status
							NOT IN (
								'completed',
								'canceled'
							)
				) AS tasks_blocked
			FROM projection_companies company
			WHERE company.company_id = $1::uuid
		`,
		companyID,
	).Scan(
		&overview.TenantID,
		&overview.CompanyID,
		&overview.Slug,
		&overview.DisplayName,
		&overview.Status,
		&overview.IsDefault,
		&overview.AgentsTotal,
		&overview.AgentsReady,
		&overview.AgentsDegraded,
		&overview.AgentsPaused,
		&overview.TasksTotal,
		&overview.TasksReady,
		&overview.TasksInProgress,
		&overview.TasksWaitingApproval,
		&overview.TasksCompleted,
		&overview.TasksBlocked,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CompanyOverview{},
				domain.ProjectionMetadata{},
				domain.ErrNotFound
		}

		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"query company overview: %w",
				err,
			)
	}

	metadata, err :=
		readTenantWatermark(
			ctx,
			transaction,
			tenantID,
		)
	if err != nil {
		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			err
	}

	if err := transaction.Commit(ctx); err != nil {
		return domain.CompanyOverview{},
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"commit company overview query: %w",
				err,
			)
	}

	return overview, metadata, nil
}

// ListAgentOverviews returns a page of agent read models ordered by
// most recently projected first.
func (repository *Repository) ListAgentOverviews(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID *string,
	pageSize int,
	cursor *ports.AgentCursor,
) (
	[]domain.AgentOverview,
	*ports.AgentCursor,
	domain.ProjectionMetadata,
	error,
) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{
				AccessMode: pgx.ReadOnly,
			},
		)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	hasCursor := false
	var cursorUpdatedAt interface{}
	var cursorAgentID interface{}

	if cursor != nil {
		hasCursor = true
		cursorUpdatedAt = cursor.UpdatedAt
		cursorAgentID = cursor.AgentID
	}

	fetchLimit := pageSize + 1

	rows, err := transaction.Query(
		ctx,
		`
			SELECT
				agent.tenant_id::text,
				agent.company_id::text,
				agent.agent_id::text,
				agent.slug,
				agent.display_name,
				agent.role_title,
				agent.status,
				agent.manager_agent_id::text,
				agent.projected_at,
				COUNT(
					DISTINCT assignment.assignment_id
				) AS assigned_task_count,
				COUNT(
					DISTINCT task.task_id
				) FILTER (
					WHERE task.status IN (
						'ready',
						'in_progress',
						'waiting_approval'
					)
				) AS active_task_count
			FROM projection_agents agent
			LEFT JOIN projection_task_assignments assignment
				ON assignment.agent_id = agent.agent_id
			LEFT JOIN projection_tasks task
				ON task.task_id = assignment.task_id
			WHERE (
				$1::uuid IS NULL
				OR agent.company_id = $1::uuid
			)
			AND (
				$2::boolean IS FALSE
				OR (
					agent.projected_at,
					agent.agent_id
				)
				<
				(
					$3::timestamptz,
					$4::uuid
				)
			)
			GROUP BY agent.tenant_id, agent.company_id,
				agent.agent_id, agent.slug,
				agent.display_name, agent.role_title,
				agent.status, agent.manager_agent_id,
				agent.projected_at
			ORDER BY agent.projected_at DESC,
				agent.agent_id DESC
			LIMIT $5
		`,
		companyID,
		hasCursor,
		cursorUpdatedAt,
		cursorAgentID,
		fetchLimit,
	)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"query agent overviews: %w",
				err,
			)
	}

	defer rows.Close()

	previousCursor := cursor

	overviews := make(
		[]domain.AgentOverview,
		0,
		pageSize,
	)

	for rows.Next() {
		var overview domain.AgentOverview
		var managerAgentID *string

		if err := rows.Scan(
			&overview.TenantID,
			&overview.CompanyID,
			&overview.AgentID,
			&overview.Slug,
			&overview.DisplayName,
			&overview.RoleTitle,
			&overview.Status,
			&managerAgentID,
			&overview.UpdatedAt,
			&overview.AssignedTaskCount,
			&overview.ActiveTaskCount,
		); err != nil {
			return nil, nil,
				domain.ProjectionMetadata{},
				fmt.Errorf(
					"scan agent overview: %w",
					err,
				)
		}

		overview.ManagerAgentID = managerAgentID

		overviews = append(overviews, overview)
	}

	if err := rows.Err(); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"iterate agent overviews: %w",
				err,
			)
	}

	var nextCursor *ports.AgentCursor

	if len(overviews) > pageSize {
		overviews = overviews[:pageSize]

		last := overviews[len(overviews)-1]

		nextCursor = &ports.AgentCursor{
			UpdatedAt: last.UpdatedAt,
			AgentID:   last.AgentID,
		}
	}

	if nextCursor == nil {
		nextCursor = previousCursor
	}

	metadata, err :=
		readTenantWatermark(
			ctx,
			transaction,
			tenantID,
		)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := transaction.Commit(ctx); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"commit agent overviews query: %w",
				err,
			)
	}

	return overviews, nextCursor, metadata, nil
}

// ListTaskActivity returns a page of task activity feed items ordered
// by most recent occurrence first.
func (repository *Repository) ListTaskActivity(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID *string,
	taskID *string,
	pageSize int,
	cursor *ports.ActivityCursor,
) (
	[]domain.Activity,
	*ports.ActivityCursor,
	domain.ProjectionMetadata,
	error,
) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{
				AccessMode: pgx.ReadOnly,
			},
		)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	hasCursor := false
	var cursorOccurredAt interface{}
	var cursorEventID interface{}

	if cursor != nil {
		hasCursor = true
		cursorOccurredAt = cursor.OccurredAt
		cursorEventID = cursor.EventID
	}

	fetchLimit := pageSize + 1

	rows, err := transaction.Query(
		ctx,
		`
			SELECT
				tenant_id::text,
				event_id::text,
				event_type,
				company_id::text,
				project_id::text,
				task_id::text,
				actor_type,
				actor_id::text,
				summary,
				occurred_at,
				projected_at
			FROM projection_task_activity
			WHERE (
				$1::uuid IS NULL
				OR company_id = $1::uuid
			)
			AND (
				$2::uuid IS NULL
				OR task_id = $2::uuid
			)
			AND (
				$3::boolean IS FALSE
				OR (
					occurred_at,
					event_id
				)
				<
				(
					$4::timestamptz,
					$5::uuid
				)
			)
			ORDER BY occurred_at DESC,
				event_id DESC
			LIMIT $6
		`,
		companyID,
		taskID,
		hasCursor,
		cursorOccurredAt,
		cursorEventID,
		fetchLimit,
	)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"query task activity: %w",
				err,
			)
	}

	defer rows.Close()

	previousCursor := cursor

	activity := make(
		[]domain.Activity,
		0,
		pageSize,
	)

	for rows.Next() {
		var item domain.Activity

		if err := rows.Scan(
			&item.TenantID,
			&item.EventID,
			&item.EventType,
			&item.CompanyID,
			&item.ProjectID,
			&item.TaskID,
			&item.ActorType,
			&item.ActorID,
			&item.Summary,
			&item.OccurredAt,
			&item.ProjectedAt,
		); err != nil {
			return nil, nil,
				domain.ProjectionMetadata{},
				fmt.Errorf(
					"scan task activity: %w",
					err,
				)
		}

		activity = append(activity, item)
	}

	if err := rows.Err(); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"iterate task activity: %w",
				err,
			)
	}

	var nextCursor *ports.ActivityCursor

	if len(activity) > pageSize {
		activity = activity[:pageSize]

		last := activity[len(activity)-1]

		nextCursor = &ports.ActivityCursor{
			OccurredAt: last.OccurredAt,
			EventID:    last.EventID,
		}
	}

	if nextCursor == nil {
		nextCursor = previousCursor
	}

	metadata, err :=
		readTenantWatermark(
			ctx,
			transaction,
			tenantID,
		)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := transaction.Commit(ctx); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"commit task activity query: %w",
				err,
			)
	}

	return activity, nextCursor, metadata, nil
}

// Search returns a page of matching tenant search documents ordered by
// rank.
func (repository *Repository) Search(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	query string,
	companyID *string,
	types []string,
	pageSize int,
	cursor *ports.SearchCursor,
) (
	[]domain.SearchResult,
	*ports.SearchCursor,
	domain.ProjectionMetadata,
	error,
) {
	transaction, err :=
		repository.beginUserTenant(
			ctx,
			tenantID,
			actorUserID,
			pgx.TxOptions{
				AccessMode: pgx.ReadOnly,
			},
		)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	hasCursor := false
	var cursorRank interface{}
	var cursorUpdatedAt interface{}
	var cursorType interface{}
	var cursorID interface{}

	if cursor != nil {
		hasCursor = true
		cursorRank = cursor.Rank
		cursorUpdatedAt = cursor.UpdatedAt
		cursorType = cursor.Type
		cursorID = cursor.ID
	}

	fetchLimit := pageSize + 1

	rows, err := transaction.Query(
		ctx,
		`
			SELECT
				document_type,
				document_id::text,
				company_id::text,
				title,
				subtitle,
				status,
				ts_rank(
					search_vector,
					websearch_to_tsquery(
						'simple',
						$5
					)
				) AS rank,
				updated_at
			FROM projection_search_documents
			WHERE NOT deleted
			  AND (
				$1::uuid IS NULL
				OR company_id = $1::uuid
			  )
			  AND (
				COALESCE(
					cardinality($2::text[]),
					0
				) = 0
				OR document_type = ANY(
					$2::text[]
				)
			  )
			  AND (
				search_vector @@ websearch_to_tsquery(
					'simple',
					$5
				)
				OR title % $5
			  )
			  AND (
				$3::boolean IS FALSE
				OR (
					ts_rank(
						search_vector,
						websearch_to_tsquery(
							'simple',
							$5
						)
					),
					updated_at,
					document_type,
					document_id
				)
				<
				(
					$6::real,
					$7::timestamptz,
					$8,
					$9::uuid
				)
			  )
			ORDER BY rank DESC,
				updated_at DESC,
				document_type ASC,
				document_id ASC
			LIMIT $4
		`,
		companyID,
		types,
		hasCursor,
		fetchLimit,
		query,
		cursorRank,
		cursorUpdatedAt,
		cursorType,
		cursorID,
	)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"query search: %w",
				err,
			)
	}

	defer rows.Close()

	previousCursor := cursor

	results := make(
		[]domain.SearchResult,
		0,
		pageSize,
	)

	for rows.Next() {
		var result domain.SearchResult

		if err := rows.Scan(
			&result.Type,
			&result.ID,
			&result.CompanyID,
			&result.Title,
			&result.Subtitle,
			&result.Status,
			&result.Rank,
			&result.UpdatedAt,
		); err != nil {
			return nil, nil,
				domain.ProjectionMetadata{},
				fmt.Errorf(
					"scan search result: %w",
					err,
				)
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"iterate search results: %w",
				err,
			)
	}

	var nextCursor *ports.SearchCursor

	if len(results) > pageSize {
		results = results[:pageSize]

		last := results[len(results)-1]

		nextCursor = &ports.SearchCursor{
			Rank:      last.Rank,
			UpdatedAt: last.UpdatedAt,
			Type:      last.Type,
			ID:        last.ID,
		}
	}

	if nextCursor == nil {
		nextCursor = previousCursor
	}

	metadata, err :=
		readTenantWatermark(
			ctx,
			transaction,
			tenantID,
		)
	if err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			err
	}

	if err := transaction.Commit(ctx); err != nil {
		return nil, nil,
			domain.ProjectionMetadata{},
			fmt.Errorf(
				"commit search query: %w",
				err,
			)
	}

	return results, nextCursor, metadata, nil
}

func readTenantWatermark(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
) (
	domain.ProjectionMetadata,
	error,
) {
	var metadata domain.ProjectionMetadata

	err := transaction.QueryRow(
		ctx,
		`
			SELECT
				projected_through_event_time,
				last_processed_at
			FROM projection_tenant_watermarks
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	).Scan(
		&metadata.ProjectedThroughEventTime,
		&metadata.LastProcessedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProjectionMetadata{}, nil
		}

		return domain.ProjectionMetadata{},
			fmt.Errorf(
				"query tenant watermark: %w",
				err,
			)
	}

	return metadata, nil
}
