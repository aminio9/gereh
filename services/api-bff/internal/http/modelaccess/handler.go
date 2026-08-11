// Package modelaccess exposes Model Access through the browser BFF.
package modelaccess

import (
	"context"
	"log/slog"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
)

// Client is the Model Access gRPC client surface used by the BFF.
type Client interface {
	ListProviders(
		context.Context,
		*modelv1.ListProvidersRequest,
		...grpc.CallOption,
	) (*modelv1.ListProvidersResponse, error)

	CreateConnection(
		context.Context,
		*modelv1.CreateConnectionRequest,
		...grpc.CallOption,
	) (*modelv1.CreateConnectionResponse, error)

	CreateBYOKConnection(
		context.Context,
		*modelv1.CreateBYOKConnectionRequest,
		...grpc.CallOption,
	) (*modelv1.CreateBYOKConnectionResponse, error)

	RotateBYOKCredential(
		context.Context,
		*modelv1.RotateBYOKCredentialRequest,
		...grpc.CallOption,
	) (*modelv1.RotateBYOKCredentialResponse, error)

	GetConnection(
		context.Context,
		*modelv1.GetConnectionRequest,
		...grpc.CallOption,
	) (*modelv1.GetConnectionResponse, error)

	ListConnections(
		context.Context,
		*modelv1.ListConnectionsRequest,
		...grpc.CallOption,
	) (*modelv1.ListConnectionsResponse, error)

	UpdateConnection(
		context.Context,
		*modelv1.UpdateConnectionRequest,
		...grpc.CallOption,
	) (*modelv1.UpdateConnectionResponse, error)

	ArchiveConnection(
		context.Context,
		*modelv1.ArchiveConnectionRequest,
		...grpc.CallOption,
	) (*modelv1.ArchiveConnectionResponse, error)
}

// PermissionChecker checks tenant permissions through Tenant Service.
type PermissionChecker interface {
	CheckAuthorization(
		context.Context,
		*tenantv1.CheckAuthorizationRequest,
		...grpc.CallOption,
	) (*tenantv1.CheckAuthorizationResponse, error)
}

// Handler serves Model Access REST endpoints.
type Handler struct {
	client Client

	authorizer PermissionChecker

	logger *slog.Logger
}

// New constructs the BFF handler.
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

// Register mounts Model Access routes under a tenant-scoped resource.
func (handler *Handler) Register(
	router chi.Router,
	authHandler *authhttp.Handler,
) {
	router.Route(
		"/v1/tenants/{tenantID}",
		func(resource chi.Router) {
			resource.Use(authHandler.RequireSession)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_MODEL_PROVIDER_READ,
				),
			).Get(
				"/model-providers",
				handler.listProviders,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_MODEL_CONNECTION_READ,
				),
			).Get(
				"/model-connections",
				handler.listConnections,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_MODEL_CONNECTION_CREATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/model-connections",
				handler.createConnection,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_MODEL_CONNECTION_READ,
				),
			).Get(
				"/model-connections/{connectionID}",
				handler.getConnection,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_MODEL_CONNECTION_UPDATE,
				),
				authHandler.RequireCSRF,
			).Patch(
				"/model-connections/{connectionID}",
				handler.updateConnection,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_MODEL_CONNECTION_ARCHIVE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/model-connections/{connectionID}/archive",
				handler.archiveConnection,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_MODEL_CONNECTION_UPDATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/model-connections/{connectionID}/credential/rotate",
				handler.rotateBYOKCredential,
			)
		},
	)
}
