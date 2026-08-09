// Package policy allows the API BFF to act as the Policy Service to
// browser-facing clients.
package policy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
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

// Client is implemented by the generated Policy Management Service client.
type Client interface {
	CreatePolicy(
		context.Context,
		*policyv1.CreatePolicyRequest,
		...grpc.CallOption,
	) (*policyv1.CreatePolicyResponse, error)

	GetPolicy(
		context.Context,
		*policyv1.GetPolicyRequest,
		...grpc.CallOption,
	) (*policyv1.GetPolicyResponse, error)

	ListPolicies(
		context.Context,
		*policyv1.ListPoliciesRequest,
		...grpc.CallOption,
	) (*policyv1.ListPoliciesResponse, error)

	CreatePolicyVersion(
		context.Context,
		*policyv1.CreatePolicyVersionRequest,
		...grpc.CallOption,
	) (*policyv1.CreatePolicyVersionResponse, error)

	ActivatePolicy(
		context.Context,
		*policyv1.ActivatePolicyRequest,
		...grpc.CallOption,
	) (*policyv1.ActivatePolicyResponse, error)

	ArchivePolicy(
		context.Context,
		*policyv1.ArchivePolicyRequest,
		...grpc.CallOption,
	) (*policyv1.ArchivePolicyResponse, error)

	GetDecision(
		context.Context,
		*policyv1.GetDecisionRequest,
		...grpc.CallOption,
	) (*policyv1.GetDecisionResponse, error)

	ListDecisions(
		context.Context,
		*policyv1.ListDecisionsRequest,
		...grpc.CallOption,
	) (*policyv1.ListDecisionsResponse, error)
}

// PermissionChecker validates one tenant permission on the BFF.
type PermissionChecker interface {
	CheckAuthorization(
		context.Context,
		*tenantv1.CheckAuthorizationRequest,
		...grpc.CallOption,
	) (*tenantv1.CheckAuthorizationResponse, error)
}

// Handler exposes Policy Management Service operations through the BFF.
type Handler struct {
	client     Client
	authorizer PermissionChecker
	logger     *slog.Logger
}

// New creates Policy Management Service HTTP handlers.
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

// Register registers authenticated and authorized policy routes.
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

			handler.registerPolicyRoutes(
				tenantRouter,
				authHandler,
			)

			handler.registerDecisionRoutes(
				tenantRouter,
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
			"Invalid policy request",
		)

	case codes.NotFound:
		writeProblem(
			writer,
			http.StatusNotFound,
			"not_found",
			"Policy resource not found",
		)

	case codes.PermissionDenied:
		writeProblem(
			writer,
			http.StatusForbidden,
			"forbidden",
			"Policy operation forbidden",
		)

	case codes.AlreadyExists:
		writeProblem(
			writer,
			http.StatusConflict,
			"conflict",
			"Policy resource already exists",
		)

	case codes.Aborted:
		writeProblem(
			writer,
			http.StatusConflict,
			"version_conflict",
			"Policy resource changed; reload and retry",
		)

	case codes.FailedPrecondition:
		writeProblem(
			writer,
			http.StatusConflict,
			"failed_precondition",
			"Policy operation conflicts with the current state",
		)

	case codes.Unavailable, codes.DeadlineExceeded:
		writeProblem(
			writer,
			http.StatusServiceUnavailable,
			"policy_unavailable",
			"Policy Service is unavailable",
		)

	default:
		handler.logger.ErrorContext(
			context.Background(),
			"Policy Service request failed",
			"error",
			err,
		)

		writeProblem(
			writer,
			http.StatusInternalServerError,
			"policy_error",
			"Policy operation failed",
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
