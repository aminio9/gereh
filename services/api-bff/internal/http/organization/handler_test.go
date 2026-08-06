package organization

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	bffconfig "github.com/aminio9/gereh/services/api-bff/internal/config"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	testActorUserID = uuid.NewString()
	testTenantID    = uuid.NewString()
	testCompanyID   = uuid.NewString()
	testAgentID     = uuid.NewString()
)

type mockOrgClient struct {
	mock.Mock
}

func (client *mockOrgClient) CreateCompany(
	ctx context.Context,
	request *organizationv1.CreateCompanyRequest,
	_ ...grpc.CallOption,
) (*organizationv1.CreateCompanyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.CreateCompanyResponse),
		args.Error(1)
}

func (client *mockOrgClient) GetCompany(
	ctx context.Context,
	request *organizationv1.GetCompanyRequest,
	_ ...grpc.CallOption,
) (*organizationv1.GetCompanyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.GetCompanyResponse),
		args.Error(1)
}

func (client *mockOrgClient) ListCompanies(
	ctx context.Context,
	request *organizationv1.ListCompaniesRequest,
	_ ...grpc.CallOption,
) (*organizationv1.ListCompaniesResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.ListCompaniesResponse),
		args.Error(1)
}

func (client *mockOrgClient) UpdateCompany(
	ctx context.Context,
	request *organizationv1.UpdateCompanyRequest,
	_ ...grpc.CallOption,
) (*organizationv1.UpdateCompanyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.UpdateCompanyResponse),
		args.Error(1)
}

func (client *mockOrgClient) ArchiveCompany(
	ctx context.Context,
	request *organizationv1.ArchiveCompanyRequest,
	_ ...grpc.CallOption,
) (*organizationv1.ArchiveCompanyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.ArchiveCompanyResponse),
		args.Error(1)
}

func (client *mockOrgClient) CreateAgent(
	ctx context.Context,
	request *organizationv1.CreateAgentRequest,
	_ ...grpc.CallOption,
) (*organizationv1.CreateAgentResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.CreateAgentResponse),
		args.Error(1)
}

func (client *mockOrgClient) GetAgent(
	ctx context.Context,
	request *organizationv1.GetAgentRequest,
	_ ...grpc.CallOption,
) (*organizationv1.GetAgentResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.GetAgentResponse),
		args.Error(1)
}

func (client *mockOrgClient) ListAgents(
	ctx context.Context,
	request *organizationv1.ListAgentsRequest,
	_ ...grpc.CallOption,
) (*organizationv1.ListAgentsResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.ListAgentsResponse),
		args.Error(1)
}

func (client *mockOrgClient) UpdateAgent(
	ctx context.Context,
	request *organizationv1.UpdateAgentRequest,
	_ ...grpc.CallOption,
) (*organizationv1.UpdateAgentResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.UpdateAgentResponse),
		args.Error(1)
}

func (client *mockOrgClient) SetAgentManager(
	ctx context.Context,
	request *organizationv1.SetAgentManagerRequest,
	_ ...grpc.CallOption,
) (*organizationv1.SetAgentManagerResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.SetAgentManagerResponse),
		args.Error(1)
}

func (client *mockOrgClient) PauseAgent(
	ctx context.Context,
	request *organizationv1.PauseAgentRequest,
	_ ...grpc.CallOption,
) (*organizationv1.PauseAgentResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.PauseAgentResponse),
		args.Error(1)
}

func (client *mockOrgClient) ResumeAgent(
	ctx context.Context,
	request *organizationv1.ResumeAgentRequest,
	_ ...grpc.CallOption,
) (*organizationv1.ResumeAgentResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.ResumeAgentResponse),
		args.Error(1)
}

func (client *mockOrgClient) DeleteAgent(
	ctx context.Context,
	request *organizationv1.DeleteAgentRequest,
	_ ...grpc.CallOption,
) (*organizationv1.DeleteAgentResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.DeleteAgentResponse),
		args.Error(1)
}

func (client *mockOrgClient) GetAgentHierarchy(
	ctx context.Context,
	request *organizationv1.GetAgentHierarchyRequest,
	_ ...grpc.CallOption,
) (*organizationv1.GetAgentHierarchyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*organizationv1.GetAgentHierarchyResponse),
		args.Error(1)
}

type mockPermissionChecker struct {
	mock.Mock
}

func (checker *mockPermissionChecker) CheckAuthorization(
	ctx context.Context,
	request *tenantv1.CheckAuthorizationRequest,
	_ ...grpc.CallOption,
) (*tenantv1.CheckAuthorizationResponse, error) {
	args := checker.Called(ctx, request)
	return args.Get(0).(*tenantv1.CheckAuthorizationResponse),
		args.Error(1)
}

type mockIdentityClient struct {
	mock.Mock
}

func (client *mockIdentityClient) BeginLogin(
	ctx context.Context,
	request *identityv1.BeginLoginRequest,
	_ ...grpc.CallOption,
) (*identityv1.BeginLoginResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*identityv1.BeginLoginResponse),
		args.Error(1)
}

