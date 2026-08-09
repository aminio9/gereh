package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/aminio9/gereh/services/organization-agent/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreateAgent commits a new agent, its first revision, and the outbox event
// atomically.
func (repository *Repository) CreateAgent(
	ctx context.Context,
	params ports.CreateAgentParams,
) (domain.Agent, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Agent.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Agent{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := lockCompany(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Agent.CompanyID,
	); err != nil {
		return domain.Agent{}, err
	}

	if params.Agent.ManagerAgentID != nil {
		manager, queryErr := queryAgent(
			ctx,
			transaction,
			params.Agent.TenantID,
			*params.Agent.ManagerAgentID,
		)
		if queryErr != nil {
			return domain.Agent{}, queryErr
		}

		if manager.CompanyID != params.Agent.CompanyID {
			return domain.Agent{}, domain.ErrInvalidArgument
		}
	}

	configuration, err := json.Marshal(
		params.Agent.Configuration,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO organization_agents (
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
				capabilities,
				configuration,
				version,
				created_by_user_id,
				created_at,
				updated_at
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
				$13::jsonb,
				$14,
				$15::uuid,
				$16,
				$17
			)
		`,
		params.Agent.TenantID,
		params.Agent.CompanyID,
		params.Agent.ID,
		params.Agent.Slug,
		params.Agent.DisplayName,
		params.Agent.RoleTitle,
		params.Agent.Objective,
		params.Agent.ManagerAgentID,
		string(params.Agent.Status),
		string(params.Agent.ExecutionProfile),
		string(params.Agent.AutonomyLevel),
		params.Agent.Capabilities,
		configuration,
		params.Agent.Version,
		params.Agent.CreatedByUserID,
		params.Agent.CreatedAt,
		params.Agent.UpdatedAt,
	)
	if err != nil {
		return domain.Agent{}, mapDatabaseError(err)
	}

	if err := insertAgentRevision(
		ctx,
		transaction,
		params.Agent,
		"created",
		params.ActorUserID,
		params.Agent.CreatedAt,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Event,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Agent{}, err
	}

	return params.Agent, nil
}

// GetAgent returns one agent by identity.
func (repository *Repository) GetAgent(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	agentID string,
) (domain.Agent, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Agent{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	agent, err := queryAgent(
		ctx,
		transaction,
		tenantID,
		agentID,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Agent{}, err
	}

	return agent, nil
}

// ListAgents lists one company's agents after the cursor.
func (repository *Repository) ListAgents(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	limit int,
	cursor *ports.AgentCursor,
	includeDeleted bool,
) ([]domain.Agent, error) {
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
			agent_id::text,
			slug,
			display_name,
			role_title,
			objective,
			manager_agent_id::text,
			status,
			execution_profile,
			autonomy_level,
			capabilities,
			configuration,
			version,
			created_by_user_id::text,
			created_at,
			updated_at,
			deleted_at
		FROM organization_agents
		WHERE tenant_id = $1::uuid
		  AND company_id = $2::uuid
	`

	args := []any{tenantID, companyID}

	if cursor != nil {
		args = append(args, cursor.AgentID)
		query += fmt.Sprintf(
			" AND agent_id > $%d::uuid",
			len(args),
		)
	}

	if !includeDeleted {
		query += ` AND status <> 'deleted'`
	}

	args = append(args, limit)
	query += fmt.Sprintf(
		`
			ORDER BY agent_id
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"list organization agents: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var agents []domain.Agent

	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}

		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate organization agents: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return agents, nil
}

// UpdateAgent applies an optimistic-concurrency agent update.
func (repository *Repository) UpdateAgent(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	configuration, err := json.Marshal(
		params.Agent.Configuration,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	return repository.updateAgent(
		ctx,
		params,
		`
			UPDATE organization_agents
			SET
				display_name = $4,
				role_title = $5,
				objective = $6,
				execution_profile = $7,
				autonomy_level = $8,
				capabilities = $9,
				configuration = $10::jsonb,
				version = version + 1,
				updated_at = $11
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
			  AND version = $3
		`,
		[]any{
			params.Agent.DisplayName,
			params.Agent.RoleTitle,
			params.Agent.Objective,
			string(params.Agent.ExecutionProfile),
			string(params.Agent.AutonomyLevel),
			params.Agent.Capabilities,
			configuration,
			params.Agent.UpdatedAt,
		},
	)
}

// SetAgentManager reassigns a manager after cycle detection.
func (repository *Repository) SetAgentManager(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Agent.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Agent{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := lockCompany(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Agent.CompanyID,
	); err != nil {
		return domain.Agent{}, err
	}

	if params.Agent.ManagerAgentID != nil {
		if *params.Agent.ManagerAgentID == params.Agent.ID {
			return domain.Agent{}, domain.ErrHierarchyCycle
		}

		manager, queryErr := queryAgent(
			ctx,
			transaction,
			params.Agent.TenantID,
			*params.Agent.ManagerAgentID,
		)
		if queryErr != nil {
			return domain.Agent{}, queryErr
		}

		if manager.CompanyID != params.Agent.CompanyID {
			return domain.Agent{}, domain.ErrInvalidArgument
		}

		cycle, cycleErr := hierarchyWouldCycle(
			ctx,
			transaction,
			params.Agent.TenantID,
			params.Agent.CompanyID,
			params.Agent.ID,
			*params.Agent.ManagerAgentID,
		)
		if cycleErr != nil {
			return domain.Agent{}, cycleErr
		}

		if cycle {
			return domain.Agent{}, domain.ErrHierarchyCycle
		}
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE organization_agents
			SET
				manager_agent_id = $4,
				version = version + 1,
				updated_at = $5
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
			  AND version = $3
		`,
		params.Agent.TenantID,
		params.Agent.ID,
		params.ExpectedVersion,
		params.Agent.ManagerAgentID,
		params.Agent.UpdatedAt,
	)
	if err != nil {
		return domain.Agent{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectAgentVersionOrMissing(
			ctx,
			transaction,
			params.Agent.TenantID,
			params.Agent.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := insertAgentRevision(
		ctx,
		transaction,
		params.Agent,
		params.ChangeKind,
		params.ActorUserID,
		params.Agent.UpdatedAt,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Event,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Agent{}, err
	}

	return params.Agent, nil
}

// ChangeAgentStatus applies a lifecycle status transition.
func (repository *Repository) ChangeAgentStatus(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	return repository.updateAgent(
		ctx,
		params,
		`
			UPDATE organization_agents
			SET
				status = $4,
				version = version + 1,
				updated_at = $5
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
			  AND version = $3
		`,
		[]any{
			string(params.Agent.Status),
			params.Agent.UpdatedAt,
		},
	)
}

// DeleteAgent soft-deletes an agent with no direct reports.
func (repository *Repository) DeleteAgent(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Agent.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Agent{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := lockCompany(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Agent.CompanyID,
	); err != nil {
		return domain.Agent{}, err
	}

	var hasReports bool

	err = transaction.QueryRow(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM organization_agents
				WHERE tenant_id = $1::uuid
				  AND company_id = $2::uuid
				  AND manager_agent_id = $3::uuid
				  AND status <> 'deleted'
			)
		`,
		params.Agent.TenantID,
		params.Agent.CompanyID,
		params.Agent.ID,
	).Scan(&hasReports)
	if err != nil {
		return domain.Agent{}, fmt.Errorf(
			"check agent direct reports: %w",
			err,
		)
	}

	if hasReports {
		return domain.Agent{}, domain.ErrAgentHasReports
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE organization_agents
			SET
				status = 'deleted',
				deleted_at = $4,
				version = version + 1,
				updated_at = $5
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
			  AND version = $3
		`,
		params.Agent.TenantID,
		params.Agent.ID,
		params.ExpectedVersion,
		params.Agent.DeletedAt,
		params.Agent.UpdatedAt,
	)
	if err != nil {
		return domain.Agent{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectAgentVersionOrMissing(
			ctx,
			transaction,
			params.Agent.TenantID,
			params.Agent.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := insertAgentRevision(
		ctx,
		transaction,
		params.Agent,
		params.ChangeKind,
		params.ActorUserID,
		params.Agent.UpdatedAt,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Event,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Agent{}, err
	}

	return params.Agent, nil
}

// GetHierarchy returns the reporting tree of a company with depths.
func (repository *Repository) GetHierarchy(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) ([]domain.HierarchyNode, error) {
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

	rows, err := transaction.Query(
		ctx,
		`
			WITH RECURSIVE hierarchy AS (
				SELECT
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
					capabilities,
					configuration,
					version,
					created_by_user_id,
					created_at,
					updated_at,
					deleted_at,
					0 AS depth
				FROM organization_agents
				WHERE tenant_id = $1::uuid
				  AND company_id = $2::uuid
				  AND status <> 'deleted'
				  AND manager_agent_id IS NULL

				UNION ALL

				SELECT
					child.tenant_id,
					child.company_id,
					child.agent_id,
					child.slug,
					child.display_name,
					child.role_title,
					child.objective,
					child.manager_agent_id,
					child.status,
					child.execution_profile,
					child.autonomy_level,
					child.capabilities,
					child.configuration,
					child.version,
					child.created_by_user_id,
					child.created_at,
					child.updated_at,
					child.deleted_at,
					hierarchy.depth + 1
				FROM organization_agents AS child
				JOIN hierarchy
				  ON child.manager_agent_id =
					hierarchy.agent_id
				 AND child.tenant_id = $1::uuid
				 AND child.company_id = $2::uuid
				WHERE child.status <> 'deleted'
			)
			SELECT
				tenant_id::text,
				company_id::text,
				agent_id::text,
				slug,
				display_name,
				role_title,
				objective,
				manager_agent_id::text,
				status,
				execution_profile,
				autonomy_level,
				capabilities,
				configuration,
				version,
				created_by_user_id::text,
				created_at,
				updated_at,
				deleted_at,
				depth
			FROM hierarchy
		`,
		tenantID,
		companyID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"query organization hierarchy: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var nodes []domain.HierarchyNode

	for rows.Next() {
		var node domain.HierarchyNode

		node.Agent, err = scanAgent(rows)
		if err != nil {
			return nil, err
		}

		if err := rows.Scan(&node.Depth); err != nil {
			return nil, fmt.Errorf(
				"scan hierarchy depth: %w",
				err,
			)
		}

		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate organization hierarchy: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return nodes, nil
}

func (repository *Repository) updateAgent(
	ctx context.Context,
	params ports.UpdateAgentParams,
	updateSQL string,
	updateArgs []any,
) (domain.Agent, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Agent.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Agent{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := lockCompany(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Agent.CompanyID,
	); err != nil {
		return domain.Agent{}, err
	}

	args := []any{
		params.Agent.TenantID,
		params.Agent.ID,
		params.ExpectedVersion,
	}
	args = append(args, updateArgs...)

	result, err := transaction.Exec(ctx, updateSQL, args...)
	if err != nil {
		return domain.Agent{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectAgentVersionOrMissing(
			ctx,
			transaction,
			params.Agent.TenantID,
			params.Agent.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := insertAgentRevision(
		ctx,
		transaction,
		params.Agent,
		params.ChangeKind,
		params.ActorUserID,
		params.Agent.UpdatedAt,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Agent.TenantID,
		params.Event,
	); err != nil {
		return domain.Agent{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Agent{}, err
	}

	return params.Agent, nil
}

func rejectAgentVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	agentID string,
	expectedVersion int64,
) error {
	var currentVersion int64

	err := transaction.QueryRow(
		ctx,
		`
			SELECT version
			FROM organization_agents
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
		`,
		tenantID,
		agentID,
	).Scan(&currentVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query agent version: %w",
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

func insertAgentRevision(
	ctx context.Context,
	transaction pgx.Tx,
	agent domain.Agent,
	changeKind string,
	actorUserID string,
	occurredAt time.Time,
) error {
	snapshot, err := json.Marshal(agent)
	if err != nil {
		return fmt.Errorf(
			"marshal agent revision: %w",
			err,
		)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO organization_agent_revisions (
				tenant_id,
				agent_id,
				version,
				change_kind,
				snapshot,
				actor_user_id,
				occurred_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4,
				$5::jsonb,
				$6::uuid,
				$7
			)
		`,
		agent.TenantID,
		agent.ID,
		agent.Version,
		changeKind,
		snapshot,
		actorUserID,
		occurredAt,
	)
	if err != nil {
		return fmt.Errorf(
			"insert agent revision: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}

func queryAgent(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	agentID string,
) (domain.Agent, error) {
	return scanAgent(querier.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				company_id::text,
				agent_id::text,
				slug,
				display_name,
				role_title,
				objective,
				manager_agent_id::text,
				status,
				execution_profile,
				autonomy_level,
				capabilities,
				configuration,
				version,
				created_by_user_id::text,
				created_at,
				updated_at,
				deleted_at
			FROM organization_agents
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
		`,
		tenantID,
		agentID,
	))
}

func scanAgent(
	scanner rowScanner,
) (domain.Agent, error) {
	var agent domain.Agent
	var status string
	var profile string
	var autonomy string
	var configurationJSON []byte

	err := scanner.Scan(
		&agent.TenantID,
		&agent.CompanyID,
		&agent.ID,
		&agent.Slug,
		&agent.DisplayName,
		&agent.RoleTitle,
		&agent.Objective,
		&agent.ManagerAgentID,
		&status,
		&profile,
		&autonomy,
		&agent.Capabilities,
		&configurationJSON,
		&agent.Version,
		&agent.CreatedByUserID,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.DeletedAt,
	)
	if err != nil {
		return domain.Agent{}, mapDatabaseError(err)
	}

	agent.Status = domain.AgentStatus(status)
	agent.ExecutionProfile = domain.ExecutionProfile(profile)
	agent.AutonomyLevel = domain.AutonomyLevel(autonomy)

	if len(configurationJSON) > 0 {
		if err := json.Unmarshal(
			configurationJSON,
			&agent.Configuration,
		); err != nil {
			return domain.Agent{}, fmt.Errorf(
				"decode agent configuration: %w",
				err,
			)
		}
	}

	return agent, nil
}

// GetAgentAsService retrieves an agent under the service principal scope.
//
// It is used by the workload-only policy-context API so the Policy Service can
// resolve trusted agent context without impersonating a user.
func (repository *Repository) GetAgentAsService(
	ctx context.Context,
	tenantID string,
	servicePrincipalID string,
	agentID string,
) (domain.Agent, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		tenantID,
		servicePrincipalID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Agent{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	agent, err := queryAgent(
		ctx,
		transaction,
		tenantID,
		agentID,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Agent{}, err
	}

	return agent, nil
}
