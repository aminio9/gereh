package policy

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
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

const (
	testPolicyID  = "policy-1"
	testVersionID = "version-1"
	testActorUser = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae1"
)

var testTenantID = uuid.NewString()

type mockPolicyClient struct {
	mock.Mock
}

func (client *mockPolicyClient) CreatePolicy(
	ctx context.Context,
	request *policyv1.CreatePolicyRequest,
	_ ...grpc.CallOption,
) (*policyv1.CreatePolicyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.CreatePolicyResponse), args.Error(1)
}

func (client *mockPolicyClient) GetPolicy(
	ctx context.Context,
	request *policyv1.GetPolicyRequest,
	_ ...grpc.CallOption,
) (*policyv1.GetPolicyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.GetPolicyResponse), args.Error(1)
}

func (client *mockPolicyClient) ListPolicies(
	ctx context.Context,
	request *policyv1.ListPoliciesRequest,
	_ ...grpc.CallOption,
) (*policyv1.ListPoliciesResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.ListPoliciesResponse), args.Error(1)
}

func (client *mockPolicyClient) CreatePolicyVersion(
	ctx context.Context,
	request *policyv1.CreatePolicyVersionRequest,
	_ ...grpc.CallOption,
) (*policyv1.CreatePolicyVersionResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.CreatePolicyVersionResponse), args.Error(1)
}

func (client *mockPolicyClient) ActivatePolicy(
	ctx context.Context,
	request *policyv1.ActivatePolicyRequest,
	_ ...grpc.CallOption,
) (*policyv1.ActivatePolicyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.ActivatePolicyResponse), args.Error(1)
}

func (client *mockPolicyClient) ArchivePolicy(
	ctx context.Context,
	request *policyv1.ArchivePolicyRequest,
	_ ...grpc.CallOption,
) (*policyv1.ArchivePolicyResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.ArchivePolicyResponse), args.Error(1)
}

func (client *mockPolicyClient) GetDecision(
	ctx context.Context,
	request *policyv1.GetDecisionRequest,
	_ ...grpc.CallOption,
) (*policyv1.GetDecisionResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.GetDecisionResponse), args.Error(1)
}

func (client *mockPolicyClient) ListDecisions(
	ctx context.Context,
	request *policyv1.ListDecisionsRequest,
	_ ...grpc.CallOption,
) (*policyv1.ListDecisionsResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*policyv1.ListDecisionsResponse), args.Error(1)
}

func (client *mockPolicyClient) CheckAuthorization(
	ctx context.Context,
	request *tenantv1.CheckAuthorizationRequest,
	_ ...grpc.CallOption,
) (*tenantv1.CheckAuthorizationResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*tenantv1.CheckAuthorizationResponse), args.Error(1)
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

	return args.Get(0).(*tenantv1.CheckAuthorizationResponse), args.Error(1)
}

func newTestHandler(
	client *mockPolicyClient,
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
	client *mockPolicyClient,
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
				UserID: testActorUser,
			},
		),
	)
}

func TestCreatePolicyRouteReturnsCreated(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	defer client.AssertExpectations(t)

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
					tenantv1.Permission_PERMISSION_POLICY_CREATE
			}),
		).
		Return(
			&tenantv1.CheckAuthorizationResponse{
				Decision: &tenantv1.AuthorizationDecision{
					Allowed: true,
				},
			},
			nil,
		).
		Once()

	client.
		On(
			"CreatePolicy",
			mock.Anything,
			mock.MatchedBy(func(
				request *policyv1.CreatePolicyRequest,
			) bool {
				return request.GetName() == "Cost Guard" &&
					request.GetScope().GetType() ==
						policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_AGENT
			}),
		).
		Return(
			&policyv1.CreatePolicyResponse{
				Policy: &policyv1.Policy{
					PolicyId: testPolicyID,
					Name:     "Cost Guard",
				},
			},
			nil,
		).
		Once()

	router := newTestRouter(t, client, authorizer)

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/policies",
		`{
			"scopeType": 3,
			"scopeId": "agent-1",
			"name": "Cost Guard",
			"description": "Bounds agent spend"
		}`,
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusCreated, writer.Code)
	require.Contains(t, writer.Body.String(), "Cost Guard")
}

