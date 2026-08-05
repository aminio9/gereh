package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Client is implemented by the generated Tenant Service client.
type Client interface {
	CreateTenant(
		context.Context,
		*tenantv1.CreateTenantRequest,
		...grpc.CallOption,
	) (*tenantv1.CreateTenantResponse, error)

	GetTenant(
		context.Context,
		*tenantv1.GetTenantRequest,
		...grpc.CallOption,
	) (*tenantv1.GetTenantResponse, error)

	ListTenants(
		context.Context,
		*tenantv1.ListTenantsRequest,
		...grpc.CallOption,
	) (*tenantv1.ListTenantsResponse, error)

	UpdateTenant(
		context.Context,
		*tenantv1.UpdateTenantRequest,
		...grpc.CallOption,
	) (*tenantv1.UpdateTenantResponse, error)

	ArchiveTenant(
		context.Context,
		*tenantv1.ArchiveTenantRequest,
		...grpc.CallOption,
	) (*tenantv1.ArchiveTenantResponse, error)

	GetTenantContext(
		context.Context,
		*tenantv1.GetTenantContextRequest,
		...grpc.CallOption,
	) (*tenantv1.GetTenantContextResponse, error)

	CheckAuthorization(
		context.Context,
		*tenantv1.CheckAuthorizationRequest,
		...grpc.CallOption,
	) (*tenantv1.CheckAuthorizationResponse, error)

	ListMembers(
		context.Context,
		*tenantv1.ListMembersRequest,
		...grpc.CallOption,
	) (*tenantv1.ListMembersResponse, error)

	AddMember(
		context.Context,
		*tenantv1.AddMemberRequest,
		...grpc.CallOption,
	) (*tenantv1.AddMemberResponse, error)

	UpdateMemberRole(
		context.Context,
		*tenantv1.UpdateMemberRoleRequest,
		...grpc.CallOption,
	) (*tenantv1.UpdateMemberRoleResponse, error)

	RemoveMember(
		context.Context,
		*tenantv1.RemoveMemberRequest,
		...grpc.CallOption,
	) (*tenantv1.RemoveMemberResponse, error)
}

// Handler exposes Tenant Service operations through the BFF.
type Handler struct {
	client Client
	logger *slog.Logger
}

// New creates Tenant Service HTTP handlers.
func New(
	client Client,
	logger *slog.Logger,
) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		client: client,
		logger: logger,
	}
}

// Register registers authenticated and authorized tenant routes.
func (handler *Handler) Register(
	router chi.Router,
	authHandler *authhttp.Handler,
) {
	router.Route(
		"/v1/tenants",
		func(tenantRouter chi.Router) {
			tenantRouter.Use(
				authHandler.RequireSession,
			)

			// These operations are scoped only to the authenticated user,
			// because no tenant exists or has been selected yet.
			tenantRouter.Get(
				"/",
				handler.listTenants,
			)

			tenantRouter.With(
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.createTenant,
			)

			tenantRouter.Route(
				"/{tenantID}",
				func(resource chi.Router) {
					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_TENANT_READ,
						),
					).Get(
						"/",
						handler.getTenant,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_TENANT_UPDATE,
						),
						authHandler.RequireCSRF,
					).Patch(
						"/",
						handler.updateTenant,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_TENANT_ARCHIVE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/archive",
						handler.archiveTenant,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_MEMBER_LIST,
						),
					).Get(
						"/members",
						handler.listMembers,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_MEMBER_ADD,
						),
						authHandler.RequireCSRF,
					).Post(
						"/members",
						handler.addMember,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_MEMBER_UPDATE_ROLE,
						),
						authHandler.RequireCSRF,
					).Patch(
						"/members/{userID}",
						handler.updateMemberRole,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_MEMBER_REMOVE,
						),
						authHandler.RequireCSRF,
					).Delete(
						"/members/{userID}",
						handler.removeMember,
					)
				},
			)
		},
	)
}

type createTenantRequest struct {
	Slug          string `json:"slug"`
	DisplayName   string `json:"displayName"`
	Region        string `json:"region"`
	RetentionDays int32  `json:"retentionDays"`
}

type updateTenantRequest struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	DisplayName     *string `json:"displayName"`
	Region          *string `json:"region"`
	RetentionDays   *int32  `json:"retentionDays"`
}

type archiveTenantRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

type memberRequest struct {
	UserID string              `json:"userId"`
	Role   tenantv1.TenantRole `json:"role"`
}

type updateMemberRequest struct {
	Role                      tenantv1.TenantRole `json:"role"`
	ExpectedMembershipVersion int64               `json:"expectedMembershipVersion"`
}

