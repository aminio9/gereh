// Package organization exposes browser-facing Company and Agent Service
// HTTP endpoints.
package organization

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
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

// Client is implemented by the generated Organization Service client.
type Client interface {
	CreateCompany(
		context.Context,
		*organizationv1.CreateCompanyRequest,
		...grpc.CallOption,
	) (*organizationv1.CreateCompanyResponse, error)

	GetCompany(
		context.Context,
		*organizationv1.GetCompanyRequest,
		...grpc.CallOption,
	) (*organizationv1.GetCompanyResponse, error)

	ListCompanies(
		context.Context,
		*organizationv1.ListCompaniesRequest,
		...grpc.CallOption,
	) (*organizationv1.ListCompaniesResponse, error)

	UpdateCompany(
		context.Context,
		*organizationv1.UpdateCompanyRequest,
		...grpc.CallOption,
	) (*organizationv1.UpdateCompanyResponse, error)

	ArchiveCompany(
		context.Context,
		*organizationv1.ArchiveCompanyRequest,
		...grpc.CallOption,
	) (*organizationv1.ArchiveCompanyResponse, error)

	CreateAgent(
		context.Context,
		*organizationv1.CreateAgentRequest,
		...grpc.CallOption,
	) (*organizationv1.CreateAgentResponse, error)

	GetAgent(
		context.Context,
		*organizationv1.GetAgentRequest,
		...grpc.CallOption,
	) (*organizationv1.GetAgentResponse, error)

	ListAgents(
		context.Context,
		*organizationv1.ListAgentsRequest,
		...grpc.CallOption,
	) (*organizationv1.ListAgentsResponse, error)

	UpdateAgent(
		context.Context,
		*organizationv1.UpdateAgentRequest,
		...grpc.CallOption,
	) (*organizationv1.UpdateAgentResponse, error)

	SetAgentManager(
		context.Context,
		*organizationv1.SetAgentManagerRequest,
		...grpc.CallOption,
	) (*organizationv1.SetAgentManagerResponse, error)

	PauseAgent(
		context.Context,
		*organizationv1.PauseAgentRequest,
		...grpc.CallOption,
	) (*organizationv1.PauseAgentResponse, error)

	ResumeAgent(
		context.Context,
		*organizationv1.ResumeAgentRequest,
		...grpc.CallOption,
	) (*organizationv1.ResumeAgentResponse, error)

	DeleteAgent(
		context.Context,
		*organizationv1.DeleteAgentRequest,
		...grpc.CallOption,
	) (*organizationv1.DeleteAgentResponse, error)

	GetAgentHierarchy(
		context.Context,
		*organizationv1.GetAgentHierarchyRequest,
		...grpc.CallOption,
	) (*organizationv1.GetAgentHierarchyResponse, error)
}

// PermissionChecker validates one tenant permission on the BFF.
//
// The Organization Service performs the authoritative check; this middleware
// fails fast with 403 before the downstream call.
type PermissionChecker interface {
	CheckAuthorization(
		context.Context,
		*tenantv1.CheckAuthorizationRequest,
		...grpc.CallOption,
	) (*tenantv1.CheckAuthorizationResponse, error)
}

// Handler exposes Organization Service operations through the BFF.
type Handler struct {
	client     Client
	authorizer PermissionChecker
	logger     *slog.Logger
}

// New creates Organization Service HTTP handlers.
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