func TestCreatePolicyRouteForbidsWithoutPermission(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
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
					tenantv1.Permission_PERMISSION_POLICY_CREATE
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

	router := newTestRouter(t, client, authorizer)

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/policies",
		`{
			"scopeType": 3,
			"scopeId": "agent-1",
			"name": "Cost Guard"
		}`,
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusForbidden, writer.Code)
}

func TestCreatePolicyReturnsCreated(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	defer authorizer.AssertExpectations(t)

	handler := newTestHandler(client, authorizer)

	client.
		On(
			"CreatePolicy",
			mock.Anything,
			mock.MatchedBy(func(
				request *policyv1.CreatePolicyRequest,
			) bool {
				return request.GetName() == "Cost Guard" &&
					request.GetScope().GetType() ==
						policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_AGENT
			}),
		).
		Return(
			&policyv1.CreatePolicyResponse{
				Policy: &policyv1.Policy{
					PolicyId: testPolicyID,
					Name:     "Cost Guard",
				},
			},
			nil,
		).
		Once()

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/policies",
		`{
			"scopeType": 3,
			"scopeId": "agent-1",
			"name": "Cost Guard",
			"description": "Bounds agent spend"
		}`,
	)

	writer := httptest.NewRecorder()

	handler.createPolicy(writer, request)

	require.Equal(t, http.StatusCreated, writer.Code)
	require.Contains(t, writer.Body.String(), "Cost Guard")
}

func TestCreatePolicyVersionWithRules(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	client.
		On(
			"CreatePolicyVersion",
			mock.Anything,
			mock.MatchedBy(func(
				request *policyv1.CreatePolicyVersionRequest,
			) bool {
				rules := request.GetRules()
				return request.GetTenantId() == testTenantID &&
					request.GetPolicyId() == testPolicyID &&
					len(rules) == 1 &&
					rules[0].GetCondition() ==
						"request.estimated_cost_usd < 100"
			}),
		).
		Return(
			&policyv1.CreatePolicyVersionResponse{
				Version: &policyv1.PolicyVersion{
					TenantId:      testTenantID,
					PolicyId:      testPolicyID,
					PolicyVersion: 1,
				},
			},
			nil,
		).
		Once()

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/policies/"+testPolicyID+"/versions",
		`{
			"expectedResourceVersion": 0,
			"defaultEffect": 4,
			"rules": [{
				"ruleId": "rule-1",
				"priority": 1,
				"name": "Low spend",
				"enabled": true,
				"effect": 4,
				"condition": "request.estimated_cost_usd < 100",
				"actionPatterns": ["work.task.*"],
				"resourceTypes": ["task"]
			}],
			"notes": "initial budget"
		}`,
	)
	request = withRouteParams(request, map[string]string{
		"tenantID": testTenantID,
		"policyID": testPolicyID,
	})

	writer := httptest.NewRecorder()

	handler.createPolicyVersion(writer, request)

	require.Equal(t, http.StatusCreated, writer.Code)
}

func TestCreatePolicyInvalidBody(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	request := authenticatedRequest(
		http.MethodPost,
		"/v1/tenants/"+testTenantID+"/policies",
		`{"name": `,
	)

	writer := httptest.NewRecorder()

	handler.createPolicy(writer, request)

	require.Equal(t, http.StatusBadRequest, writer.Code)
}

func TestGetPolicyForbidden(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	client.
		On(
			"GetPolicy",
			mock.Anything,
			mock.Anything,
		).
		Return(
			nil,
			status.Error(
				codes.PermissionDenied,
				"denied",
			),
		).
		Once()

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/policies/"+testPolicyID,
		"",
	)

	writer := httptest.NewRecorder()

	handler.getPolicy(writer, request)

	require.Equal(t, http.StatusForbidden, writer.Code)
}