func (client *mockIdentityClient) CompleteLogin(
	ctx context.Context,
	request *identityv1.CompleteLoginRequest,
	_ ...grpc.CallOption,
) (*identityv1.CompleteLoginResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*identityv1.CompleteLoginResponse),
		args.Error(1)
}

func (client *mockIdentityClient) GetSession(
	ctx context.Context,
	request *identityv1.GetSessionRequest,
	_ ...grpc.CallOption,
) (*identityv1.GetSessionResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*identityv1.GetSessionResponse),
		args.Error(1)
}

func (client *mockIdentityClient) DeleteSession(
	ctx context.Context,
	request *identityv1.DeleteSessionRequest,
	_ ...grpc.CallOption,
) (*identityv1.DeleteSessionResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*identityv1.DeleteSessionResponse),
		args.Error(1)
}

func newTestHandler(
	client *mockOrgClient,
	authorizer *mockPermissionChecker,
) *Handler {
	return New(
		client,
		authorizer,
		slog.New(slog.NewTextHandler(
			&strings.Builder{},
			nil,
		)),
	)
}

func newTestRouter(
	t *testing.T,
	client *mockOrgClient,
	authorizer *mockPermissionChecker,
) chi.Router {
	t.Helper()

	router := chi.NewRouter()

	authConfig := bffconfig.AuthConfig{
		IdentityTarget:        "passthrough:///unused",
		IdentityInsecure:      true,
		WebOrigin:             "http://localhost:8080",
		SessionCookieName:     "session",
		TransactionCookieName: "transaction",
		CSRFCookieName:        "csrf",
		CookieSecure:          false,
	}

	identity := new(mockIdentityClient)

	identity.
		On(
			"GetSession",
			mock.Anything,
			mock.Anything,
		).
		Return(
			&identityv1.GetSessionResponse{
				Session: &identityv1.Session{
					User: &identityv1.AuthenticatedUser{
						UserId:  testActorUserID,
						Issuer:  "test",
						Subject: "test",
						Email:   "a@b.test",
					},
					CreatedAt: timestamppb.Now(),
					ExpiresAt: timestamppb.New(
						time.Now().Add(time.Hour),
					),
				},
			},
			nil,
		)

	authHandler := authhttp.NewHandler(
		authConfig,
		identity,
		slog.Default(),
	)

	handler := newTestHandler(client, authorizer)
	handler.Register(router, authHandler)

	return router
}

func authenticatedRequest(
	method string,
	path string,
	body string,
) *http.Request {
	request := httptest.NewRequestWithContext(
		context.Background(),
		method,
		path,
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:8080")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	request.AddCookie(
		&http.Cookie{
			Name:  "session",
			Value: "session-id",
		},
	)
	request.AddCookie(
		&http.Cookie{
			Name:  "csrf",
			Value: "csrf-token",
		},
	)

	return request.WithContext(
		platformauth.WithPrincipal(
			request.Context(),
			platformauth.Principal{
				UserID: testActorUserID,
			},
		),
	)
}

// withRouteParams attaches chi route parameters for direct handler calls.
func withRouteParams(
	request *http.Request,
	params map[string]string,
) *http.Request {
	routeContext := chi.NewRouteContext()
	for name, value := range params {
		routeContext.URLParams.Add(name, value)
	}

	return request.WithContext(
		context.WithValue(
			request.Context(),
			chi.RouteCtxKey,
			routeContext,
		),
	)
}

func TestCreateCompanyAdminReturnsCreated(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	defer authorizer.AssertExpectations(t)

	handler := newTestHandler(client, authorizer)

	companyID := uuid.NewString()

	client.
		On(
			"CreateCompany",
			mock.Anything,
			mock.MatchedBy(func(
				request *organizationv1.CreateCompanyRequest,
			) bool {
				return request.GetActorUserId() ==
					testActorUserID &&
					request.GetTenantId() ==
						testTenantID &&
					request.GetSlug() == "acme"
			}),
		).
		Return(
			&organizationv1.CreateCompanyResponse{
				Company: &organizationv1.Company{
					TenantId:    testTenantID,
					CompanyId:   companyID,
					Slug:        "acme",
					DisplayName: "Acme",
					Status:      organizationv1.CompanyStatus_COMPANY_STATUS_ACTIVE,
					Version:     1,
				},
			},
			nil,
		).
		Once()

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/companies",
		`{
			"slug": "acme",
			"displayName": "Acme",
			"description": "Test"
		}`,
	)
	request = withRouteParams(request, map[string]string{
		"tenantID": testTenantID,
	})

	writer := httptest.NewRecorder()

	handler.createCompany(writer, request)

	require.Equal(t, http.StatusCreated, writer.Code)
}

func TestCreateCompanyRequiresAuthentication(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/companies",
		strings.NewReader(`{
			"slug": "acme",
			"displayName": "Acme"
		}`),
	)

	writer := httptest.NewRecorder()

	handler.createCompany(writer, request)

	require.Equal(t, http.StatusUnauthorized, writer.Code)
}

