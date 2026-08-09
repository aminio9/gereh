package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreatePolicy commits a new policy set and its outbox event atomically.
func (repository *Repository) CreatePolicy(
	ctx context.Context,
	params ports.CreatePolicyParams,
) (domain.Policy, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Policy.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Policy{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertPolicy(
		ctx,
		transaction,
		params.Policy,
	); err != nil {
		return domain.Policy{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Policy.TenantID,
		params.Event,
	); err != nil {
		return domain.Policy{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Policy{}, err
	}

	return params.Policy, nil
}

// GetPolicy returns one policy set with its active version when present.
func (repository *Repository) GetPolicy(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	policyID string,
) (domain.Policy, *domain.PolicyVersion, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Policy{}, nil, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	policy, err := queryPolicy(
		ctx,
		transaction,
		tenantID,
		policyID,
	)
	if err != nil {
		return domain.Policy{}, nil, err
	}

	var version *domain.PolicyVersion

	if policy.ActivePolicyVersion != nil {
		loadedVersion, err := queryPolicyVersion(
			ctx,
			transaction,
			tenantID,
			policyID,
			*policy.ActivePolicyVersion,
		)
		if err != nil {
			return domain.Policy{}, nil, err
		}

		version = &loadedVersion
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Policy{}, nil, err
	}

	return policy, version, nil
}

// ListPolicies lists policy sets after the cursor.
func (repository *Repository) ListPolicies(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	limit int,
	cursor *ports.PolicyCursor,
	includeArchived bool,
) ([]domain.Policy, error) {
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
			policy_id::text,
			scope_type,
			scope_id::text,
			name,
			description,
			status,
			active_policy_version,
			resource_version,
			created_by_user_id::text,
			created_at,
			updated_at,
			archived_at
		FROM policy_sets
		WHERE tenant_id = $1::uuid
	`

	args := []any{tenantID}

	if cursor != nil {
		args = append(args, cursor.PolicyID)
		query += fmt.Sprintf(
			" AND policy_id > $%d::uuid",
			len(args),
		)
	}

	if !includeArchived {
		query += ` AND status <> 'archived'`
	}

	args = append(args, limit)
	query += fmt.Sprintf(
		`
			ORDER BY policy_id
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"list policy sets: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var policies []domain.Policy

	for rows.Next() {
		policy, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}

		policies = append(policies, policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate policy sets: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return policies, nil
}

// CreateVersion creates a new immutable policy version.
func (repository *Repository) CreateVersion(
	ctx context.Context,
	params ports.CreateVersionParams,
) (domain.Policy, domain.PolicyVersion, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Policy.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertVersion(
		ctx,
		transaction,
		params.Version,
	); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE policy_sets
			SET
				resource_version = resource_version + 1,
				updated_at = $4
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
			  AND resource_version = $3
			  AND status <> 'archived'
		`,
		params.Policy.TenantID,
		params.Policy.ID,
		params.ExpectedResourceVersion,
		params.Policy.UpdatedAt,
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectPolicyVersionOrMissing(
			ctx,
			transaction,
			params.Policy.TenantID,
			params.Policy.ID,
			params.ExpectedResourceVersion,
		); err != nil {
			return domain.Policy{}, domain.PolicyVersion{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Policy.TenantID,
		params.Event,
	); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	return params.Policy, params.Version, nil
}

// ActivatePolicy activates an existing policy version.
func (repository *Repository) ActivatePolicy(
	ctx context.Context,
	params ports.ActivatePolicyParams,
) (domain.Policy, domain.PolicyVersion, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := activateVersionRow(
		ctx,
		transaction,
		params.TenantID,
		params.PolicyID,
		params.PolicyVersion,
	); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE policy_sets
			SET
				status = 'active',
				active_policy_version = $4,
				resource_version = resource_version + 1,
				updated_at = $5
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
			  AND resource_version = $3
			  AND status <> 'archived'
		`,
		params.TenantID,
		params.PolicyID,
		params.ExpectedResourceVersion,
		params.PolicyVersion,
		params.ActivatedAt,
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectPolicyVersionOrMissing(
			ctx,
			transaction,
			params.TenantID,
			params.PolicyID,
			params.ExpectedResourceVersion,
		); err != nil {
			return domain.Policy{}, domain.PolicyVersion{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.TenantID,
		params.Event,
	); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	policy, err := queryPolicy(
		ctx,
		transaction,
		params.TenantID,
		params.PolicyID,
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	version, err := queryPolicyVersion(
		ctx,
		transaction,
		params.TenantID,
		params.PolicyID,
		params.PolicyVersion,
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	return policy, version, nil
}

// ArchivePolicy archives a policy set.
func (repository *Repository) ArchivePolicy(
	ctx context.Context,
	params ports.ArchivePolicyParams,
) (domain.Policy, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Policy{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE policy_sets
			SET
				status = 'archived',
				resource_version = resource_version + 1,
				updated_at = $4,
				archived_at = $5
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
			  AND resource_version = $3
			  AND status <> 'archived'
		`,
		params.TenantID,
		params.PolicyID,
		params.ExpectedResourceVersion,
		params.ArchivedAt,
		params.ArchivedAt,
	)
	if err != nil {
		return domain.Policy{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectPolicyVersionOrMissing(
			ctx,
			transaction,
			params.TenantID,
			params.PolicyID,
			params.ExpectedResourceVersion,
		); err != nil {
			return domain.Policy{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.TenantID,
		params.Event,
	); err != nil {
		return domain.Policy{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Policy{}, err
	}

	return queryPolicy(
		ctx,
		transaction,
		params.TenantID,
		params.PolicyID,
	)
}

const activeBundleSelect = `
	SELECT
		policy_sets.tenant_id::text,
		policy_sets.policy_id::text,
		policy_sets.scope_type,
		policy_sets.scope_id::text,
		policy_sets.name,
		policy_sets.description,
		policy_sets.status,
		policy_sets.active_policy_version,
		policy_sets.resource_version,
		policy_sets.created_by_user_id::text,
		policy_sets.created_at,
		policy_sets.updated_at,
		policy_sets.archived_at,
		policy_versions.default_effect,
		policy_versions.notes,
		policy_versions.created_by_user_id::text,
		policy_versions.created_at,
		policy_versions.activated_at
	FROM policy_sets
	JOIN policy_versions
	  ON policy_versions.tenant_id = policy_sets.tenant_id
	 AND policy_versions.policy_id = policy_sets.policy_id
	 AND policy_versions.policy_version = policy_sets.active_policy_version
`

// ListActiveBundles returns active policy versions that can match the given
// tenant, company, and agent scope.
func (repository *Repository) ListActiveBundles(
	ctx context.Context,
	servicePrincipalID string,
	tenantID string,
	companyID *string,
	agentID *string,
) ([]domain.ActiveBundle, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		servicePrincipalID,
		tenantID,
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

	query := activeBundleSelect + `
		WHERE policy_sets.tenant_id = $1::uuid
		  AND policy_sets.status = 'active'
		  AND (
				(
					policy_sets.scope_type = 'tenant'
					AND policy_sets.scope_id IS NULL
				)
				OR (
					$2::uuid IS NOT NULL
					AND policy_sets.scope_type = 'company'
					AND policy_sets.scope_id = $2::uuid
				)
				OR (
					$3::uuid IS NOT NULL
					AND policy_sets.scope_type = 'agent'
					AND policy_sets.scope_id = $3::uuid
				)
		  )
	`

	var companyIDValue any
	if companyID != nil {
		companyIDValue = *companyID
	}

	var agentIDValue any
	if agentID != nil {
		agentIDValue = *agentID
	}

	rows, err := transaction.Query(
		ctx,
		query,
		tenantID,
		companyIDValue,
		agentIDValue,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list active policy bundles: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	bundles, err := scanActiveBundles(rows)
	if err != nil {
		return nil, err
	}

	for index := range bundles {
		rules, err := queryRules(
			ctx,
			transaction,
			bundles[index].Policy.TenantID,
			bundles[index].Policy.ID,
			bundles[index].Version.PolicyVersion,
		)
		if err != nil {
			return nil, err
		}

		bundles[index].Version.Rules = rules
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return bundles, nil
}

func insertPolicy(
	ctx context.Context,
	transaction pgx.Tx,
	policy domain.Policy,
) error {
	var scopeID any
	if policy.ScopeID != nil {
		scopeID = *policy.ScopeID
	}

	var activeVersion any
	if policy.ActivePolicyVersion != nil {
		activeVersion = *policy.ActivePolicyVersion
	}

	var archivedAt any
	if policy.ArchivedAt != nil {
		archivedAt = *policy.ArchivedAt
	}

	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO policy_sets (
				tenant_id,
				policy_id,
				scope_type,
				scope_id,
				name,
				description,
				status,
				active_policy_version,
				resource_version,
				created_by_user_id,
				created_at,
				updated_at,
				archived_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4::uuid,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10::uuid,
				$11,
				$12,
				$13
			)
		`,
		policy.TenantID,
		policy.ID,
		string(policy.ScopeType),
		scopeID,
		policy.Name,
		policy.Description,
		string(policy.Status),
		activeVersion,
		policy.ResourceVersion,
		policy.CreatedByUserID,
		policy.CreatedAt,
		policy.UpdatedAt,
		archivedAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return nil
}

const policySelect = `
	SELECT
		tenant_id::text,
		policy_id::text,
		scope_type,
		scope_id::text,
		name,
		description,
		status,
		active_policy_version,
		resource_version,
		created_by_user_id::text,
		created_at,
		updated_at,
		archived_at
	FROM policy_sets
`

func queryPolicy(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	policyID string,
) (domain.Policy, error) {
	return scanPolicy(querier.QueryRow(
		ctx,
		policySelect+`
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
		`,
		tenantID,
		policyID,
	))
}

func scanPolicy(
	scanner rowScanner,
) (domain.Policy, error) {
	var policy domain.Policy
	var scopeType string
	var status string
	var activeVersion any
	var archivedAt any

	err := scanner.Scan(
		&policy.TenantID,
		&policy.ID,
		&scopeType,
		&policy.ScopeID,
		&policy.Name,
		&policy.Description,
		&status,
		&activeVersion,
		&policy.ResourceVersion,
		&policy.CreatedByUserID,
		&policy.CreatedAt,
		&policy.UpdatedAt,
		&archivedAt,
	)
	if err != nil {
		return domain.Policy{}, mapDatabaseError(err)
	}

	policy.ScopeType = domain.ScopeType(scopeType)
	policy.Status = domain.PolicyStatus(status)

	if value, ok := activeVersion.(int64); ok && value > 0 {
		version := value
		policy.ActivePolicyVersion = &version
	}

	policy.ArchivedAt = optionalTime(archivedAt)

	return policy, nil
}

func queryPolicyVersion(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	policyID string,
	policyVersion int64,
) (domain.PolicyVersion, error) {
	version, err := scanVersion(querier.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				policy_id::text,
				policy_version,
				default_effect,
				notes,
				created_by_user_id::text,
				created_at,
				activated_at
			FROM policy_versions
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
			  AND policy_version = $3
		`,
		tenantID,
		policyID,
		policyVersion,
	))
	if err != nil {
		return domain.PolicyVersion{}, err
	}

	rules, err := queryRules(
		ctx,
		querier,
		tenantID,
		policyID,
		policyVersion,
	)
	if err != nil {
		return domain.PolicyVersion{}, err
	}

	version.Rules = rules

	return version, nil
}

func scanVersion(
	scanner rowScanner,
) (domain.PolicyVersion, error) {
	var version domain.PolicyVersion
	var defaultEffect string
	var activatedAt any

	err := scanner.Scan(
		&version.TenantID,
		&version.PolicyID,
		&version.PolicyVersion,
		&defaultEffect,
		&version.Notes,
		&version.CreatedByUserID,
		&version.CreatedAt,
		&activatedAt,
	)
	if err != nil {
		return domain.PolicyVersion{}, mapDatabaseError(err)
	}

	version.DefaultEffect = domain.Effect(defaultEffect)
	version.ActivatedAt = optionalTime(activatedAt)

	return version, nil
}

func queryRules(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	policyID string,
	policyVersion int64,
) ([]domain.Rule, error) {
	rows, err := querier.Query(
		ctx,
		`
			SELECT
				rule_id::text,
				priority,
				name,
				enabled,
				effect,
				action_patterns,
				resource_types,
				risk_levels,
				maximum_estimated_cost_micro_usd,
				condition,
				constraints,
				reason
			FROM policy_rules
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
			  AND policy_version = $3
			ORDER BY priority
		`,
		tenantID,
		policyID,
		policyVersion,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list policy rules: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var rules []domain.Rule

	for rows.Next() {
		var rule domain.Rule
		var effect string
		var maximumCost any
		var constraintsBytes []byte
		var riskLevels []string

		err := rows.Scan(
			&rule.ID,
			&rule.Priority,
			&rule.Name,
			&rule.Enabled,
			&effect,
			&rule.ActionPatterns,
			&rule.ResourceTypes,
			&riskLevels,
			&maximumCost,
			&rule.Condition,
			&constraintsBytes,
			&rule.Reason,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"scan policy rule: %w",
				mapDatabaseError(err),
			)
		}

		rule.Effect = domain.Effect(effect)

		for _, risk := range riskLevels {
			rule.RiskLevels = append(
				rule.RiskLevels,
				domain.Risk(risk),
			)
		}

		if maximumCost != nil {
			cost := maximumCost.(int64)
			rule.MaximumEstimatedCostMicroUSD = &cost
		}

		if len(constraintsBytes) > 0 {
			if err := json.Unmarshal(
				constraintsBytes,
				&rule.Constraints,
			); err != nil {
				return nil, fmt.Errorf(
					"decode rule constraints: %w",
					err,
				)
			}
		}

		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate policy rules: %w",
			err,
		)
	}

	return rules, nil
}

func insertVersion(
	ctx context.Context,
	transaction pgx.Tx,
	version domain.PolicyVersion,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO policy_versions (
				tenant_id,
				policy_id,
				policy_version,
				default_effect,
				notes,
				created_by_user_id,
				created_at,
				activated_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4,
				$5,
				$6::uuid,
				$7,
				$8
			)
		`,
		version.TenantID,
		version.PolicyID,
		version.PolicyVersion,
		string(version.DefaultEffect),
		version.Notes,
		version.CreatedByUserID,
		version.CreatedAt,
		version.ActivatedAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	for _, rule := range version.Rules {
		if err := insertRule(
			ctx,
			transaction,
			version.TenantID,
			version.PolicyID,
			version.PolicyVersion,
			rule,
		); err != nil {
			return err
		}
	}

	return nil
}

func insertRule(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	policyID string,
	policyVersion int64,
	rule domain.Rule,
) error {
	riskLevels := make([]string, 0, len(rule.RiskLevels))
	for _, risk := range rule.RiskLevels {
		riskLevels = append(riskLevels, string(risk))
	}

	var maximumCost any
	if rule.MaximumEstimatedCostMicroUSD != nil {
		maximumCost = *rule.MaximumEstimatedCostMicroUSD
	}

	constraints, err := json.Marshal(rule.Constraints)
	if err != nil {
		return fmt.Errorf(
			"marshal rule constraints: %w",
			err,
		)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO policy_rules (
				tenant_id,
				policy_id,
				policy_version,
				rule_id,
				priority,
				name,
				enabled,
				effect,
				action_patterns,
				resource_types,
				risk_levels,
				maximum_estimated_cost_micro_usd,
				condition,
				constraints,
				reason
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4::uuid,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11,
				$12,
				$13,
				$14::jsonb,
				$15
			)
		`,
		tenantID,
		policyID,
		policyVersion,
		rule.ID,
		rule.Priority,
		rule.Name,
		rule.Enabled,
		string(rule.Effect),
		rule.ActionPatterns,
		rule.ResourceTypes,
		riskLevels,
		maximumCost,
		rule.Condition,
		constraints,
		rule.Reason,
	)
	if err != nil {
		return fmt.Errorf(
			"insert policy rule: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}

func activateVersionRow(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	policyID string,
	policyVersion int64,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			UPDATE policy_versions
			SET activated_at = COALESCE(
				activated_at,
				clock_timestamp()
			)
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
			  AND policy_version = $3
		`,
		tenantID,
		policyID,
		policyVersion,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return nil
}

func rejectPolicyVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	policyID string,
	expectedVersion int64,
) error {
	var currentVersion int64
	var status string

	err := transaction.QueryRow(
		ctx,
		`
			SELECT resource_version, status
			FROM policy_sets
			WHERE tenant_id = $1::uuid
			  AND policy_id = $2::uuid
		`,
		tenantID,
		policyID,
	).Scan(&currentVersion, &status)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query policy set version: %w",
			err,
		)
	}

	if status == "archived" {
		return fmt.Errorf(
			"%w: policy set is archived",
			domain.ErrConflict,
		)
	}

	return fmt.Errorf(
		"%w: expected %d, found %d",
		domain.ErrVersionConflict,
		expectedVersion,
		currentVersion,
	)
}

func scanActiveBundles(
	rows pgx.Rows,
) ([]domain.ActiveBundle, error) {
	var bundles []domain.ActiveBundle

	for rows.Next() {
		var bundle domain.ActiveBundle
		var scopeType string
		var status string
		var defaultEffect string
		var activePolicyVersion any
		var archivedAt any
		var activatedAt any

		err := rows.Scan(
			&bundle.Policy.TenantID,
			&bundle.Policy.ID,
			&scopeType,
			&bundle.Policy.ScopeID,
			&bundle.Policy.Name,
			&bundle.Policy.Description,
			&status,
			&activePolicyVersion,
			&bundle.Policy.ResourceVersion,
			&bundle.Policy.CreatedByUserID,
			&bundle.Policy.CreatedAt,
			&bundle.Policy.UpdatedAt,
			&archivedAt,
			&defaultEffect,
			&bundle.Version.Notes,
			&bundle.Version.CreatedByUserID,
			&bundle.Version.CreatedAt,
			&activatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"scan active policy bundle: %w",
				mapDatabaseError(err),
			)
		}

		bundle.Policy.ScopeType = domain.ScopeType(scopeType)
		bundle.Policy.Status = domain.PolicyStatus(status)

		if value, ok := activePolicyVersion.(int64); ok && value > 0 {
			version := value
			bundle.Policy.ActivePolicyVersion = &version
			bundle.Version.PolicyVersion = version
		}

		bundle.Policy.ArchivedAt = optionalTime(archivedAt)

		bundle.Version.TenantID = bundle.Policy.TenantID
		bundle.Version.PolicyID = bundle.Policy.ID
		bundle.Version.DefaultEffect =
			domain.Effect(defaultEffect)
		bundle.Version.ActivatedAt = optionalTime(activatedAt)

		bundles = append(bundles, bundle)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate active policy bundles: %w",
			err,
		)
	}

	return bundles, nil
}

func optionalTime(value any) *time.Time {
	valueTime, ok := value.(*time.Time)
	if !ok || valueTime == nil {
		return nil
	}

	copied := *valueTime
	return &copied
}
