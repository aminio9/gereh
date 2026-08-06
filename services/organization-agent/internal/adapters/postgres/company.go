package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/aminio9/gereh/services/organization-agent/internal/ports"
	"github.com/jackc/pgx/v5"
)

// CreateCompany commits a new company and its outbox event atomically.
func (repository *Repository) CreateCompany(
	ctx context.Context,
	params ports.CreateCompanyParams,
) (domain.Company, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Company.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Company{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := insertCompany(
		ctx,
		transaction,
		params.Company,
	); err != nil {
		return domain.Company{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Company.TenantID,
		params.Event,
	); err != nil {
		return domain.Company{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Company{}, err
	}

	return params.Company, nil
}

// EnsureDefaultCompany idempotently creates the tenant default company under
// the service principal scope.
func (repository *Repository) EnsureDefaultCompany(
	ctx context.Context,
	params ports.EnsureDefaultCompanyParams,
) (domain.Company, error) {
	transaction, err := repository.beginServiceTenant(
		ctx,
		params.Company.TenantID,
		params.ServicePrincipalID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Company{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var existingCompanyID string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT company_id::text
			FROM organization_bootstrap_requests
			WHERE onboarding_operation_id = $1::uuid
			  AND tenant_id = $2::uuid
		`,
		params.OnboardingOperationID,
		params.Company.TenantID,
	).Scan(&existingCompanyID)

	if err == nil {
		company, queryErr := queryCompany(
			ctx,
			transaction,
			params.Company.TenantID,
			existingCompanyID,
		)
		if queryErr != nil {
			return domain.Company{}, queryErr
		}

		if err := commit(ctx, transaction); err != nil {
			return domain.Company{}, err
		}

		return company, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Company{}, fmt.Errorf(
			"query default-company bootstrap: %w",
			err,
		)
	}

	// Serialize default-company creation for this tenant so concurrent
	// onboarding retries cannot create duplicate default companies.
	_, err = transaction.Exec(
		ctx,
		`
			SELECT pg_advisory_xact_lock(
				hashtextextended($1, 0)
			)
		`,
		params.Company.TenantID,
	)
	if err != nil {
		return domain.Company{}, fmt.Errorf(
			"lock default-company bootstrap: %w",
			err,
		)
	}

	err = transaction.QueryRow(
		ctx,
		`
			SELECT company_id::text
			FROM organization_companies
			WHERE tenant_id = $1::uuid
			  AND is_default
			  AND status = 'active'
		`,
		params.Company.TenantID,
	).Scan(&existingCompanyID)

	if err == nil {
		if err := recordBootstrapRequest(
			ctx,
			transaction,
			params,
			existingCompanyID,
		); err != nil {
			return domain.Company{}, err
		}

		company, queryErr := queryCompany(
			ctx,
			transaction,
			params.Company.TenantID,
			existingCompanyID,
		)
		if queryErr != nil {
			return domain.Company{}, queryErr
		}

		if err := commit(ctx, transaction); err != nil {
			return domain.Company{}, err
		}

		return company, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Company{}, err
	}

	if err := insertCompany(
		ctx,
		transaction,
		params.Company,
	); err != nil {
		return domain.Company{}, err
	}

	if err := recordBootstrapRequest(
		ctx,
		transaction,
		params,
		params.Company.ID,
	); err != nil {
		return domain.Company{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Company.TenantID,
		params.Event,
	); err != nil {
		return domain.Company{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Company{}, err
	}

	return params.Company, nil
}

// GetCompany returns one company by identity.
func (repository *Repository) GetCompany(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) (domain.Company, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Company{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	company, err := queryCompany(
		ctx,
		transaction,
		tenantID,
		companyID,
	)
	if err != nil {
		return domain.Company{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Company{}, err
	}

	return company, nil
}

// ListCompanies lists a tenant's companies after the cursor.
func (repository *Repository) ListCompanies(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	limit int,
	cursor *ports.CompanyCursor,
	includeArchived bool,
) ([]domain.Company, error) {
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
			slug,
			display_name,
			description,
			status,
			is_default,
			version,
			created_by_user_id::text,
			created_at,
			updated_at,
			archived_at
		FROM organization_companies
		WHERE tenant_id = $1::uuid
	`

	args := []any{tenantID}

	if cursor != nil {
		args = append(args, cursor.CompanyID)
		query += fmt.Sprintf(
			" AND company_id > $%d::uuid",
			len(args),
		)
	}

	if !includeArchived {
		query += ` AND status = 'active'`
	}

	args = append(args, limit)
	query += fmt.Sprintf(
		`
			ORDER BY company_id
			LIMIT $%d
		`,
		len(args),
	)

	rows, err := transaction.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"list organization companies: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var companies []domain.Company

	for rows.Next() {
		company, err := scanCompany(rows)
		if err != nil {
			return nil, err
		}

		companies = append(companies, company)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate organization companies: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return companies, nil
}

// UpdateCompany applies an optimistic-concurrency company update.
func (repository *Repository) UpdateCompany(
	ctx context.Context,
	params ports.UpdateCompanyParams,
) (domain.Company, error) {
	return repository.updateCompany(
		ctx,
		params,
		`
			UPDATE organization_companies
			SET
				display_name = $4,
				description = $5,
				version = version + 1,
				updated_at = $6
			WHERE tenant_id = $1::uuid
			  AND company_id = $2::uuid
			  AND version = $3
		`,
	)
}

// ArchiveCompany archives a company after verifying no active agents remain.
func (repository *Repository) ArchiveCompany(
	ctx context.Context,
	params ports.UpdateCompanyParams,
) (domain.Company, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Company.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Company{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := lockCompany(
		ctx,
		transaction,
		params.Company.TenantID,
		params.Company.ID,
	); err != nil {
		return domain.Company{}, err
	}

	var hasAgents bool

	err = transaction.QueryRow(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM organization_agents
				WHERE tenant_id = $1::uuid
				  AND company_id = $2::uuid
				  AND status <> 'deleted'
			)
		`,
		params.Company.TenantID,
		params.Company.ID,
	).Scan(&hasAgents)
	if err != nil {
		return domain.Company{}, fmt.Errorf(
			"check company active agents: %w",
			err,
		)
	}

	if hasAgents {
		return domain.Company{}, domain.ErrCompanyHasAgents
	}

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE organization_companies
			SET
				status = 'archived',
				archived_at = $4,
				version = version + 1,
				updated_at = $5
			WHERE tenant_id = $1::uuid
			  AND company_id = $2::uuid
			  AND version = $3
		`,
		params.Company.TenantID,
		params.Company.ID,
		params.ExpectedVersion,
		params.Company.ArchivedAt,
		params.Company.UpdatedAt,
	)
	if err != nil {
		return domain.Company{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectVersionOrMissing(
			ctx,
			transaction,
			params.Company.TenantID,
			params.Company.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Company{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Company.TenantID,
		params.Event,
	); err != nil {
		return domain.Company{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Company{}, err
	}

	return params.Company, nil
}

func (repository *Repository) updateCompany(
	ctx context.Context,
	params ports.UpdateCompanyParams,
	updateSQL string,
) (domain.Company, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Company.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Company{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		updateSQL,
		params.Company.TenantID,
		params.Company.ID,
		params.ExpectedVersion,
		params.Company.DisplayName,
		params.Company.Description,
		params.Company.UpdatedAt,
	)
	if err != nil {
		return domain.Company{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectVersionOrMissing(
			ctx,
			transaction,
			params.Company.TenantID,
			params.Company.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Company{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Company.TenantID,
		params.Event,
	); err != nil {
		return domain.Company{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Company{}, err
	}

	return params.Company, nil
}

func rejectVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	companyID string,
	expectedVersion int64,
) error {
	var currentVersion int64

	err := transaction.QueryRow(
		ctx,
		`
			SELECT version
			FROM organization_companies
			WHERE tenant_id = $1::uuid
			  AND company_id = $2::uuid
		`,
		tenantID,
		companyID,
	).Scan(&currentVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query company version: %w",
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

func recordBootstrapRequest(
	ctx context.Context,
	transaction pgx.Tx,
	params ports.EnsureDefaultCompanyParams,
	companyID string,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO organization_bootstrap_requests (
				onboarding_operation_id,
				tenant_id,
				company_id,
				actor_user_id,
				created_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5
			)
			ON CONFLICT (
				onboarding_operation_id
			) DO NOTHING
		`,
		params.OnboardingOperationID,
		params.Company.TenantID,
		companyID,
		params.Company.CreatedByUserID,
		params.Company.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"insert default-company bootstrap: %w",
			mapDatabaseError(err),
		)
	}

	return nil
}

func insertCompany(
	ctx context.Context,
	transaction pgx.Tx,
	company domain.Company,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO organization_companies (
				tenant_id,
				company_id,
				slug,
				display_name,
				description,
				status,
				is_default,
				version,
				created_by_user_id,
				created_at,
				updated_at,
				archived_at
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
				$11,
				$12
			)
		`,
		company.TenantID,
		company.ID,
		company.Slug,
		company.DisplayName,
		company.Description,
		string(company.Status),
		company.IsDefault,
		company.Version,
		company.CreatedByUserID,
		company.CreatedAt,
		company.UpdatedAt,
		company.ArchivedAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return nil
}

func queryCompany(
	ctx context.Context,
	querier rowQuerier,
	tenantID string,
	companyID string,
) (domain.Company, error) {
	return scanCompany(querier.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				company_id::text,
				slug,
				display_name,
				description,
				status,
				is_default,
				version,
				created_by_user_id::text,
				created_at,
				updated_at,
				archived_at
			FROM organization_companies
			WHERE tenant_id = $1::uuid
			  AND company_id = $2::uuid
		`,
		tenantID,
		companyID,
	))
}

func scanCompany(
	scanner rowScanner,
) (domain.Company, error) {
	var company domain.Company
	var status string

	err := scanner.Scan(
		&company.TenantID,
		&company.ID,
		&company.Slug,
		&company.DisplayName,
		&company.Description,
		&status,
		&company.IsDefault,
		&company.Version,
		&company.CreatedByUserID,
		&company.CreatedAt,
		&company.UpdatedAt,
		&company.ArchivedAt,
	)
	if err != nil {
		return domain.Company{}, mapDatabaseError(err)
	}

	company.Status = domain.CompanyStatus(status)

	return company, nil
}