// Register registers authenticated and authorized organization routes.
func (handler *Handler) Register(
	router chi.Router,
	authHandler *authhttp.Handler,
) {
	router.Route(
		"/v1/tenants/{tenantID}",
		func(tenantRouter chi.Router) {
			tenantRouter.Use(
				authHandler.RequireSession,
			)

			tenantRouter.Route(
				"/companies",
				func(companyRouter chi.Router) {
					companyRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_COMPANY_CREATE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/",
						handler.createCompany,
					)

					companyRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_COMPANY_READ,
						),
					).Get(
						"/",
						handler.listCompanies,
					)

					companyRouter.Route(
						"/{companyID}",
						func(resource chi.Router) {
							resource.With(
								handler.RequirePermission(
									tenantv1.Permission_PERMISSION_COMPANY_READ,
								),
							).Get(
								"/",
								handler.getCompany,
							)

							resource.With(
								handler.RequirePermission(
									tenantv1.Permission_PERMISSION_COMPANY_UPDATE,
								),
								authHandler.RequireCSRF,
							).Patch(
								"/",
								handler.updateCompany,
							)

							resource.With(
								handler.RequirePermission(
									tenantv1.Permission_PERMISSION_COMPANY_ARCHIVE,
								),
								authHandler.RequireCSRF,
							).Post(
								"/archive",
								handler.archiveCompany,
							)

							resource.With(
								handler.RequirePermission(
									tenantv1.Permission_PERMISSION_AGENT_CREATE,
								),
								authHandler.RequireCSRF,
							).Post(
								"/agents",
								handler.createAgent,
							)

							resource.With(
								handler.RequirePermission(
									tenantv1.Permission_PERMISSION_AGENT_READ,
								),
							).Get(
								"/agents",
								handler.listAgents,
							)

							resource.With(
								handler.RequirePermission(
									tenantv1.Permission_PERMISSION_AGENT_READ,
								),
							).Get(
								"/hierarchy",
								handler.getAgentHierarchy,
							)
						},
					)
				},
			)

			tenantRouter.Route(
				"/agents",
				func(agentRouter chi.Router) {
					agentRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_AGENT_READ,
						),
					).Get(
						"/{agentID}",
						handler.getAgent,
					)

					agentRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_AGENT_UPDATE,
						),
						authHandler.RequireCSRF,
					).Patch(
						"/{agentID}",
						handler.updateAgent,
					)

					agentRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_AGENT_HIERARCHY_MANAGE,
						),
						authHandler.RequireCSRF,
					).Put(
						"/{agentID}/manager",
						handler.setAgentManager,
					)

					agentRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_AGENT_LIFECYCLE_MANAGE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/{agentID}/pause",
						handler.pauseAgent,
					)

					agentRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_AGENT_LIFECYCLE_MANAGE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/{agentID}/resume",
						handler.resumeAgent,
					)

					agentRouter.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_AGENT_DELETE,
						),
						authHandler.RequireCSRF,
					).Delete(
						"/{agentID}",
						handler.deleteAgent,
					)
				},
			)
		},
	)
}

type createCompanyRequest struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

type updateCompanyRequest struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	DisplayName     *string `json:"displayName"`
	Description     *string `json:"description"`
}

type archiveCompanyRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

type createAgentRequest struct {
	Slug             string                               `json:"slug"`
	DisplayName      string                               `json:"displayName"`
	RoleTitle        string                               `json:"roleTitle"`
	Objective        string                               `json:"objective"`
	ManagerAgentID   *string                              `json:"managerAgentId"`
	ExecutionProfile organizationv1.AgentExecutionProfile `json:"executionProfile"`
	AutonomyLevel    organizationv1.AgentAutonomyLevel    `json:"autonomyLevel"`
	Capabilities     []string                             `json:"capabilities"`
	Configuration    map[string]any                       `json:"configuration"`
}

type updateAgentRequest struct {
	ExpectedVersion  int64                                 `json:"expectedVersion"`
	DisplayName      *string                               `json:"displayName"`
	RoleTitle        *string                               `json:"roleTitle"`
	Objective        *string                               `json:"objective"`
	ExecutionProfile *organizationv1.AgentExecutionProfile `json:"executionProfile"`
	AutonomyLevel    *organizationv1.AgentAutonomyLevel    `json:"autonomyLevel"`
	Capabilities     *[]string                             `json:"capabilities"`
	Configuration    map[string]any                        `json:"configuration"`
}

type setManagerRequest struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	ManagerAgentID  *string `json:"managerAgentId"`
}

type lifecycleRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

