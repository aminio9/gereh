package application

import (
	"context"
	"errors"
	"testing"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/aminio9/gereh/services/organization-agent/internal/ports"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testActorID   = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae1"
	testTenantID  = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae2"
	testCompanyID = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae3"
	testAgentID   = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae4"
)

type mockRepository struct {
	mock.Mock
}

func (repository *mockRepository) CreateCompany(
	ctx context.Context,
	params ports.CreateCompanyParams,
) (domain.Company, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Company), args.Error(1)
}

func (repository *mockRepository) EnsureDefaultCompany(
	ctx context.Context,
	params ports.EnsureDefaultCompanyParams,
) (domain.Company, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Company), args.Error(1)
}

func (repository *mockRepository) GetCompany(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) (domain.Company, error) {
	args := repository.Called(ctx, actorUserID, tenantID, companyID)
	return args.Get(0).(domain.Company), args.Error(1)
}

func (repository *mockRepository) ListCompanies(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	limit int,
	cursor *ports.CompanyCursor,
	includeArchived bool,
) ([]domain.Company, error) {
	args := repository.Called(ctx, actorUserID, tenantID, limit, cursor, includeArchived)
	return args.Get(0).([]domain.Company), args.Error(1)
}

func (repository *mockRepository) UpdateCompany(
	ctx context.Context,
	params ports.UpdateCompanyParams,
) (domain.Company, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Company), args.Error(1)
}

func (repository *mockRepository) ArchiveCompany(
	ctx context.Context,
	params ports.UpdateCompanyParams,
) (domain.Company, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Company), args.Error(1)
}

func (repository *mockRepository) CreateAgent(
	ctx context.Context,
	params ports.CreateAgentParams,
) (domain.Agent, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Agent), args.Error(1)
}

func (repository *mockRepository) GetAgent(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	agentID string,
) (domain.Agent, error) {
	args := repository.Called(ctx, actorUserID, tenantID, agentID)
	return args.Get(0).(domain.Agent), args.Error(1)
}

func (repository *mockRepository) GetAgentAsService(
	ctx context.Context,
	tenantID string,
	servicePrincipalID string,
	agentID string,
) (domain.Agent, error) {
	args := repository.Called(ctx, tenantID, servicePrincipalID, agentID)
	return args.Get(0).(domain.Agent), args.Error(1)
}

func (repository *mockRepository) ListAgents(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	limit int,
	cursor *ports.AgentCursor,
	includeDeleted bool,
) ([]domain.Agent, error) {
	args := repository.Called(ctx, actorUserID, tenantID, companyID, limit, cursor, includeDeleted)
	return args.Get(0).([]domain.Agent), args.Error(1)
}

func (repository *mockRepository) UpdateAgent(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Agent), args.Error(1)
}

func (repository *mockRepository) SetAgentManager(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Agent), args.Error(1)
}

func (repository *mockRepository) ChangeAgentStatus(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Agent), args.Error(1)
}

func (repository *mockRepository) DeleteAgent(
	ctx context.Context,
	params ports.UpdateAgentParams,
) (domain.Agent, error) {
	args := repository.Called(ctx, params)
	return args.Get(0).(domain.Agent), args.Error(1)
}

func (repository *mockRepository) GetHierarchy(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) ([]domain.HierarchyNode, error) {
	args := repository.Called(ctx, actorUserID, tenantID, companyID)
	return args.Get(0).([]domain.HierarchyNode), args.Error(1)
}

func (repository *mockRepository) ClaimOutbox(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]domain.OutboxRecord, error) {
	args := repository.Called(ctx, limit, lease)
	return args.Get(0).([]domain.OutboxRecord), args.Error(1)
}

func (repository *mockRepository) MarkOutboxPublished(
	ctx context.Context,
	outboxID int64,
) error {
	args := repository.Called(ctx, outboxID)
	return args.Error(0)
}

func (repository *mockRepository) ReleaseOutbox(
	ctx context.Context,
	outboxID int64,
	retryAt time.Time,
	publishError string,
) error {
	args := repository.Called(ctx, outboxID, retryAt, publishError)
	return args.Error(0)
}

type mockAuthorizer struct {
	mock.Mock
}

func (authorizer *mockAuthorizer) Require(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	permission tenantv1.Permission,
) error {
	args := authorizer.Called(ctx, actorUserID, tenantID, permission)
	return args.Error(0)
}

