package tenant

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockClient struct {
	mock.Mock
}

func (client *mockClient) CreateTenant(
	ctx context.Context,
	request *tenantv1.CreateTenantRequest,
	_ ...grpc.CallOption,
) (*tenantv1.CreateTenantResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*tenantv1.CreateTenantResponse),
		args.Error(1)
}

func (client *mockClient) GetTenant(
	ctx context.Context,
	request *tenantv1.GetTenantRequest,
	_ ...grpc.CallOption,
) (*tenantv1.GetTenantResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.GetTenantResponse),
		args.Error(1)
}

func (client *mockClient) ListTenants(
	ctx context.Context,
	request *tenantv1.ListTenantsRequest,
	_ ...grpc.CallOption,
) (*tenantv1.ListTenantsResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.ListTenantsResponse),
		args.Error(1)
}

func (client *mockClient) UpdateTenant(
	ctx context.Context,
	request *tenantv1.UpdateTenantRequest,
	_ ...grpc.CallOption,
) (*tenantv1.UpdateTenantResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.UpdateTenantResponse),
		args.Error(1)
}

func (client *mockClient) ArchiveTenant(
	ctx context.Context,
	request *tenantv1.ArchiveTenantRequest,
	_ ...grpc.CallOption,
) (*tenantv1.ArchiveTenantResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.ArchiveTenantResponse),
		args.Error(1)
}

func (client *mockClient) GetTenantContext(
	ctx context.Context,
	request *tenantv1.GetTenantContextRequest,
	_ ...grpc.CallOption,
) (*tenantv1.GetTenantContextResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.GetTenantContextResponse),
		args.Error(1)
}

func (client *mockClient) CheckAuthorization(
	ctx context.Context,
	request *tenantv1.CheckAuthorizationRequest,
	_ ...grpc.CallOption,
) (*tenantv1.CheckAuthorizationResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.CheckAuthorizationResponse),
		args.Error(1)
}

func (client *mockClient) ListMembers(
	ctx context.Context,
	request *tenantv1.ListMembersRequest,
	_ ...grpc.CallOption,
) (*tenantv1.ListMembersResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.ListMembersResponse),
		args.Error(1)
}

func (client *mockClient) AddMember(
	ctx context.Context,
	request *tenantv1.AddMemberRequest,
	_ ...grpc.CallOption,
) (*tenantv1.AddMemberResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.AddMemberResponse),
		args.Error(1)
}

func (client *mockClient) UpdateMemberRole(
	ctx context.Context,
	request *tenantv1.UpdateMemberRoleRequest,
	_ ...grpc.CallOption,
) (*tenantv1.UpdateMemberRoleResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.UpdateMemberRoleResponse),
		args.Error(1)
}

func (client *mockClient) RemoveMember(
	ctx context.Context,
	request *tenantv1.RemoveMemberRequest,
	_ ...grpc.CallOption,
) (*tenantv1.RemoveMemberResponse, error) {
	args := client.Called(ctx, request)
	return args.Get(0).(*tenantv1.RemoveMemberResponse),
		args.Error(1)
}

func (client *mockClient) GetOperation(
	ctx context.Context,
	request *tenantv1.GetOperationRequest,
	_ ...grpc.CallOption,
) (*tenantv1.GetOperationResponse, error) {
	args := client.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*tenantv1.GetOperationResponse),
		args.Error(1)
}

func TestCreateTenantReturnsAcceptedOperation(
	t *testing.T,
) {
	t.Parallel()

	client := new(mockClient)
	defer client.AssertExpectations(t)

	handler := New(client, slog.Default())

	actorID := uuid.NewString()
	operationID := uuid.NewString()

	client.
		On(
			"CreateTenant",
			mock.Anything,
			mock.MatchedBy(func(
				request *tenantv1.CreateTenantRequest,
			) bool {
				return request.GetActorUserId() ==
					actorID
			}),
		).
		Return(
			&tenantv1.CreateTenantResponse{
				Operation: &commonv1.Operation{
					OperationId:  operationID,
					ResourceName: "tenants/" + uuid.NewString(),
				},
			},
			nil,
		).
		Once()

	request := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost,
		"/v1/onboarding",
		strings.NewReader(`{
			"slug": "acme-ai",
			"displayName": "Acme AI"
		}`),
	)
	request = request.WithContext(
		platformauth.WithPrincipal(
			request.Context(),
			platformauth.Principal{
				UserID: actorID,
			},
		),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)
	request.Header.Set(
		"Idempotency-Key",
		uuid.NewString(),
	)

	writer := httptest.NewRecorder()

	handler.createTenant(writer, request)

	require.Equal(t, http.StatusAccepted, writer.Code)
	require.Equal(
		t,
		"/v1/operations/"+operationID,
		writer.Header().Get("Location"),
	)
	require.Equal(t, "2", writer.Header().Get("Retry-After"))
}

