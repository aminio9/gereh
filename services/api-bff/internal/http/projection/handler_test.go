package projection

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	projectionv1 "github.com/aminio9/gereh/gen/go/gereh/projection/v1"
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

const testActorUser = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae1"

var testTenantID = uuid.NewString()

type mockProjectionClient struct {
	mock.Mock
}

func (client *mockProjectionClient) GetDashboardSummary(
	ctx context.Context,
	request *projectionv1.GetDashboardSummaryRequest,
	_ ...grpc.CallOption,
) (*projectionv1.GetDashboardSummaryResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*projectionv1.GetDashboardSummaryResponse),
		args.Error(1)
}

func (client *mockProjectionClient) GetCompanyOverview(
	ctx context.Context,
	request *projectionv1.GetCompanyOverviewRequest,
	_ ...grpc.CallOption,
) (*projectionv1.GetCompanyOverviewResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*projectionv1.GetCompanyOverviewResponse),
		args.Error(1)
}

func (client *mockProjectionClient) ListAgentOverviews(
	ctx context.Context,
	request *projectionv1.ListAgentOverviewsRequest,
	_ ...grpc.CallOption,
) (*projectionv1.ListAgentOverviewsResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*projectionv1.ListAgentOverviewsResponse),
		args.Error(1)
}

func (client *mockProjectionClient) ListTaskActivity(
	ctx context.Context,
	request *projectionv1.ListTaskActivityRequest,
	_ ...grpc.CallOption,
) (*projectionv1.ListTaskActivityResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*projectionv1.ListTaskActivityResponse),
		args.Error(1)
}

func (client *mockProjectionClient) Search(
	ctx context.Context,
	request *projectionv1.SearchRequest,
	_ ...grpc.CallOption,
) (*projectionv1.SearchResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*projectionv1.SearchResponse),
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*tenantv1.CheckAuthorizationResponse),
		args.Error(1)
}

func newTestHandler(
	client *mockProjectionClient,
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
	client *mockProjectionClient,
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
						UserId:  testActorUser,
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
) *http.Request {
	request := httptest.NewRequestWithContext(
		context.Background(),
		method,
		path,
		nil,
	)
	request.Header.Set("Origin", "http://localhost:8080")
	request.AddCookie(
		&http.Cookie{
			Name:  "session",
			Value: "session-id",
		},
	)

	return request.WithContext(
		platformauth.WithPrincipal(
			request.Context(),
			platformauth.Principal{
				UserID: testActorUser,
			},
		),
	)
}

func allowAuthorization(
	authorizer *mockPermissionChecker,
) {
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
		)
}

func TestDashboardRouteReturnsReadModel(t *testing.T) {
	t.Parallel()

	client := new(mockProjectionClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	defer authorizer.AssertExpectations(t)

	allowAuthorization(authorizer)

	client.
		On(
			"GetDashboardSummary",
			mock.Anything,
			&projectionv1.GetDashboardSummaryRequest{
				ActorUserId: testActorUser,
				TenantId:    testTenantID,
			},
		).
		Return(
			&projectionv1.GetDashboardSummaryResponse{
				Summary: &projectionv1.DashboardSummary{
					CompaniesTotal: 2,
					TasksCompleted: 5,
				},
			},
			nil,
		)

	router := newTestRouter(t, client, authorizer)

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/dashboard",
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusOK, writer.Code)

	var body struct {
		Summary struct {
			CompaniesTotal string `json:"companiesTotal"`
			TasksCompleted string `json:"tasksCompleted"`
		} `json:"summary"`
	}

	require.NoError(
		t,
		json.Unmarshal(writer.Body.Bytes(), &body),
	)
	require.Equal(t, "2", body.Summary.CompaniesTotal)
	require.Equal(t, "5", body.Summary.TasksCompleted)
}

func TestSearchRoutePassesQuery(t *testing.T) {
	t.Parallel()

	client := new(mockProjectionClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	defer authorizer.AssertExpectations(t)

	allowAuthorization(authorizer)

	client.
		On(
			"Search",
			mock.Anything,
			&projectionv1.SearchRequest{
				ActorUserId: testActorUser,
				TenantId:    testTenantID,
				Query:       "backend agent",
				PageSize:    25,
			},
		).
		Return(
			&projectionv1.SearchResponse{
				Results: []*projectionv1.SearchResult{
					{
						Type:  projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_AGENT,
						Id:    uuid.NewString(),
						Title: "Backend Agent",
					},
				},
			},
			nil,
		)

	router := newTestRouter(t, client, authorizer)

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/search?q=backend+agent",
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusOK, writer.Code)
	require.Contains(t, writer.Body.String(), "Backend Agent")
}

func TestProjectionForbiddenWhenPermissionDenied(t *testing.T) {
	t.Parallel()

	client := new(mockProjectionClient)
	defer client.AssertExpectations(t)

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
					Allowed: false,
				},
			},
			nil,
		)

	router := newTestRouter(t, client, authorizer)

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/dashboard",
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusForbidden, writer.Code)

	// The projection client must not be called.
	client.AssertNotCalled(
		t,
		"GetDashboardSummary",
		mock.Anything,
		mock.Anything,
	)
}

func TestProjectionMapsUnauthenticatedTo401(t *testing.T) {
	t.Parallel()

	client := new(mockProjectionClient)
	authorizer := new(mockPermissionChecker)

	router := newTestRouter(t, client, authorizer)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/dashboard",
		nil,
	)
	request.Header.Set("Origin", "http://localhost:8080")

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusUnauthorized, writer.Code)
}

func TestProjectionMapsServiceUnavailableTo503(t *testing.T) {
	t.Parallel()

	client := new(mockProjectionClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	defer authorizer.AssertExpectations(t)

	allowAuthorization(authorizer)

	client.
		On(
			"GetCompanyOverview",
			mock.Anything,
			mock.Anything,
		).
		Return(
			nil,
			status.Error(
				codes.Unavailable,
				"projection unavailable",
			),
		)

	router := newTestRouter(t, client, authorizer)

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+
			"/companies/"+uuid.NewString()+"/overview",
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusServiceUnavailable, writer.Code)
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*identityv1.BeginLoginResponse),
		args.Error(1)
}

func (client *mockIdentityClient) CompleteLogin(
	ctx context.Context,
	request *identityv1.CompleteLoginRequest,
	_ ...grpc.CallOption,
) (*identityv1.CompleteLoginResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*identityv1.CompleteLoginResponse),
		args.Error(1)
}

func (client *mockIdentityClient) GetSession(
	ctx context.Context,
	request *identityv1.GetSessionRequest,
	_ ...grpc.CallOption,
) (*identityv1.GetSessionResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*identityv1.GetSessionResponse),
		args.Error(1)
}

func (client *mockIdentityClient) DeleteSession(
	ctx context.Context,
	request *identityv1.DeleteSessionRequest,
	_ ...grpc.CallOption,
) (*identityv1.DeleteSessionResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*identityv1.DeleteSessionResponse),
		args.Error(1)
}
