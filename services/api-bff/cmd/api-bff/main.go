package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/service"
	bffconfig "github.com/aminio9/gereh/services/api-bff/internal/config"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	tenanthttp "github.com/aminio9/gereh/services/api-bff/internal/http/tenant"
	"github.com/go-chi/chi/v5"
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

	service.Run(
		service.Config{
			Name:           "api-bff",
			Version:        envOrDefault("SERVICE_VERSION", "dev"),
			DefaultAddress: ":8080",
		},
		func(router chi.Router) {
			authHandler.Register(router)
			tenantHandler.Register(router, authHandler)

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
