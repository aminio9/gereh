package postgres

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/jackc/pgx/v5"
)

// EnsureDefaults idempotently creates the default tenant policy set.
func (repository *Repository) EnsureDefaults(
	ctx context.Context,
	params ports.EnsureDefaultsParams,
) ([]domain.Policy, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		params.ServicePrincipalID,
		params.TenantID,
		pgx.TxOptions{},
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var exists bool

	err = transaction.QueryRow(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM policy_bootstrap_requests
				WHERE tenant_id = $1::uuid
			)
		`,
		params.TenantID,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf(
			"check policy bootstrap request: %w",
			mapDatabaseError(err),
		)
	}

	if exists {
		if err := commit(ctx, transaction); err != nil {
			return nil, err
		}

		return loadBootstrapPolicies(
			ctx,
			repository,
			params.ServicePrincipalID,
			params.TenantID,
		)
	}

	createdPolicyIDs := make([]string, 0, len(params.Policies))

	for index, policy := range params.Policies {
		if err := insertPolicy(
			ctx,
			transaction,
			policy,
		); err != nil {
			return nil, err
		}

		if err := insertVersion(
			ctx,
			transaction,
			params.Versions[index],
		); err != nil {
			return nil, err
		}

		createdPolicyIDs = append(
			createdPolicyIDs,
			policy.ID,
		)
	}

	var requestID string

	err = transaction.QueryRow(
		ctx,
		`
			INSERT INTO policy_bootstrap_requests (
				onboarding_operation_id,
				tenant_id,
				created_policy_ids,
				actor_user_id,
				created_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid[],
				$4::uuid,
				$5
			)
			RETURNING onboarding_operation_id::text
		`,
		params.OnboardingOperationID,
		params.TenantID,
		createdPolicyIDs,
		params.ActorUserID,
		params.CreatedAt,
	).Scan(&requestID)
	if err != nil {
		return nil, fmt.Errorf(
			"insert policy bootstrap request: %w",
			mapDatabaseError(err),
		)
	}

	_ = requestID

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return loadBootstrapPolicies(
		ctx,
		repository,
		params.ServicePrincipalID,
		params.TenantID,
	)
}

func loadBootstrapPolicies(
	ctx context.Context,
	repository *Repository,
	servicePrincipalID string,
	tenantID string,
) ([]domain.Policy, error) {
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

	rows, err := transaction.Query(
		ctx,
		`
			SELECT
				policy_id::text
			FROM policy_bootstrap_requests
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list policy bootstrap request: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var policyIDs []string

	for rows.Next() {
		var policyID string
		if err := rows.Scan(&policyID); err != nil {
			return nil, fmt.Errorf(
				"scan policy bootstrap request: %w",
				err,
			)
		}

		policyIDs = append(policyIDs, policyID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate policy bootstrap request: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	var policies []domain.Policy

	for _, policyID := range policyIDs {
		policy, err := repository.loadBootstrapPolicy(
			ctx,
			servicePrincipalID,
			tenantID,
			policyID,
		)
		if err != nil {
			return nil, err
		}

		policies = append(policies, policy)
	}

	return policies, nil
}

func (repository *Repository) loadBootstrapPolicy(
	ctx context.Context,
	servicePrincipalID string,
	tenantID string,
	policyID string,
) (domain.Policy, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		servicePrincipalID,
		tenantID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Policy{}, err
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
		return domain.Policy{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Policy{}, err
	}

	return policy, nil
}
