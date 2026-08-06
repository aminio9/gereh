package application

import (
	"context"
	"fmt"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/aminio9/gereh/services/organization-agent/internal/ports"
	"github.com/aminio9/gereh/services/organization-agent/internal/protoutil"
	"github.com/google/uuid"
)

// CreateCompanyInput is the input to company creation.
type CreateCompanyInput struct {
	ActorUserID string
	TenantID    string
	Slug        string
	DisplayName string
	Description string
}

// UpdateCompanyInput is the input to a company update.
type UpdateCompanyInput struct {
	ActorUserID     string
	TenantID        string
	CompanyID       string
	ExpectedVersion int64
	DisplayName     *string
	Description     *string
}

// EnsureDefaultCompanyInput is the input to tenant onboarding bootstrap.
type EnsureDefaultCompanyInput struct {
	TenantID              string
	OnboardingOperationID string
	ActorUserID           string
	TenantDisplayName     string
}

// CreateCompany validates, authorizes, and commits a new company.
func (service *Service) CreateCompany(
	ctx context.Context,
	input CreateCompanyInput,
) (domain.Company, error) {
	if err := validateUUID(
		"actor_user_id",
		input.ActorUserID,
	); err != nil {
		return domain.Company{}, err
	}

	if err := validateUUID(
		"tenant_id",
		input.TenantID,
	); err != nil {
		return domain.Company{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_COMPANY_CREATE,
	); err != nil {
		return domain.Company{}, err
	}

	slug, err := normalizeSlug(input.Slug)
	if err != nil {
		return domain.Company{}, err
	}

	displayName, err := boundedText(
		"display_name",
		input.DisplayName,
		1,
		120,
	)
	if err != nil {
		return domain.Company{}, err
	}

	description, err := boundedText(
		"description",
		input.Description,
		0,
		2000,
	)
	if err != nil {
		return domain.Company{}, err
	}

	companyID, err := uuid.NewV7()
	if err != nil {
		return domain.Company{}, fmt.Errorf(
			"generate company ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	company := domain.Company{
		TenantID:        input.TenantID,
		ID:              companyID.String(),
		Slug:            slug,
		DisplayName:     displayName,
		Description:     description,
		Status:          domain.CompanyStatusActive,
		IsDefault:       false,
		Version:         1,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.CompanyEventTopic,
		company.ID,
		"company.created",
		company.TenantID,
		"company",
		company.ID,
		company.Version,
		&organizationv1.CompanyCreated{
			Company: protoutil.Company(company),
		},
		now,
	)
	if err != nil {
		return domain.Company{}, err
	}

	return service.repository.CreateCompany(
		ctx,
		ports.CreateCompanyParams{
			ActorUserID: input.ActorUserID,
			Company:     company,
			Event:       event,
		},
	)
}

// GetCompany returns one company by identity.
func (service *Service) GetCompany(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) (domain.Company, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"company_id":    companyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Company{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_COMPANY_READ,
	); err != nil {
		return domain.Company{}, err
	}

	return service.repository.GetCompany(
		ctx,
		actorUserID,
		tenantID,
		companyID,
	)
}

// ListCompanies returns a paginated company page for a tenant.
func (service *Service) ListCompanies(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	pageSize int32,
	pageToken string,
	includeArchived bool,
) ([]domain.Company, string, error) {
	if err := validateUUID(
		"actor_user_id",
		actorUserID,
	); err != nil {
		return nil, "", err
	}

	if err := validateUUID(
		"tenant_id",
		tenantID,
	); err != nil {
		return nil, "", err
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_COMPANY_READ,
	); err != nil {
		return nil, "", err
	}

	limit := normalizePageSize(pageSize)

	cursorValue, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	var cursor *ports.CompanyCursor

	if cursorValue != "" {
		if err := validateUUID(
			"page_token",
			cursorValue,
		); err != nil {
			return nil, "", err
		}

		cursor = &ports.CompanyCursor{
			CompanyID: cursorValue,
		}
	}

	companies, err := service.repository.ListCompanies(
		ctx,
		actorUserID,
		tenantID,
		limit,
		cursor,
		includeArchived,
	)
	if err != nil {
		return nil, "", err
	}

	nextToken := ""

	if len(companies) == limit && len(companies) > 0 {
		nextToken = encodePageToken(
			companies[len(companies)-1].ID,
		)
	}

	return companies, nextToken, nil
}

// UpdateCompany validates and commits a versioned company update.
func (service *Service) UpdateCompany(
	ctx context.Context,
	input UpdateCompanyInput,
) (domain.Company, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"company_id":    input.CompanyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Company{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_COMPANY_UPDATE,
	); err != nil {
		return domain.Company{}, err
	}

	company, err := service.repository.GetCompany(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.CompanyID,
	)
	if err != nil {
		return domain.Company{}, err
	}

	if company.Status == domain.CompanyStatusArchived {
		return domain.Company{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	if input.DisplayName != nil {
		displayName, err := boundedText(
			"display_name",
			*input.DisplayName,
			1,
			120,
		)
		if err != nil {
			return domain.Company{}, err
		}

		company.DisplayName = displayName
	}

	if input.Description != nil {
		description, err := boundedText(
			"description",
			*input.Description,
			0,
			2000,
		)
		if err != nil {
			return domain.Company{}, err
		}

		company.Description = description
	}

	company.Version++
	company.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.CompanyEventTopic,
		company.ID,
		"company.updated",
		company.TenantID,
		"company",
		company.ID,
		company.Version,
		&organizationv1.CompanyUpdated{
			Company:         protoutil.Company(company),
			UpdatedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Company{}, err
	}

	return service.repository.UpdateCompany(
		ctx,
		ports.UpdateCompanyParams{
			ActorUserID:     input.ActorUserID,
			Company:         company,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)
}

// ArchiveCompany archives a company after checking invariants.
func (service *Service) ArchiveCompany(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	expectedVersion int64,
) (domain.Company, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"company_id":    companyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Company{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_COMPANY_ARCHIVE,
	); err != nil {
		return domain.Company{}, err
	}

	company, err := service.repository.GetCompany(
		ctx,
		actorUserID,
		tenantID,
		companyID,
	)
	if err != nil {
		return domain.Company{}, err
	}

	if company.IsDefault {
		return domain.Company{}, domain.ErrDefaultCompany
	}

	if company.Status == domain.CompanyStatusArchived {
		return company, nil
	}

	now := service.now().UTC()

	company.Status = domain.CompanyStatusArchived
	company.ArchivedAt = &now
	company.Version++
	company.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.CompanyEventTopic,
		company.ID,
		"company.archived",
		company.TenantID,
		"company",
		company.ID,
		company.Version,
		&organizationv1.CompanyArchived{
			Company:          protoutil.Company(company),
			ArchivedByUserId: actorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Company{}, err
	}

	return service.repository.ArchiveCompany(
		ctx,
		ports.UpdateCompanyParams{
			ActorUserID:     actorUserID,
			Company:         company,
			ExpectedVersion: expectedVersion,
			Event:           event,
		},
	)
}

// EnsureDefaultCompany idempotently creates the default company during tenant
// onboarding. It uses the service principal scope, never user authorization.
func (service *Service) EnsureDefaultCompany(
	ctx context.Context,
	input EnsureDefaultCompanyInput,
) (domain.Company, error) {
	for name, value := range map[string]string{
		"tenant_id":               input.TenantID,
		"onboarding_operation_id": input.OnboardingOperationID,
		"actor_user_id":           input.ActorUserID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Company{}, err
		}
	}

	displayName, err := boundedText(
		"tenant_display_name",
		input.TenantDisplayName,
		1,
		120,
	)
	if err != nil {
		return domain.Company{}, err
	}

	companyID, err := uuid.NewV7()
	if err != nil {
		return domain.Company{}, err
	}

	now := service.now().UTC()

	company := domain.Company{
		TenantID:        input.TenantID,
		ID:              companyID.String(),
		Slug:            "main",
		DisplayName:     displayName,
		Description:     "Default AI organization",
		Status:          domain.CompanyStatusActive,
		IsDefault:       true,
		Version:         1,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.CompanyEventTopic,
		company.ID,
		"company.created",
		company.TenantID,
		"company",
		company.ID,
		company.Version,
		&organizationv1.CompanyCreated{
			Company: protoutil.Company(company),
		},
		now,
	)
	if err != nil {
		return domain.Company{}, err
	}

	return service.repository.EnsureDefaultCompany(
		ctx,
		ports.EnsureDefaultCompanyParams{
			ServicePrincipalID:    service.config.BootstrapServicePrincipalID,
			OnboardingOperationID: input.OnboardingOperationID,
			Company:               company,
			Event:                 event,
		},
	)
}
