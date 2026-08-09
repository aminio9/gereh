// Package projection exposes Projection Service read models through the BFF.
package projection

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	projectionv1 "github.com/aminio9/gereh/gen/go/gereh/projection/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	"github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Client is implemented by the generated Projection Service client.
type Client interface {
	GetDashboardSummary(
		context.Context,
		*projectionv1.GetDashboardSummaryRequest,
		...grpc.CallOption,
	) (*projectionv1.GetDashboardSummaryResponse, error)

	GetCompanyOverview(
		context.Context,
		*projectionv1.GetCompanyOverviewRequest,
		...grpc.CallOption,
	) (*projectionv1.GetCompanyOverviewResponse, error)

	ListAgentOverviews(
		context.Context,
		*projectionv1.ListAgentOverviewsRequest,
		...grpc.CallOption,
	) (*projectionv1.ListAgentOverviewsResponse, error)

	ListTaskActivity(
		context.Context,
		*projectionv1.ListTaskActivityRequest,
		...grpc.CallOption,
	) (*projectionv1.ListTaskActivityResponse, error)

	Search(
		context.Context,
		*projectionv1.SearchRequest,
		...grpc.CallOption,
	) (*projectionv1.SearchResponse, error)
}

// PermissionChecker validates one tenant permission on the BFF.
//
// The Projection Service performs the authoritative authorization check on
// every query; this middleware fails fast with 403 before the downstream call.
type PermissionChecker interface {
	CheckAuthorization(
		context.Context,
		*tenantv1.CheckAuthorizationRequest,
		...grpc.CallOption,
	) (*tenantv1.CheckAuthorizationResponse, error)
}

// Handler exposes Projection Service read models through the BFF.
type Handler struct {
	client     Client
	authorizer PermissionChecker
	logger     *slog.Logger
}

// New creates Projection Service HTTP handlers.
func New(
	client Client,
	authorizer PermissionChecker,
	logger *slog.Logger,
) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		client:     client,
		authorizer: authorizer,
		logger:     logger,
	}
}

// Register registers authenticated and authorized projection routes.
func (handler *Handler) Register(
	router chi.Router,
	authHandler *auth.Handler,
) {
	router.Route(
		"/v1/tenants/{tenantID}",
		func(tenantRouter chi.Router) {
			tenantRouter.Use(
				authHandler.RequireSession,
			)

			tenantRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TENANT_READ,
				),
			).Get(
				"/dashboard",
				handler.dashboard,
			)

			tenantRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TENANT_READ,
				),
			).Get(
				"/agents/overview",
				handler.listAgentOverviews,
			)

			tenantRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TENANT_READ,
				),
			).Get(
				"/activity",
				handler.listTaskActivity,
			)

			tenantRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TENANT_READ,
				),
			).Get(
				"/search",
				handler.search,
			)

			tenantRouter.Route(
				"/companies/{companyID}",
				func(companyRouter chi.Router) {
					companyRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_TENANT_READ,
						),
					).Get(
						"/overview",
						handler.companyOverview,
					)

					companyRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_TENANT_READ,
						),
					).Get(
						"/agents/overview",
						handler.listAgentOverviews,
					)
				},
			)
		},
	)
}

// RequirePermission validates one tenant permission and stores trusted
// authorization context for downstream handlers.
func (handler *Handler) RequirePermission(
	permission tenantv1.Permission,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				principal, ok :=
					platformauth.PrincipalFromContext(
						request.Context(),
					)
				if !ok {
					writeProblem(
						writer,
						http.StatusUnauthorized,
						"unauthenticated",
						"Authentication is required",
					)
					return
				}

				response, err :=
					handler.authorizer.CheckAuthorization(
						request.Context(),
						&tenantv1.CheckAuthorizationRequest{
							ActorUserId: principal.UserID,
							TenantId:    tenantID(request),
							Permission:  permission,
						},
					)
				if err != nil {
					handler.writeGRPCError(
						writer,
						err,
					)
					return
				}

				decision := response.GetDecision()

				if decision != nil &&
					decision.GetAllowed() {
					next.ServeHTTP(
						writer,
						request,
					)
					return
				}

				writeProblem(
					writer,
					http.StatusForbidden,
					"forbidden",
					"Projection operation forbidden",
				)
			},
		)
	}
}

func (handler *Handler) dashboard(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok :=
		platformauth.PrincipalFromContext(
			request.Context(),
		)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	response, err := handler.client.GetDashboardSummary(
		request.Context(),
		&projectionv1.GetDashboardSummaryRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) companyOverview(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok :=
		platformauth.PrincipalFromContext(
			request.Context(),
		)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	response, err := handler.client.GetCompanyOverview(
		request.Context(),
		&projectionv1.GetCompanyOverviewRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId:   chi.URLParam(request, "companyID"),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listAgentOverviews(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok :=
		platformauth.PrincipalFromContext(
			request.Context(),
		)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	pageSize, pageToken := pagination(request)

	response, err := handler.client.ListAgentOverviews(
		request.Context(),
		&projectionv1.ListAgentOverviewsRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId:   optionalParam(request, "companyID"),
			PageSize:    pageSize,
			PageToken:   pageToken,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listTaskActivity(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok :=
		platformauth.PrincipalFromContext(
			request.Context(),
		)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	pageSize, pageToken := pagination(request)

	response, err := handler.client.ListTaskActivity(
		request.Context(),
		&projectionv1.ListTaskActivityRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId:   optionalParam(request, "companyID"),
			PageSize:    pageSize,
			PageToken:   pageToken,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) search(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok :=
		platformauth.PrincipalFromContext(
			request.Context(),
		)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	pageSize, pageToken := pagination(request)

	response, err := handler.client.Search(
		request.Context(),
		&projectionv1.SearchRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			Query:       request.URL.Query().Get("q"),
			CompanyId:   optionalParam(request, "companyID"),
			PageSize:    pageSize,
			PageToken:   pageToken,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
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
			"Invalid projection request",
		)

	case codes.NotFound:
		writeProblem(
			writer,
			http.StatusNotFound,
			"not_found",
			"Projection resource not found",
		)

	case codes.PermissionDenied:
		writeProblem(
			writer,
			http.StatusForbidden,
			"forbidden",
			"Projection operation forbidden",
		)

	case codes.Unavailable, codes.DeadlineExceeded:
		writeProblem(
			writer,
			http.StatusServiceUnavailable,
			"projection_unavailable",
			"Projection Service is unavailable",
		)

	default:
		handler.logger.ErrorContext(
			context.Background(),
			"Projection Service request failed",
			"error",
			err,
		)

		writeProblem(
			writer,
			http.StatusInternalServerError,
			"projection_error",
			"Projection operation failed",
		)
	}
}

// tenantID always comes from the route, never from the request body.
func tenantID(request *http.Request) string {
	return chi.URLParam(request, "tenantID")
}

func optionalParam(
	request *http.Request,
	name string,
) *string {
	value := chi.URLParam(request, name)
	if value == "" {
		return nil
	}

	return &value
}

func pagination(
	request *http.Request,
) (int32, string) {
	raw := request.URL.Query().Get("page_size")

	pageSize, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || pageSize <= 0 {
		pageSize = 25
	}

	if pageSize > 50 {
		pageSize = 50
	}

	return int32(pageSize),
		request.URL.Query().Get("page_token")
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
