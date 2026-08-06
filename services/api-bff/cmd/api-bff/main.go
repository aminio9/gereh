package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/service"
	bffconfig "github.com/aminio9/gereh/services/api-bff/internal/config"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	organizationhttp "github.com/aminio9/gereh/services/api-bff/internal/http/organization"
	tenanthttp "github.com/aminio9/gereh/services/api-bff/internal/http/tenant"
	workhttp "github.com/aminio9/gereh/services/api-bff/internal/http/work"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	authConfig, err := bffconfig.AuthConfigFromEnv()
	if err != nil {
		slog.Error(
			"load authentication configuration",
			"error",
			err,
		)
		os.Exit(1)
	}

	tenantConfig, err := bffconfig.TenantConfigFromEnv()
	if err != nil {
		slog.Error(
			"load Tenant Service configuration",
			"error",
			err,
		)

		os.Exit(1)
	}

	organizationConfig, err :=
		bffconfig.OrganizationConfigFromEnv()
	if err != nil {
		slog.Error(
			"load Organization Service configuration",
			"error",
			err,
		)

		os.Exit(1)
	}

	workConfig, err := bffconfig.WorkConfigFromEnv()
	if err != nil {
		slog.Error(
			"load Work Management Service configuration",
			"error",
			err,
		)

		os.Exit(1)
	}

	clientConfig := grpcx.DefaultClientConfig(
		authConfig.IdentityTarget,
	)
	clientConfig.Insecure = authConfig.IdentityInsecure

	connection, err := grpcx.NewClient(
		clientConfig,
		nil,
	)
	if err != nil {
		slog.Error(
			"create identity gRPC client",
			"error",
			err,
		)
		os.Exit(1)
	}

	identityClient := identityv1.NewIdentityServiceClient(
		connection,
	)

	authHandler := authhttp.NewHandler(
		authConfig,
		identityClient,
		slog.Default(),
	)

	tenantClientConfig := grpcx.DefaultClientConfig(
		tenantConfig.Target,
	)

	tenantClientConfig.Insecure =
		tenantConfig.Insecure

	tenantConnection, err := grpcx.NewClient(
		tenantClientConfig,
		nil,
	)
	if err != nil {
		slog.Error(
			"create Tenant Service gRPC client",
			"error",
			err,
		)

		os.Exit(1)
	}

	defer func() {
		if err := tenantConnection.Close(); err != nil {
			slog.Warn(
				"close Tenant Service gRPC client",
				"error",
				err,
			)
		}
	}()

	defer func() {
		if err := connection.Close(); err != nil {
			slog.Warn(
				"close identity gRPC client",
				"error",
				err,
			)
		}
	}()

	tenantClient := tenantv1.NewTenantServiceClient(
		tenantConnection,
	)

	tenantHandler := tenanthttp.New(
		tenantClient,
		slog.Default(),
	)

	organizationClientConfig :=
		grpcx.DefaultClientConfig(
			organizationConfig.Target,
		)

	organizationClientConfig.Insecure =
		organizationConfig.Insecure

	organizationConnection, err := grpcx.NewClient(
		organizationClientConfig,
		nil,
	)
	if err != nil {
		slog.Error(
			"create Organization Service gRPC client",
			"error",
			err,
		)

		os.Exit(1)
	}

	defer func() {
		if err := organizationConnection.Close(); err != nil {
			slog.Warn(
				"close Organization Service gRPC client",
				"error",
				err,
			)
		}
	}()

	organizationClient :=
		organizationv1.NewOrganizationServiceClient(
			organizationConnection,
		)

	organizationHandler := organizationhttp.New(
		organizationClient,
		tenantClient,
		slog.Default(),
	)

	workClientConfig := grpcx.DefaultClientConfig(
		workConfig.Target,
	)

	workClientConfig.Insecure =
		workConfig.Insecure

	workConnection, err := grpcx.NewClient(
		workClientConfig,
		nil,
	)
	if err != nil {
		slog.Error(
			"create Work Management Service gRPC client",
			"error",
			err,
		)

		os.Exit(1)
	}

	defer func() {
		if err := workConnection.Close(); err != nil {
			slog.Warn(
				"close Work Management Service gRPC client",
				"error",
				err,
			)
		}
	}()

	workClient := workv1.NewWorkManagementServiceClient(
		workConnection,
	)

	workHandler := workhttp.New(
		workClient,
		tenantClient,
		slog.Default(),
	)

	service.Run(
		service.Config{
			Name:           "api-bff",
			Version:        envOrDefault("SERVICE_VERSION", "dev"),
			DefaultAddress: ":8080",
		},
		func(router chi.Router) {
			router.Use(
				chimiddleware.Timeout(
					15 * time.Second,
				),
			)

			authHandler.Register(router)
			tenantHandler.Register(router, authHandler)
			organizationHandler.Register(
				router,
				authHandler,
			)
			workHandler.Register(
				router,
				authHandler,
			)

			router.Get(
				"/api/system/status",
				func(
					writer http.ResponseWriter,
					_ *http.Request,
				) {
					writer.Header().Set(
						"Content-Type",
						"application/json",
					)

					_ = json.NewEncoder(writer).Encode(
						map[string]string{
							"status":  "ok",
							"service": "api-bff",
						},
					)
				},
			)
		},
	)
}

func envOrDefault(
	name string,
	fallback string,
) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
