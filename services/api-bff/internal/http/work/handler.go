// Package work exposes browser-facing Work Management Service HTTP
// endpoints.
package work

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Client is implemented by the generated Work Management Service client.
type Client interface {
	CreateGoal(
		context.Context,
		*workv1.CreateGoalRequest,
		...grpc.CallOption,
	) (*workv1.CreateGoalResponse, error)

	GetGoal(
		context.Context,
		*workv1.GetGoalRequest,
		...grpc.CallOption,
	) (*workv1.GetGoalResponse, error)

	ListGoals(
		context.Context,
		*workv1.ListGoalsRequest,
		...grpc.CallOption,
	) (*workv1.ListGoalsResponse, error)

	UpdateGoal(
		context.Context,
		*workv1.UpdateGoalRequest,
		...grpc.CallOption,
	) (*workv1.UpdateGoalResponse, error)

	ChangeGoalStatus(
		context.Context,
		*workv1.ChangeGoalStatusRequest,
		...grpc.CallOption,
	) (*workv1.ChangeGoalStatusResponse, error)

	CreateProject(
		context.Context,
		*workv1.CreateProjectRequest,
		...grpc.CallOption,
	) (*workv1.CreateProjectResponse, error)

	GetProject(
		context.Context,
		*workv1.GetProjectRequest,
		...grpc.CallOption,
	) (*workv1.GetProjectResponse, error)

	ListProjects(
		context.Context,
		*workv1.ListProjectsRequest,
		...grpc.CallOption,
	) (*workv1.ListProjectsResponse, error)

	UpdateProject(
		context.Context,
		*workv1.UpdateProjectRequest,
		...grpc.CallOption,
	) (*workv1.UpdateProjectResponse, error)

	ChangeProjectStatus(
		context.Context,
		*workv1.ChangeProjectStatusRequest,
		...grpc.CallOption,
	) (*workv1.ChangeProjectStatusResponse, error)

	CreateTask(
		context.Context,
		*workv1.CreateTaskRequest,
		...grpc.CallOption,
	) (*workv1.CreateTaskResponse, error)

	GetTask(
		context.Context,
		*workv1.GetTaskRequest,
		...grpc.CallOption,
	) (*workv1.GetTaskResponse, error)

	ListTasks(
		context.Context,
		*workv1.ListTasksRequest,
		...grpc.CallOption,
	) (*workv1.ListTasksResponse, error)

	UpdateTask(
		context.Context,
		*workv1.UpdateTaskRequest,
		...grpc.CallOption,
	) (*workv1.UpdateTaskResponse, error)

	ChangeTaskStatus(
		context.Context,
		*workv1.ChangeTaskStatusRequest,
		...grpc.CallOption,
	) (*workv1.ChangeTaskStatusResponse, error)

	AddTaskDependency(
		context.Context,
		*workv1.AddTaskDependencyRequest,
		...grpc.CallOption,
	) (*workv1.AddTaskDependencyResponse, error)

	RemoveTaskDependency(
		context.Context,
		*workv1.RemoveTaskDependencyRequest,
		...grpc.CallOption,
	) (*workv1.RemoveTaskDependencyResponse, error)

	AssignTask(
		context.Context,
		*workv1.AssignTaskRequest,
		...grpc.CallOption,
	) (*workv1.AssignTaskResponse, error)

	UnassignTask(
		context.Context,
		*workv1.UnassignTaskRequest,
		...grpc.CallOption,
	) (*workv1.UnassignTaskResponse, error)

	AddComment(
		context.Context,
		*workv1.AddCommentRequest,
		...grpc.CallOption,
	) (*workv1.AddCommentResponse, error)

	UpdateComment(
		context.Context,
		*workv1.UpdateCommentRequest,
		...grpc.CallOption,
	) (*workv1.UpdateCommentResponse, error)

	DeleteComment(
		context.Context,
		*workv1.DeleteCommentRequest,
		...grpc.CallOption,
	) (*workv1.DeleteCommentResponse, error)

	AddArtifact(
		context.Context,
		*workv1.AddArtifactRequest,
		...grpc.CallOption,
	) (*workv1.AddArtifactResponse, error)

	DeleteArtifact(
		context.Context,
		*workv1.DeleteArtifactRequest,
		...grpc.CallOption,
	) (*workv1.DeleteArtifactResponse, error)

	AddChecklistItem(
		context.Context,
		*workv1.AddChecklistItemRequest,
		...grpc.CallOption,
	) (*workv1.AddChecklistItemResponse, error)

	UpdateChecklistItem(
		context.Context,
		*workv1.UpdateChecklistItemRequest,
		...grpc.CallOption,
	) (*workv1.UpdateChecklistItemResponse, error)

	DeleteChecklistItem(
		context.Context,
		*workv1.DeleteChecklistItemRequest,
		...grpc.CallOption,
	) (*workv1.DeleteChecklistItemResponse, error)

	UpsertTaskSchedule(
		context.Context,
		*workv1.UpsertTaskScheduleRequest,
		...grpc.CallOption,
	) (*workv1.UpsertTaskScheduleResponse, error)

	DeleteTaskSchedule(
		context.Context,
		*workv1.DeleteTaskScheduleRequest,
		...grpc.CallOption,
	) (*workv1.DeleteTaskScheduleResponse, error)
}

// PermissionChecker validates one tenant permission on the BFF.
//
// The Work Management Service performs the authoritative check; this
// middleware fails fast with 403 before the downstream call.
type PermissionChecker interface {
	CheckAuthorization(
		context.Context,
		*tenantv1.CheckAuthorizationRequest,
		...grpc.CallOption,
	) (*tenantv1.CheckAuthorizationResponse, error)
}

// Handler exposes Work Management Service operations through the BFF.
type Handler struct {
	client     Client
	authorizer PermissionChecker
	logger     *slog.Logger
}

// New creates Work Management Service HTTP handlers.
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

// Register registers authenticated and authorized work routes.
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

			handler.registerGoalRoutes(
				tenantRouter,
				authHandler,
			)

			handler.registerProjectRoutes(
				tenantRouter,
				authHandler,
			)

			handler.registerTaskRoutes(
				tenantRouter,
				authHandler,
			)

			handler.registerCollaborationRoutes(
				tenantRouter,
				authHandler,
			)
		},
	)
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
			"Invalid work request",
		)

	case codes.NotFound:
		writeProblem(
			writer,
			http.StatusNotFound,
			"not_found",
			"Work resource not found",
		)

	case codes.PermissionDenied:
		writeProblem(
			writer,
			http.StatusForbidden,
			"forbidden",
			"Work operation forbidden",
		)

	case codes.AlreadyExists:
		writeProblem(
			writer,
			http.StatusConflict,
			"conflict",
			"Work resource already exists",
		)

	case codes.Aborted:
		writeProblem(
			writer,
			http.StatusConflict,
			"version_conflict",
			"Work resource changed; reload and retry",
		)

	case codes.FailedPrecondition:
		writeProblem(
			writer,
			http.StatusConflict,
			"failed_precondition",
			"Work operation conflicts with the current state",
		)

	case codes.Unavailable, codes.DeadlineExceeded:
		writeProblem(
			writer,
			http.StatusServiceUnavailable,
			"work_unavailable",
			"Work Management Service is unavailable",
		)

	default:
		handler.logger.ErrorContext(
			context.Background(),
			"Work Management Service request failed",
			"error",
			err,
		)

		writeProblem(
			writer,
			http.StatusInternalServerError,
			"work_error",
			"Work operation failed",
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