func newTestService(
	repository *mockRepository,
	authorizer *mockAuthorizer,
) *Service {
	service, err := New(
		repository,
		authorizer,
		Config{
			CompanyEventTopic:           "gereh.organization.company.events.v1",
			AgentEventTopic:             "gereh.organization.agent.events.v1",
			BootstrapServicePrincipalID: "018f7767-28d2-7f5c-a693-0bb4c8ee4ae9",
		},
	)
	if err != nil {
		panic(err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	return service
}

func TestNewRejectsMissingConfig(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)

	_, err := New(
		repository,
		authorizer,
		Config{},
	)
	require.Error(t, err)

	_, err = New(
		nil,
		authorizer,
		Config{
			CompanyEventTopic:           "a",
			AgentEventTopic:             "b",
			BootstrapServicePrincipalID: testActorID,
		},
	)
	require.Error(t, err)

	_, err = New(
		repository,
		nil,
		Config{
			CompanyEventTopic:           "a",
			AgentEventTopic:             "b",
			BootstrapServicePrincipalID: testActorID,
		},
	)
	require.Error(t, err)
}

func TestCreateCompany(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_COMPANY_CREATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"CreateCompany",
			mock.Anything,
			mock.MatchedBy(func(params ports.CreateCompanyParams) bool {
				company := params.Company
				return company.TenantID == testTenantID &&
					company.Slug == "acme" &&
					company.Status == domain.CompanyStatusActive &&
					!company.IsDefault &&
					company.Version == 1
			}),
		).
		Return(domain.Company{}, nil).
		Once()

	_, err := service.CreateCompany(
		context.Background(),
		CreateCompanyInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			Slug:        "Acme",
			DisplayName: "Acme Corp",
			Description: "Test company",
		},
	)
	require.NoError(t, err)

	authorizer.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func TestCreateCompanyRejectsForbidden(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_COMPANY_CREATE,
		).
		Return(domain.ErrForbidden).
		Once()

	_, err := service.CreateCompany(
		context.Background(),
		CreateCompanyInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			Slug:        "acme",
			DisplayName: "Acme Corp",
		},
	)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestArchiveCompanyRejectsDefault(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	now := service.now()

	company := domain.Company{
		TenantID:        testTenantID,
		ID:              testCompanyID,
		Slug:            "main",
		DisplayName:     "Default",
		Status:          domain.CompanyStatusActive,
		IsDefault:       true,
		Version:         1,
		CreatedByUserID: testActorID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_COMPANY_ARCHIVE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetCompany",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompanyID,
		).
		Return(company, nil).
		Once()

	_, err := service.ArchiveCompany(
		context.Background(),
		testActorID,
		testTenantID,
		testCompanyID,
		1,
	)
	require.ErrorIs(t, err, domain.ErrDefaultCompany)
}

func TestCreateAgentValidatesManagerInSameCompany(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_AGENT_CREATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"CreateAgent",
			mock.Anything,
			mock.MatchedBy(func(params ports.CreateAgentParams) bool {
				agent := params.Agent
				return agent.TenantID == testTenantID &&
					agent.CompanyID == testCompanyID &&
					agent.Slug == "worker" &&
					agent.Status == domain.AgentStatusDraft &&
					agent.ExecutionProfile == domain.ExecutionProfileBalanced &&
					agent.AutonomyLevel == domain.AutonomyApprovalRequired &&
					agent.Version == 1
			}),
		).
		Return(domain.Agent{}, nil).
		Once()

	_, err := service.CreateAgent(
		context.Background(),
		CreateAgentInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompanyID,
			Slug:        "worker",
			DisplayName: "Worker",
			RoleTitle:   "Worker",
			Objective:   "Process tasks",
			Capabilities: []string{
				"compute",
				"storage",
			},
		},
	)
	require.NoError(t, err)
}

func TestSetAgentManagerDetectsSelfManagement(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	now := service.now()

	current := domain.Agent{
		TenantID:  testTenantID,
		CompanyID: testCompanyID,
		ID:        testAgentID,
		Slug:      "worker",
		Status:    domain.AgentStatusDraft,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_AGENT_HIERARCHY_MANAGE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetAgent",
			mock.Anything,
			testActorID,
			testTenantID,
			testAgentID,
		).
		Return(current, nil).
		Once()

	_, err := service.SetAgentManager(
		context.Background(),
		SetAgentManagerInput{
			ActorUserID:     testActorID,
			TenantID:        testTenantID,
			AgentID:         testAgentID,
			ExpectedVersion: 1,
			ManagerAgentID:  stringPointer(testAgentID),
		},
	)
	require.ErrorIs(t, err, domain.ErrHierarchyCycle)
}