func TestGetPolicyReturnsPolicy(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	client.
		On(
			"GetPolicy",
			mock.Anything,
			mock.MatchedBy(func(
				request *policyv1.GetPolicyRequest,
			) bool {
				return request.GetTenantId() == testTenantID &&
					request.GetPolicyId() == testPolicyID
			}),
		).
		Return(
			&policyv1.GetPolicyResponse{
				Policy: &policyv1.Policy{
					TenantId:        testTenantID,
					PolicyId:        testPolicyID,
					Name:            "Cost Guard",
					ResourceVersion: 1,
				},
				ActiveVersion: &policyv1.PolicyVersion{
					TenantId:      testTenantID,
					PolicyId:      testPolicyID,
					PolicyVersion: 1,
				},
			},
			nil,
		).
		Once()

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/policies/"+testPolicyID,
		"",
	)
	request = withRouteParams(request, map[string]string{
		"tenantID": testTenantID,
		"policyID": testPolicyID,
	})

	writer := httptest.NewRecorder()

	handler.getPolicy(writer, request)

	require.Equal(t, http.StatusOK, writer.Code)
	require.Contains(t, writer.Body.String(), "Cost Guard")
}

func TestListDecisionsReturnsDecisions(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	defer client.AssertExpectations(t)

	authorizer := new(mockPermissionChecker)
	handler := newTestHandler(client, authorizer)

	client.
		On(
			"ListDecisions",
			mock.Anything,
			mock.MatchedBy(func(
				request *policyv1.ListDecisionsRequest,
			) bool {
				return request.GetTenantId() == testTenantID
			}),
		).
		Return(
			&policyv1.ListDecisionsResponse{
				Decisions: []*policyv1.PolicyDecision{
					{
						DecisionId: "decision-1",
						TenantId:   testTenantID,
						Effect:     policyv1.PolicyEffect_POLICY_EFFECT_ALLOW,
					},
				},
			},
			nil,
		).
		Once()

	request := authenticatedRequest(
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/decisions",
		"",
	)
	request = withRouteParams(request, map[string]string{
		"tenantID": testTenantID,
	})

	writer := httptest.NewRecorder()

	handler.listDecisions(writer, request)

	require.Equal(t, http.StatusOK, writer.Code)
	require.Contains(t, writer.Body.String(), "decision-1")
}

func TestUnauthenticatedReturnsUnauthorized(t *testing.T) {
	t.Parallel()

	client := new(mockPolicyClient)
	authorizer := new(mockPermissionChecker)

	router := newTestRouter(t, client, authorizer)

	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/tenants/"+testTenantID+"/policies",
		nil,
	)

	writer := httptest.NewRecorder()

	router.ServeHTTP(writer, request)

	require.Equal(t, http.StatusUnauthorized, writer.Code)
}

type mockIdentityClient struct {
	mock.Mock
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

	return args.Get(0).(*identityv1.GetSessionResponse), args.Error(1)
}

func (client *mockIdentityClient) BeginLogin(
	ctx context.Context,
	request *identityv1.BeginLoginRequest,
	_ ...grpc.CallOption,
) (*identityv1.BeginLoginResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*identityv1.BeginLoginResponse), args.Error(1)
}

func (client *mockIdentityClient) CompleteLogin(
	ctx context.Context,
	request *identityv1.CompleteLoginRequest,
	_ ...grpc.CallOption,
) (*identityv1.CompleteLoginResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*identityv1.CompleteLoginResponse), args.Error(1)
}

func (client *mockIdentityClient) DeleteSession(
	ctx context.Context,
	request *identityv1.DeleteSessionRequest,
	_ ...grpc.CallOption,
) (*identityv1.DeleteSessionResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*identityv1.DeleteSessionResponse), args.Error(1)
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