func TestUpdateCompanyStaleVersionReturnsConflict(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	client.
		On(
			"UpdateCompany",
			mock.Anything,
			mock.Anything,
		).
		Return(
			(*organizationv1.UpdateCompanyResponse)(nil),
			status.Error(
				codes.Aborted,
				"version conflict",
			),
		).
		Once()

	request := authenticatedRequest(
		http.MethodPatch,
		"/v1/tenants/"+testTenantID+"/companies/"+testCompanyID,
		`{
			"expectedVersion": 1,
			"displayName": "New Name"
		}`,
	)
	request = withRouteParams(request, map[string]string{
		"tenantID":  testTenantID,
		"companyID": testCompanyID,
	})

	writer := httptest.NewRecorder()

	handler.updateCompany(writer, request)

	require.Equal(t, http.StatusConflict, writer.Code)
}

func TestGetCompanyUnknownReturnsNotFound(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	client.
		On(
			"GetCompany",
			mock.Anything,
			mock.Anything,
		).
		Return(
			(*organizationv1.GetCompanyResponse)(nil),
			status.Error(
				codes.NotFound,
				"not found",
			),
		).
		Once()

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/companies/"+testCompanyID,
		"",
	)
	request = withRouteParams(request, map[string]string{
		"tenantID":  testTenantID,
		"companyID": testCompanyID,
	})

	writer := httptest.NewRecorder()

	handler.getCompany(writer, request)

	require.Equal(t, http.StatusNotFound, writer.Code)
}

func TestSetAgentManagerHierarchyCycleReturnsConflict(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	client.
		On(
			"SetAgentManager",
			mock.Anything,
			mock.Anything,
		).
		Return(
			(*organizationv1.SetAgentManagerResponse)(nil),
			status.Error(
				codes.FailedPrecondition,
				"hierarchy cycle",
			),
		).
		Once()

	request := authenticatedRequest(
		http.MethodPut,
		"/v1/tenants/"+testTenantID+"/agents/"+testAgentID+"/manager",
		`{
			"expectedVersion": 1,
			"managerAgentId": "018f7767-28d2-7f5c-a693-0bb4c8ee4aa1"
		}`,
	)
	request = withRouteParams(request, map[string]string{
		"tenantID": testTenantID,
		"agentID":  testAgentID,
	})

	writer := httptest.NewRecorder()

	handler.setAgentManager(writer, request)

	require.Equal(t, http.StatusConflict, writer.Code)
}

func TestViewerCreateCompanyReturnsForbidden(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	authorizer := new(mockPermissionChecker)
	defer authorizer.AssertExpectations(t)

	authorizer.
		On(
			"CheckAuthorization",
			mock.Anything,
			mock.MatchedBy(func(
				request *tenantv1.CheckAuthorizationRequest,
			) bool {
				return request.GetPermission() ==
					tenantv1.Permission_PERMISSION_COMPANY_CREATE &&
					request.GetTenantId() == testTenantID
			}),
		).
		Return(
			&tenantv1.CheckAuthorizationResponse{
				Decision: &tenantv1.AuthorizationDecision{
					Allowed: false,
				},
			},
			nil,
		).
		Once()

	router := newTestRouter(
		t,
		client,
		authorizer,
	)

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/companies",
		`{
			"slug": "acme",
			"displayName": "Acme"
		}`,
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusForbidden, writer.Code)
}

func TestCreateCompanyWithoutCSRFReturnsForbidden(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	authorizer := new(mockPermissionChecker)
	defer authorizer.AssertExpectations(t)

	authorizer.
		On(
			"CheckAuthorization",
			mock.Anything,
			mock.Anything,
		).
		Return(
			&tenantv1.CheckAuthorizationResponse{
				Decision: &tenantv1.AuthorizationDecision{
					Allowed: true,
				},
			},
			nil,
		).
		Maybe()

	router := newTestRouter(
		t,
		client,
		authorizer,
	)

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/companies",
		`{
			"slug": "acme",
			"displayName": "Acme"
		}`,
	)
	request.Header.Del("X-CSRF-Token")

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusForbidden, writer.Code)
}

func TestTenantIDComesFromRoute(t *testing.T) {
	t.Parallel()

	client := new(mockOrgClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	// The body tries to smuggle a different tenant ID; unknown fields are
	// rejected and the route value remains authoritative.
	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/companies",
		`{
			"slug": "acme",
			"displayName": "Acme",
			"tenantId": "018f7767-28d2-7f5c-a693-0bb4c8ee4aaa"
		}`,
	)

	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(
		"tenantID",
		testTenantID,
	)

	request = request.WithContext(
		context.WithValue(
			request.Context(),
			chi.RouteCtxKey,
			routeContext,
		),
	)

	writer := httptest.NewRecorder()

	handler.createCompany(writer, request)

	require.Equal(t, http.StatusBadRequest, writer.Code)
}
