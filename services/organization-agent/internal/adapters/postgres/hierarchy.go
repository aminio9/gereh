package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// lockCompany takes a FOR UPDATE lock on an active company row. This
// serializes hierarchy changes per company so concurrent mutations cannot
// both pass cycle detection and jointly create a cycle.
func lockCompany(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	companyID string,
) error {
	var lockedID string

	err := transaction.QueryRow(
		ctx,
		`
			SELECT company_id::text
			FROM organization_companies
			WHERE tenant_id = $1::uuid
			  AND company_id = $2::uuid
			  AND status = 'active'
			FOR UPDATE
		`,
		tenantID,
		companyID,
	).Scan(&lockedID)

	return mapDatabaseError(err)
}

// hierarchyWouldCycle reports whether assigning managerAgentID to agentID
// would create a cycle. It walks upward from the proposed manager and
// rejects when the agent appears anywhere in that chain.
func hierarchyWouldCycle(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	companyID string,
	agentID string,
	managerAgentID string,
) (bool, error) {
	var cycle bool

	err := transaction.QueryRow(
		ctx,
		`
			WITH RECURSIVE manager_chain AS (
				SELECT
					agent_id,
					manager_agent_id,
					ARRAY[agent_id] AS visited
				FROM organization_agents
				WHERE tenant_id = $1::uuid
				  AND company_id = $2::uuid
				  AND agent_id = $3::uuid
				  AND status <> 'deleted'

				UNION ALL

				SELECT
					parent.agent_id,
					parent.manager_agent_id,
					chain.visited || parent.agent_id
				FROM organization_agents AS parent
				JOIN manager_chain AS chain
				  ON parent.agent_id = chain.manager_agent_id
				 AND parent.tenant_id = $1::uuid
				 AND parent.company_id = $2::uuid
				WHERE parent.status <> 'deleted'
				  AND NOT (
					parent.agent_id = ANY(chain.visited)
				  )
			)
			SELECT EXISTS (
				SELECT 1
				FROM manager_chain
				WHERE agent_id = $4::uuid
			)
		`,
		tenantID,
		companyID,
		managerAgentID,
		agentID,
	).Scan(&cycle)
	if err != nil {
		return false, fmt.Errorf(
			"check agent hierarchy cycle: %w",
			err,
		)
	}

	return cycle, nil
}