func TestCreateTenantRequiresIdempotencyKey(
	t *testing.T,
) {
	t.Parallel()

	client := new(mockClient)
	handler := New(client, slog.Default())

	request := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost,
		"/v1/onboarding",
		strings.NewReader(`{
			"slug": "acme-ai",
			"displayName": "Acme AI"
		}`),
	)
	request = request.WithContext(
		platformauth.WithPrincipal(
			request.Context(),
			platformauth.Principal{
				UserID: uuid.NewString(),
			},
		),
	)
	request.Header.Set("Content-Type", "application/json")

	writer := httptest.NewRecorder()

	handler.createTenant(writer, request)

	require.Equal(t, http.StatusBadRequest, writer.Code)
}

func TestCreateTenantRequiresAuthentication(
	t *testing.T,
) {
	t.Parallel()

	client := new(mockClient)
	handler := New(client, slog.Default())

	request := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost,
		"/v1/onboarding",
		strings.NewReader(`{
			"slug": "acme-ai",
			"displayName": "Acme AI"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Idempotency-Key",
		uuid.NewString(),
	)

	writer := httptest.NewRecorder()

	handler.createTenant(writer, request)

	require.Equal(t, http.StatusUnauthorized, writer.Code)
}

func TestCreateTenantBadGatewayOnMissingOperation(
	t *testing.T,
) {
	t.Parallel()

	client := new(mockClient)
	defer client.AssertExpectations(t)

	handler := New(client, slog.Default())

	client.
		On(
			"CreateTenant",
			mock.Anything,
			mock.Anything,
		).
		Return(
			&tenantv1.CreateTenantResponse{},
			nil,
		).
		Once()

	request := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost,
		"/v1/onboarding",
		strings.NewReader(`{
			"slug": "acme-ai",
			"displayName": "Acme AI"
		}`),
	)
	request = request.WithContext(
		platformauth.WithPrincipal(
			request.Context(),
			platformauth.Principal{
				UserID: uuid.NewString(),
			},
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Idempotency-Key",
		uuid.NewString(),
	)

	writer := httptest.NewRecorder()

	handler.createTenant(writer, request)

	require.Equal(t, http.StatusBadGateway, writer.Code)
}

func operationRequest(
	t *testing.T,
	method string,
	path string,
	operationID string,
) *http.Request {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), method, path, nil)

	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(
		"operationID",
		operationID,
	)

	request = request.WithContext(
		context.WithValue(
			request.Context(),
			chi.RouteCtxKey,
			routeContext,
		),
	)

	return request
}

func TestGetOperationReturnsOperation(
	t *testing.T) {
	t.Parallel()

	client := new(mockClient)
	defer client.AssertExpectations(t)

	handler := New(client, slog.Default())

	actorID := uuid.NewString()
	operationID := uuid.NewString()

	client.
		On(
			"GetOperation",
			mock.Anything,
			mock.MatchedBy(func(
				request *tenantv1.GetOperationRequest,
			) bool {
				return request.GetActorUserId() == actorID &&
					request.GetOperationId() == operationID
			}),
		).
		Return(
			&tenantv1.GetOperationResponse{
				Operation: &commonv1.Operation{
					OperationId:  operationID,
					ResourceName: "tenants/" + uuid.NewString(),
				},
			},
			nil,
		).
		Once()

	request := operationRequest(
		t,
		http.MethodGet,
		"/v1/operations/"+operationID,
		operationID,
	)
	request = request.WithContext(
		platformauth.WithPrincipal(
			request.Context(),
			platformauth.Principal{
				UserID: actorID,
			},
		),
	)

	writer := httptest.NewRecorder()

	handler.getOperation(writer, request)

	require.Equal(t, http.StatusOK, writer.Code)
}

func TestGetOperationNotFound(
	t *testing.T) {
	t.Parallel()

	client := new(mockClient)
	defer client.AssertExpectations(t)

	handler := New(client, slog.Default())

	client.
		On(
			"GetOperation",
			mock.Anything,
			mock.Anything,
		).
		Return(
			nil,
			status.Error(
				codes.NotFound,
				"operation not found",
			),
		).
		Once()

	request := operationRequest(
		t,
		http.MethodGet,
		"/v1/operations/"+uuid.NewString(),
		uuid.NewString(),
	)
	request = request.WithContext(
		platformauth.WithPrincipal(
			request.Context(),
			platformauth.Principal{
				UserID: uuid.NewString(),
			},
		),
	)

	writer := httptest.NewRecorder()

	handler.getOperation(writer, request)

	require.Equal(t, http.StatusNotFound, writer.Code)
}