func TestPauseAgentRejectsUnpausableStatus(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	now := service.now()

	current := domain.Agent{
		TenantID:  testTenantID,
		CompanyID: testCompanyID,
		ID:        testAgentID,
		Status:    domain.AgentStatusDraft,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_AGENT_LIFECYCLE_MANAGE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetAgent",
			mock.Anything,
			testActorID,
			testTenantID,
			testAgentID,
		).
		Return(current, nil).
		Once()

	_, err := service.PauseAgent(
		context.Background(),
		LifecycleInput{
			ActorUserID:     testActorID,
			TenantID:        testTenantID,
			AgentID:         testAgentID,
			ExpectedVersion: 1,
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestDeleteAgentRejectsReadyStatus(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	now := service.now()

	current := domain.Agent{
		TenantID:  testTenantID,
		CompanyID: testCompanyID,
		ID:        testAgentID,
		Status:    domain.AgentStatusReady,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_AGENT_DELETE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetAgent",
			mock.Anything,
			testActorID,
			testTenantID,
			testAgentID,
		).
		Return(current, nil).
		Once()

	_, err := service.DeleteAgent(
		context.Background(),
		LifecycleInput{
			ActorUserID:     testActorID,
			TenantID:        testTenantID,
			AgentID:         testAgentID,
			ExpectedVersion: 1,
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestUpdateCompanyRejectsArchived(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	now := service.now()

	company := domain.Company{
		TenantID:        testTenantID,
		ID:              testCompanyID,
		Slug:            "acme",
		DisplayName:     "Acme",
		Status:          domain.CompanyStatusArchived,
		Version:         2,
		CreatedByUserID: testActorID,
		CreatedAt:       now,
		UpdatedAt:       now,
		ArchivedAt:      &now,
	}

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_COMPANY_UPDATE,
		).
		Return(nil).
		Once()

	repository.
		On(
			"GetCompany",
			mock.Anything,
			testActorID,
			testTenantID,
			testCompanyID,
		).
		Return(company, nil).
		Once()

	_, err := service.UpdateCompany(
		context.Background(),
		UpdateCompanyInput{
			ActorUserID:     testActorID,
			TenantID:        testTenantID,
			CompanyID:       testCompanyID,
			ExpectedVersion: 2,
			DisplayName:     stringPointer("New Name"),
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidTransition)
}

func TestEnsureDefaultCompany(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	repository.
		On(
			"EnsureDefaultCompany",
			mock.Anything,
			mock.MatchedBy(func(params ports.EnsureDefaultCompanyParams) bool {
				company := params.Company
				return company.TenantID == testTenantID &&
					company.Slug == "main" &&
					company.IsDefault &&
					company.Description == "Default AI organization" &&
					params.OnboardingOperationID == "018f7767-28d2-7f5c-a693-0bb4c8ee4ae5"
			}),
		).
		Return(domain.Company{}, nil).
		Once()

	_, err := service.EnsureDefaultCompany(
		context.Background(),
		EnsureDefaultCompanyInput{
			TenantID:              testTenantID,
			OnboardingOperationID: "018f7767-28d2-7f5c-a693-0bb4c8ee4ae5",
			ActorUserID:           testActorID,
			TenantDisplayName:     "Acme Corp",
		},
	)
	require.NoError(t, err)
}

func TestCreateAgentRejectsSecretConfiguration(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_AGENT_CREATE,
		).
		Return(nil).
		Once()

	_, err := service.CreateAgent(
		context.Background(),
		CreateAgentInput{
			ActorUserID: testActorID,
			TenantID:    testTenantID,
			CompanyID:   testCompanyID,
			Slug:        "worker",
			DisplayName: "Worker",
			RoleTitle:   "Worker",
			Objective:   "Process tasks",
			Configuration: map[string]any{
				"api_key": "secret",
			},
		},
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestGetAgentRequiresPermission(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_AGENT_READ,
		).
		Return(errors.New("tenant is not active")).
		Once()

	_, err := service.GetAgent(
		context.Background(),
		testActorID,
		testTenantID,
		testAgentID,
	)
	require.Error(t, err)
}

func TestListCompaniesNextToken(t *testing.T) {
	t.Parallel()

	repository := new(mockRepository)
	authorizer := new(mockAuthorizer)
	service := newTestService(repository, authorizer)

	authorizer.
		On(
			"Require",
			mock.Anything,
			testActorID,
			testTenantID,
			tenantv1.Permission_PERMISSION_COMPANY_READ,
		).
		Return(nil).
		Once()

	now := service.now()

	var fullPage []domain.Company

	for index := 0; index < 50; index++ {
		fullPage = append(fullPage, domain.Company{
			TenantID:    testTenantID,
			ID:          testCompanyID,
			Slug:        "acme",
			DisplayName: "Acme",
			Status:      domain.CompanyStatusActive,
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	repository.
		On(
			"ListCompanies",
			mock.Anything,
			testActorID,
			testTenantID,
			50,
			(*ports.CompanyCursor)(nil),
			false,
		).
		Return(
			fullPage,
			nil,
		).
		Once()

	companies, nextToken, err := service.ListCompanies(
		context.Background(),
		testActorID,
		testTenantID,
		50,
		"",
		false,
	)
	require.NoError(t, err)
	require.Len(t, companies, 50)
	require.NotEmpty(t, nextToken)
}

func stringPointer(value string) *string {
	return &value
}
