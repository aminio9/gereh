package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/jackc/pgx/v5"
)

// FindDecisionByRequestID returns a previously recorded decision for the same
// evaluation request.
func (repository *Repository) FindDecisionByRequestID(
	ctx context.Context,
	servicePrincipalID string,
	tenantID string,
	requestID string,
) (domain.Decision, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		servicePrincipalID,
		tenantID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Decision{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	decision, err := queryDecisionByRequestID(
		ctx,
		transaction,
		tenantID,
		requestID,
	)
	if err != nil {
		return domain.Decision{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Decision{}, err
	}

	return decision, nil
}

// RecordDecision commits a signed decision and its outbox event atomically.
func (repository *Repository) RecordDecision(
	ctx context.Context,
	params ports.RecordDecisionParams,
) (domain.Decision, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		params.ServicePrincipalID,
		params.Decision.TenantID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Decision{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertDecision(
		ctx,
		transaction,
		params.Decision,
	); err != nil {
		return domain.Decision{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Decision.TenantID,
		params.Event,
	); err != nil {
		return domain.Decision{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Decision{}, err
	}

	return params.Decision, nil
}

// GetDecision returns one signed decision.
func (repository *Repository) GetDecision(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	decisionID string,
) (domain.Decision, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Decision{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	decision, err := scanDecision(transaction.QueryRow(
		ctx,
		decisionSelect+`
			WHERE tenant_id = $1::uuid
			  AND decision_id = $2::uuid
		`,
		tenantID,
		decisionID,
	))
	if err != nil {
		return domain.Decision{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Decision{}, err
	}

	return decision, nil
}

// ListDecisions lists decisions after the cursor.
func (repository *Repository) ListDecisions(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	subjectID *string,
	limit int,
	cursor *ports.DecisionCursor,
) ([]domain.Decision, error) {
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

	query := decisionSelect + `
		WHERE tenant_id = $1::uuid
	`

	args := []any{tenantID}

	if subjectID != nil {
		args = append(args, *subjectID)
		query += fmt.Sprintf(
			" AND subject_id = $%d::uuid",
			len(args),
		)
	}

	if cursor != nil {
		args = append(args, cursor.DecidedAt, cursor.DecisionID)
		query += fmt.Sprintf(
			" AND (decided_at, decision_id) < ($%d, $%d::uuid)",
			len(args)-1,
			len(args),
		)
	}

	args = append(args, limit)
	query += fmt.Sprintf(
		`
			ORDER BY decided_at DESC, decision_id DESC
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"list policy decisions: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var decisions []domain.Decision

	for rows.Next() {
		decision, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}

		decisions = append(decisions, decision)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate policy decisions: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return decisions, nil
}

const decisionSelect = `
	SELECT
		tenant_id::text,
		decision_id::text,
		evaluation_request_id::text,
		caller_service,
		subject_type,
		subject_id::text,
		company_id::text,
		action,
		resource_type,
		resource_id,
		resource_attributes,
		risk,
		estimated_cost_micro_usd,
		effect,
		constraints,
		reason,
		matched_policy_id::text,
		matched_policy_version,
		matched_rule_id::text,
		input_hash,
		decided_at,
		expires_at,
		signing_key_id,
		signature
	FROM policy_decisions
`

func queryDecisionByRequestID(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	requestID string,
) (domain.Decision, error) {
	return scanDecision(querier.QueryRow(
		ctx,
		decisionSelect+`
			WHERE tenant_id = $1::uuid
			  AND evaluation_request_id = $2::uuid
		`,
		tenantID,
		requestID,
	))
}

func scanDecision(
	scanner rowScanner,
) (domain.Decision, error) {
	var decision domain.Decision
	var subjectType string
	var risk string
	var effect string
	var resourceAttributes []byte
	var constraintsBytes []byte
	var matchedPolicyID any
	var matchedPolicyVersion any
	var matchedRuleID any

	err := scanner.Scan(
		&decision.TenantID,
		&decision.ID,
		&decision.RequestID,
		&decision.CallerService,
		&subjectType,
		&decision.Subject.ID,
		&decision.Subject.CompanyID,
		&decision.Action,
		&decision.Resource.Type,
		&decision.Resource.ID,
		&resourceAttributes,
		&risk,
		&decision.EstimatedCostMicroUSD,
		&effect,
		&constraintsBytes,
		&decision.Reason,
		&matchedPolicyID,
		&matchedPolicyVersion,
		&matchedRuleID,
		&decision.InputHash,
		&decision.DecidedAt,
		&decision.ExpiresAt,
		&decision.SigningKeyID,
		&decision.Signature,
	)
	if err != nil {
		return domain.Decision{}, mapDatabaseError(err)
	}

	decision.Subject.Type = domain.SubjectType(subjectType)
	decision.Risk = domain.Risk(risk)
	decision.Effect = domain.Effect(effect)

	if len(resourceAttributes) > 0 {
		if err := json.Unmarshal(
			resourceAttributes,
			&decision.Resource.Attributes,
		); err != nil {
			return domain.Decision{}, fmt.Errorf(
				"decode decision resource attributes: %w",
				err,
			)
		}
	}

	if len(constraintsBytes) > 0 {
		if err := json.Unmarshal(
			constraintsBytes,
			&decision.Constraints,
		); err != nil {
			return domain.Decision{}, fmt.Errorf(
				"decode decision constraints: %w",
				err,
			)
		}
	}

	if value, ok := matchedPolicyID.(string); ok && value != "" {
		decision.MatchedPolicyID = &value
	}

	if value, ok := matchedPolicyVersion.(int64); ok && value > 0 {
		decision.MatchedPolicyVersion = &value
	}

	if value, ok := matchedRuleID.(string); ok && value != "" {
		decision.MatchedRuleID = &value
	}

	return decision, nil
}

func insertDecision(
	ctx context.Context,
	transaction pgx.Tx,
	decision domain.Decision,
) error {
	resourceAttributes, err := json.Marshal(
		decision.Resource.Attributes,
	)
	if err != nil {
		return fmt.Errorf(
			"marshal decision resource attributes: %w",
			err,
		)
	}

	constraints, err := json.Marshal(decision.Constraints)
	if err != nil {
		return fmt.Errorf(
			"marshal decision constraints: %w",
			err,
		)
	}

	var companyID any
	if decision.Subject.CompanyID != nil {
		companyID = *decision.Subject.CompanyID
	}

	var matchedPolicyID any
	var matchedPolicyVersion any
	var matchedRuleID any

	if decision.MatchedPolicyID != nil {
		matchedPolicyID = *decision.MatchedPolicyID
	}

	if decision.MatchedPolicyVersion != nil {
		matchedPolicyVersion = *decision.MatchedPolicyVersion
	}

	if decision.MatchedRuleID != nil {
		matchedRuleID = *decision.MatchedRuleID
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO policy_decisions (
				tenant_id,
				decision_id,
				evaluation_request_id,
				caller_service,
				subject_type,
				subject_id,
				company_id,
				action,
				resource_type,
				resource_id,
				resource_attributes,
				risk,
				estimated_cost_micro_usd,
				effect,
				constraints,
				reason,
				matched_policy_id,
				matched_policy_version,
				matched_rule_id,
				input_hash,
				decided_at,
				expires_at,
				signing_key_id,
				signature
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4,
				$5,
				$6::uuid,
				$7::uuid,
				$8,
				$9,
				$10,
				$11::jsonb,
				$12,
				$13,
				$14,
				$15::jsonb,
				$16,
				$17::uuid,
				$18,
				$19::uuid,
				$20,
				$21,
				$22,
				$23,
				$24
			)
		`,
		decision.TenantID,
		decision.ID,
		decision.RequestID,
		decision.CallerService,
		string(decision.Subject.Type),
		decision.Subject.ID,
		companyID,
		decision.Action,
		decision.Resource.Type,
		decision.Resource.ID,
		resourceAttributes,
		string(decision.Risk),
		decision.EstimatedCostMicroUSD,
		string(decision.Effect),
		constraints,
		decision.Reason,
		matchedPolicyID,
		matchedPolicyVersion,
		matchedRuleID,
		decision.InputHash,
		decision.DecidedAt,
		decision.ExpiresAt,
		decision.SigningKeyID,
		decision.Signature,
	)
	if err != nil {
		return fmt.Errorf(
			"insert policy decision: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}