func (handler *Handler) createTenant(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok := principal(request)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	idempotencyKey := strings.TrimSpace(
		request.Header.Get("Idempotency-Key"),
	)
	if idempotencyKey == "" {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"idempotency_key_required",
			"Idempotency-Key is required",
		)
		return
	}

	var input createTenantRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid tenant request",
		)
		return
	}

	response, err := handler.client.CreateTenant(
		request.Context(),
		&tenantv1.CreateTenantRequest{
			ActorUserId:   principal.UserID,
			RequestId:     idempotencyKey,
			Slug:          input.Slug,
			DisplayName:   input.DisplayName,
			Region:        input.Region,
			RetentionDays: input.RetentionDays,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(
		writer,
		http.StatusCreated,
		response,
	)
}

func (handler *Handler) listTenants(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok := principal(request)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListTenants(
		request.Context(),
		&tenantv1.ListTenantsRequest{
			ActorUserId: principal.UserID,
			PageSize:    int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) getTenant(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok := principal(request)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	response, err := handler.client.GetTenant(
		request.Context(),
		&tenantv1.GetTenantRequest{
			ActorUserId: principal.UserID,
			TenantId: chi.URLParam(
				request,
				"tenantID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) updateTenant(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateTenantRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid tenant request",
		)
		return
	}

	response, err := handler.client.UpdateTenant(
		request.Context(),
		&tenantv1.UpdateTenantRequest{
			ActorUserId:     principal.UserID,
			TenantId:        chi.URLParam(request, "tenantID"),
			ExpectedVersion: input.ExpectedVersion,
			DisplayName:     input.DisplayName,
			Region:          input.Region,
			RetentionDays:   input.RetentionDays,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) archiveTenant(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input archiveTenantRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid tenant request",
		)
		return
	}

	response, err := handler.client.ArchiveTenant(
		request.Context(),
		&tenantv1.ArchiveTenantRequest{
			ActorUserId: principal.UserID,
			TenantId: chi.URLParam(
				request,
				"tenantID",
			),
			ExpectedVersion: input.ExpectedVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listMembers(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListMembers(
		request.Context(),
		&tenantv1.ListMembersRequest{
			ActorUserId: principal.UserID,
			TenantId: chi.URLParam(
				request,
				"tenantID",
			),
			PageSize: int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) addMember(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input memberRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid membership request",
		)
		return
	}

	response, err := handler.client.AddMember(
		request.Context(),
		&tenantv1.AddMemberRequest{
			ActorUserId: principal.UserID,
			TenantId: chi.URLParam(
				request,
				"tenantID",
			),
			UserId: input.UserID,
			Role:   input.Role,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(
		writer,
		http.StatusCreated,
		response,
	)
}

func (handler *Handler) updateMemberRole(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateMemberRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid membership request",
		)
		return
	}

	response, err := handler.client.UpdateMemberRole(
		request.Context(),
		&tenantv1.UpdateMemberRoleRequest{
			ActorUserId: principal.UserID,
			TenantId: chi.URLParam(
				request,
				"tenantID",
			),
			UserId: chi.URLParam(
				request,
				"userID",
			),
			Role: input.Role,
			ExpectedMembershipVersion: input.
				ExpectedMembershipVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) removeMember(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	version, err := strconv.ParseInt(
		request.URL.Query().Get("expected_version"),
		10,
		64,
	)
	if err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"expected_version is required",
		)
		return
	}

	response, err := handler.client.RemoveMember(
		request.Context(),
		&tenantv1.RemoveMemberRequest{
			ActorUserId: principal.UserID,
			TenantId: chi.URLParam(
				request,
				"tenantID",
			),
			UserId: chi.URLParam(
				request,
				"userID",
			),
			ExpectedMembershipVersion: version,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func principal(
	request *http.Request,
) (platformauth.Principal, bool) {
	return platformauth.PrincipalFromContext(
		request.Context(),
	)
}

func decodeJSON(
	writer http.ResponseWriter,
	request *http.Request,
	target any,
) error {
	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		1<<20,
	)

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(
		err,
		io.EOF,
	) {
		return errors.New(
			"request body must contain one JSON value",
		)
	}

	return nil
}

func writeProto(
	writer http.ResponseWriter,
	statusCode int,
	message interface {
		ProtoReflect() protoreflect.Message
	},
) {
	value, err := protojson.MarshalOptions{
		UseProtoNames:   false,
		EmitUnpopulated: false,
	}.Marshal(message)
	if err != nil {
		writeProblem(
			writer,
			http.StatusInternalServerError,
			"encoding_failed",
			"Response encoding failed",
		)
		return
	}

	writer.Header().Set(
		"Content-Type",
		"application/json",
	)
	writer.Header().Set(
		"Cache-Control",
		"no-store",
	)
	writer.WriteHeader(statusCode)

	_, _ = writer.Write(value)
}

func (handler *Handler) writeGRPCError(
	writer http.ResponseWriter,
	err error,
) {
	switch status.Code(err) {
	case codes.InvalidArgument:
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid tenant request",
		)

	case codes.NotFound:
		writeProblem(
			writer,
			http.StatusNotFound,
			"not_found",
			"Tenant resource not found",
		)

	case codes.PermissionDenied:
		writeProblem(
			writer,
			http.StatusForbidden,
			"forbidden",
			"Tenant operation forbidden",
		)

	case codes.AlreadyExists:
		writeProblem(
			writer,
			http.StatusConflict,
			"conflict",
			"Tenant resource already exists",
		)

	case codes.Aborted:
		writeProblem(
			writer,
			http.StatusConflict,
			"version_conflict",
			"Tenant resource changed; reload and retry",
		)

	case codes.FailedPrecondition:
		writeProblem(
			writer,
			http.StatusUnprocessableEntity,
			"failed_precondition",
			"Tenant operation cannot be completed",
		)

	case codes.Unavailable, codes.DeadlineExceeded:
		writeProblem(
			writer,
			http.StatusServiceUnavailable,
			"tenant_unavailable",
			"Tenant Service is unavailable",
		)

	default:
		handler.logger.ErrorContext(
			context.Background(),
			"Tenant Service request failed",
			"error",
			err,
		)

		writeProblem(
			writer,
			http.StatusInternalServerError,
			"tenant_error",
			"Tenant operation failed",
		)
	}
}

func writeProblem(
	writer http.ResponseWriter,
	statusCode int,
	problemType string,
	title string,
) {
	writer.Header().Set(
		"Content-Type",
		"application/problem+json",
	)
	writer.Header().Set(
		"Cache-Control",
		"no-store",
	)
	writer.WriteHeader(statusCode)

	_ = json.NewEncoder(writer).Encode(
		map[string]any{
			"type":   problemType,
			"title":  title,
			"status": statusCode,
		},
	)
}
