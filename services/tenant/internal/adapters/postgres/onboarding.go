package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/aminio9/gereh/services/tenant/internal/ports"
	"github.com/jackc/pgx/v5"
)

func insertOperation(
	ctx context.Context,
	transaction pgx.Tx,
	operation domain.Operation,
) error {
	metadata, err := json.Marshal(operation.Metadata)
	if err != nil {
		return fmt.Errorf(
			"marshal operation metadata: %w",
			err,
		)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO tenant_onboarding_operations (
				operation_id,
				tenant_id,
				actor_user_id,
				request_id,
				state,
				resource_name,
				metadata,
				version,
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
				$7::jsonb,
				$8,
				$9,
				$10
			)
		`,
		operation.ID,
		operation.TenantID,
		operation.ActorUserID,
		operation.RequestID,
		string(operation.State),
		operation.ResourceName,
		metadata,
		operation.Version,
		operation.CreatedAt,
		operation.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"insert onboarding operation: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}

func scanOperation(
	row rowScanner,
) (domain.Operation, error) {
	var operation domain.Operation

	var state string
	var errorCode *string
	var errorMessage *string
	var errorRetryable *bool
	var errorDetailsJSON []byte
	var metadataJSON []byte

	err := row.Scan(
		&operation.ID,
		&operation.TenantID,
		&operation.ActorUserID,
		&operation.RequestID,
		&state,
		&operation.ResourceName,
		&operation.WorkflowID,
		&operation.WorkflowRunID,
		&errorCode,
		&errorMessage,
		&errorRetryable,
		&errorDetailsJSON,
		&metadataJSON,
		&operation.Version,
		&operation.CreatedAt,
		&operation.UpdatedAt,
		&operation.StartedAt,
		&operation.CompletedAt,
	)
	if err != nil {
		return domain.Operation{},
			mapDatabaseError(err)
	}

	operation.State = domain.OperationState(state)

	operation.Metadata = make(map[string]string)
	if err := json.Unmarshal(
		metadataJSON,
		&operation.Metadata,
	); err != nil {
		return domain.Operation{}, fmt.Errorf(
			"decode operation metadata: %w",
			err,
		)
	}

	if errorCode != nil {
		details := make(map[string]any)

		if err := json.Unmarshal(
			errorDetailsJSON,
			&details,
		); err != nil {
			return domain.Operation{}, fmt.Errorf(
				"decode operation error details: %w",
				err,
			)
		}

		operation.Error = &domain.OperationError{
			Code:      *errorCode,
			Message:   dereferenceString(errorMessage),
			Retryable: dereferenceBool(errorRetryable),
			Details:   details,
		}
	}

	return operation, nil
}

func queryOperationByID(
	ctx context.Context,
	querier rowQuerier,
	operationID string,
) (domain.Operation, error) {
	return scanOperation(
		querier.QueryRow(
			ctx,
			operationSelectSQL+
				` WHERE operation_id = $1::uuid`,
			operationID,
		),
	)
}

func queryOperationByTenant(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
) (domain.Operation, error) {
	return scanOperation(
		querier.QueryRow(
			ctx,
			operationSelectSQL+
				` WHERE tenant_id = $1::uuid`,
			tenantID,
		),
	)
}

const operationSelectSQL = `
	SELECT
		operation_id::text,
		tenant_id::text,
		actor_user_id::text,
		request_id,
		state,
		resource_name,
		COALESCE(workflow_id, ''),
		COALESCE(workflow_run_id, ''),
		error_code,
		error_message,
		error_retryable,
		error_details,
		metadata,
		version,
		created_at,
		updated_at,
		started_at,
		completed_at
	FROM tenant_onboarding_operations
`

// GetOperation returns a single onboarding operation for the requesting
// actor. It is scoped by the actor's principal via RLS.
func (repository *Repository) GetOperation(
	ctx context.Context,
	actorUserID string,
	operationID string,
) (domain.Operation, error) {
	transaction, err := repository.beginPrincipal(
		ctx,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Operation{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	operation, err := queryOperationByID(
		ctx,
		transaction,
		operationID,
	)
	if err != nil {
		return domain.Operation{}, err
	}

	// Keep the explicit predicate even though RLS independently enforces it.
	if operation.ActorUserID != actorUserID {
		return domain.Operation{}, domain.ErrNotFound
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Operation{}, err
	}

	return operation, nil
}

// MarkOnboardingRunning records that the provisioning workflow started.
// It is idempotent for operations already running or succeeded.
func (repository *Repository) MarkOnboardingRunning(
	ctx context.Context,
	params ports.MarkOnboardingRunningParams,
) (domain.Operation, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		params.TenantID,
		params.ServicePrincipalID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Operation{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	commandTag, err := transaction.Exec(
		ctx,
		`
			UPDATE tenant_onboarding_operations
			SET
				state = 'running',
				workflow_id = $3,
				workflow_run_id = $4,
				version = version + 1,
				updated_at = $5,
				started_at = COALESCE(started_at, $5)
			WHERE tenant_id = $1::uuid
			  AND operation_id = $2::uuid
			  AND state = 'pending'
		`,
		params.TenantID,
		params.OperationID,
		params.WorkflowID,
		params.WorkflowRunID,
		params.StartedAt,
	)
	if err != nil {
		return domain.Operation{}, fmt.Errorf(
			"mark onboarding running: %w",
			err,
		)
	}

	operation, err := queryOperationByID(
		ctx,
		transaction,
		params.OperationID,
	)
	if err != nil {
		return domain.Operation{}, err
	}

	if commandTag.RowsAffected() == 0 {
		switch operation.State {
		case domain.OperationStateRunning,
			domain.OperationStateSucceeded:
			if err := commit(ctx, transaction); err != nil {
				return domain.Operation{}, err
			}

			return operation, nil

		case domain.OperationStateFailed,
			domain.OperationStateCanceled:
			return domain.Operation{},
				domain.ErrOperationAlreadyCompleted

		default:
			return domain.Operation{},
				domain.ErrInvalidOperationTransition
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.TenantID,
		params.Event,
	); err != nil {
		return domain.Operation{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Operation{}, err
	}

	return operation, nil
}

// CompleteOnboarding activates the tenant and succeeds the operation. It is
// idempotent: a second call for an already succeeded operation returns the
// current context and operation.
func (repository *Repository) CompleteOnboarding(
	ctx context.Context,
	params ports.CompleteOnboardingParams,
) (domain.CreateTenantResult, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		params.TenantID,
		params.ServicePrincipalID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	operation, err := queryOperationByID(
		ctx,
		transaction,
		params.OperationID,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	if operation.State == domain.OperationStateSucceeded {
		contextValue, queryErr := queryContext(
			ctx,
			transaction,
			params.TenantID,
			operation.ActorUserID,
			false,
		)
		if queryErr != nil {
			return domain.CreateTenantResult{}, queryErr
		}

		if err := commit(ctx, transaction); err != nil {
			return domain.CreateTenantResult{}, err
		}

		return domain.CreateTenantResult{
			Context:   contextValue,
			Operation: operation,
		}, nil
	}

	if operation.State != domain.OperationStateRunning {
		return domain.CreateTenantResult{},
			domain.ErrInvalidOperationTransition
	}

	tenantTag, err := transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET
				status = 'active',
				version = version + 1,
				updated_at = $2
			WHERE tenant_id = $1::uuid
			  AND status = 'provisioning'
		`,
		params.TenantID,
		params.CompletedAt,
	)
	if err != nil {
		return domain.CreateTenantResult{},
			fmt.Errorf(
				"activate tenant: %w",
				err,
			)
	}

	if tenantTag.RowsAffected() != 1 {
		return domain.CreateTenantResult{},
			domain.ErrInvalidOperationTransition
	}

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE tenant_onboarding_operations
			SET
				state = 'succeeded',
				version = version + 1,
				updated_at = $3,
				completed_at = $3
			WHERE tenant_id = $1::uuid
			  AND operation_id = $2::uuid
			  AND state = 'running'
		`,
		params.TenantID,
		params.OperationID,
		params.CompletedAt,
	)
	if err != nil {
		return domain.CreateTenantResult{},
			fmt.Errorf(
				"complete onboarding operation: %w",
				err,
			)
	}

	if params.Event == nil {
		return domain.CreateTenantResult{},
			domain.ErrInvalidArgument
	}

	operation, err = queryOperationByID(
		ctx,
		transaction,
		params.OperationID,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	contextValue, err := queryContext(
		ctx,
		transaction,
		params.TenantID,
		operation.ActorUserID,
		false,
	)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	event, err := params.Event(contextValue)
	if err != nil {
		return domain.CreateTenantResult{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.TenantID,
		event,
	); err != nil {
		return domain.CreateTenantResult{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.CreateTenantResult{}, err
	}

	return domain.CreateTenantResult{
		Context:   contextValue,
		Operation: operation,
	}, nil
}

// FailOnboarding records a terminal provisioning failure. The tenant is
// moved to provisioning_failed and the operation to failed. It is idempotent:
// a second call for an already failed operation returns the current state.
func (repository *Repository) FailOnboarding(
	ctx context.Context,
	params ports.FailOnboardingParams,
) (
	domain.Operation,
	domain.Tenant,
	error,
) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		params.TenantID,
		params.ServicePrincipalID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	operation, err := queryOperationByID(
		ctx,
		transaction,
		params.OperationID,
	)
	if err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	if operation.State == domain.OperationStateFailed {
		tenant, tenantErr := queryTenant(
			ctx,
			transaction,
			params.TenantID,
		)
		if tenantErr != nil {
			return domain.Operation{}, domain.Tenant{}, tenantErr
		}

		if err := commit(ctx, transaction); err != nil {
			return domain.Operation{}, domain.Tenant{}, err
		}

		return operation, tenant, nil
	}

	if operation.State == domain.OperationStateSucceeded {
		return domain.Operation{},
			domain.Tenant{},
			domain.ErrOperationAlreadyCompleted
	}

	details, err := json.Marshal(params.Error.Details)
	if err != nil {
		return domain.Operation{},
			domain.Tenant{},
			fmt.Errorf(
				"marshal onboarding error details: %w",
				err,
			)
	}

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET
				status = 'provisioning_failed',
				version = version + 1,
				updated_at = $2
			WHERE tenant_id = $1::uuid
			  AND status = 'provisioning'
		`,
		params.TenantID,
		params.FailedAt,
	)
	if err != nil {
		return domain.Operation{},
			domain.Tenant{},
			fmt.Errorf(
				"mark tenant provisioning failed: %w",
				err,
			)
	}

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE tenant_onboarding_operations
			SET
				state = 'failed',
				error_code = $3,
				error_message = $4,
				error_retryable = $5,
				error_details = $6::jsonb,
				version = version + 1,
				updated_at = $7,
				completed_at = $7
			WHERE tenant_id = $1::uuid
			  AND operation_id = $2::uuid
			  AND state IN ('pending', 'running')
		`,
		params.TenantID,
		params.OperationID,
		params.Error.Code,
		params.Error.Message,
		params.Error.Retryable,
		details,
		params.FailedAt,
	)
	if err != nil {
		return domain.Operation{},
			domain.Tenant{},
			fmt.Errorf(
				"fail onboarding operation: %w",
				err,
			)
	}

	if params.Event == nil {
		return domain.Operation{},
			domain.Tenant{},
			domain.ErrInvalidArgument
	}

	operation, err = queryOperationByID(
		ctx,
		transaction,
		params.OperationID,
	)
	if err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	tenant, err := queryTenant(
		ctx,
		transaction,
		params.TenantID,
	)
	if err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	event, err := params.Event(tenant, operation)
	if err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.TenantID,
		event,
	); err != nil {
		return domain.Operation{},
			domain.Tenant{},
			err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Operation{}, domain.Tenant{}, err
	}

	return operation, tenant, nil
}

func queryTenant(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
) (domain.Tenant, error) {
	var tenant domain.Tenant
	var status string

	err := querier.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				slug,
				display_name,
				status,
				region,
				retention_days,
				version,
				created_by_user_id::text,
				created_at,
				updated_at,
				archived_at
			FROM tenant_tenants
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	).Scan(
		&tenant.ID,
		&tenant.Slug,
		&tenant.DisplayName,
		&status,
		&tenant.Region,
		&tenant.RetentionDays,
		&tenant.Version,
		&tenant.CreatedByUserID,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
		&tenant.ArchivedAt,
	)
	if err != nil {
		return domain.Tenant{}, mapDatabaseError(err)
	}

	tenant.Status = domain.Status(status)

	return tenant, nil
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func dereferenceBool(value *bool) bool {
	return value != nil && *value
}