func (handler *Handler) createCompany(
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

	var input createCompanyRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid company request",
		)
		return
	}

	response, err := handler.client.CreateCompany(
		request.Context(),
		&organizationv1.CreateCompanyRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			Slug:        input.Slug,
			DisplayName: input.DisplayName,
			Description: input.Description,
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

func (handler *Handler) getCompany(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetCompany(
		request.Context(),
		&organizationv1.GetCompanyRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId: chi.URLParam(
				request,
				"companyID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listCompanies(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListCompanies(
		request.Context(),
		&organizationv1.ListCompaniesRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			PageSize:    int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
			IncludeArchived: request.URL.Query().
				Get("include_archived") == "true",
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) updateCompany(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateCompanyRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid company request",
		)
		return
	}

	response, err := handler.client.UpdateCompany(
		request.Context(),
		&organizationv1.UpdateCompanyRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			CompanyId:       chi.URLParam(request, "companyID"),
			ExpectedVersion: input.ExpectedVersion,
			DisplayName:     input.DisplayName,
			Description:     input.Description,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) archiveCompany(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input archiveCompanyRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid company request",
		)
		return
	}

	response, err := handler.client.ArchiveCompany(
		request.Context(),
		&organizationv1.ArchiveCompanyRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			CompanyId:       chi.URLParam(request, "companyID"),
			ExpectedVersion: input.ExpectedVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) createAgent(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input createAgentRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid agent request",
		)
		return
	}

	response, err := handler.client.CreateAgent(
		request.Context(),
		&organizationv1.CreateAgentRequest{
			ActorUserId:      principal.UserID,
			TenantId:         tenantID(request),
			CompanyId:        chi.URLParam(request, "companyID"),
			Slug:             input.Slug,
			DisplayName:      input.DisplayName,
			RoleTitle:        input.RoleTitle,
			Objective:        input.Objective,
			ManagerAgentId:   input.ManagerAgentID,
			ExecutionProfile: input.ExecutionProfile,
			AutonomyLevel:    input.AutonomyLevel,
			Capabilities:     input.Capabilities,
			Configuration:    configurationStruct(input.Configuration),
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

func (handler *Handler) getAgent(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetAgent(
		request.Context(),
		&organizationv1.GetAgentRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			AgentId: chi.URLParam(
				request,
				"agentID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listAgents(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListAgents(
		request.Context(),
		&organizationv1.ListAgentsRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId: chi.URLParam(
				request,
				"companyID",
			),
			PageSize: int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
			IncludeDeleted: request.URL.Query().
				Get("include_deleted") == "true",
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) updateAgent(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateAgentRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid agent request",
		)
		return
	}

	agentRequest := &organizationv1.UpdateAgentRequest{
		ActorUserId:     principal.UserID,
		TenantId:        tenantID(request),
		AgentId:         chi.URLParam(request, "agentID"),
		ExpectedVersion: input.ExpectedVersion,
		DisplayName:     input.DisplayName,
		RoleTitle:       input.RoleTitle,
		Objective:       input.Objective,
	}

	if input.ExecutionProfile != nil {
		agentRequest.ExecutionProfile =
			input.ExecutionProfile
	}

	if input.AutonomyLevel != nil {
		agentRequest.AutonomyLevel =
			input.AutonomyLevel
	}

	if input.Capabilities != nil {
		agentRequest.Capabilities =
			&organizationv1.CapabilitySet{
				Values: *input.Capabilities,
			}
	}

	if input.Configuration != nil {
		agentRequest.Configuration =
			configurationStruct(input.Configuration)
	}

	response, err := handler.client.UpdateAgent(
		request.Context(),
		agentRequest,
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) setAgentManager(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input setManagerRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid manager request",
		)
		return
	}

	response, err := handler.client.SetAgentManager(
		request.Context(),
		&organizationv1.SetAgentManagerRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			AgentId:         chi.URLParam(request, "agentID"),
			ExpectedVersion: input.ExpectedVersion,
			ManagerAgentId:  input.ManagerAgentID,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) pauseAgent(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input lifecycleRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid agent request",
		)
		return
	}

	response, err := handler.client.PauseAgent(
		request.Context(),
		&organizationv1.PauseAgentRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			AgentId:         chi.URLParam(request, "agentID"),
			ExpectedVersion: input.ExpectedVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) resumeAgent(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input lifecycleRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid agent request",
		)
		return
	}

	response, err := handler.client.ResumeAgent(
		request.Context(),
		&organizationv1.ResumeAgentRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			AgentId:         chi.URLParam(request, "agentID"),
			ExpectedVersion: input.ExpectedVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) deleteAgent(
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

	response, err := handler.client.DeleteAgent(
		request.Context(),
		&organizationv1.DeleteAgentRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			AgentId:         chi.URLParam(request, "agentID"),
			ExpectedVersion: version,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) getAgentHierarchy(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetAgentHierarchy(
		request.Context(),
		&organizationv1.GetAgentHierarchyRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId: chi.URLParam(
				request,
				"companyID",
			),
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

// tenantID always comes from the route, never from the request body.
func tenantID(request *http.Request) string {
	return chi.URLParam(request, "tenantID")
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
			"Invalid organization request",
		)

	case codes.NotFound:
		writeProblem(
			writer,
			http.StatusNotFound,
			"not_found",
			"Organization resource not found",
		)

	case codes.PermissionDenied:
		writeProblem(
			writer,
			http.StatusForbidden,
			"forbidden",
			"Organization operation forbidden",
		)

	case codes.AlreadyExists:
		writeProblem(
			writer,
			http.StatusConflict,
			"conflict",
			"Organization resource already exists",
		)

	case codes.Aborted:
		writeProblem(
			writer,
			http.StatusConflict,
			"version_conflict",
			"Organization resource changed; reload and retry",
		)

	case codes.FailedPrecondition:
		writeProblem(
			writer,
			http.StatusConflict,
			"failed_precondition",
			"Organization operation conflicts with the current state",
		)

	case codes.Unavailable, codes.DeadlineExceeded:
		writeProblem(
			writer,
			http.StatusServiceUnavailable,
			"organization_unavailable",
			"Organization Service is unavailable",
		)

	default:
		handler.logger.ErrorContext(
			context.Background(),
			"Organization Service request failed",
			"error",
			err,
		)

		writeProblem(
			writer,
			http.StatusInternalServerError,
			"organization_error",
			"Organization operation failed",
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
