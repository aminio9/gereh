package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/service"
	bffconfig "github.com/aminio9/gereh/services/api-bff/internal/config"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	organizationhttp "github.com/aminio9/gereh/services/api-bff/internal/http/organization"
	policyhttp "github.com/aminio9/gereh/services/api-bff/internal/http/policy"
	tenanthttp "github.com/aminio9/gereh/services/api-bff/internal/http/tenant"
	workhttp "github.com/aminio9/gereh/services/api-bff/internal/http/work"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	if err := run(); err != nil {
		slog.Error(
			"start api-bff",
			"error",
			err,
		)

		os.Exit(1)
	}
}

func run() error {
	authConfig, err := bffconfig.AuthConfigFromEnv()
	if err != nil {
		return fmt.Errorf(
			"load authentication configuration: %w",
			err,
		)
	}

	tenantConfig, err := bffconfig.TenantConfigFromEnv()
	if err != nil {
		return fmt.Errorf(
			"load Tenant Service configuration: %w",
			err,
		)
	}

	organizationConfig, err :=
		bffconfig.OrganizationConfigFromEnv()
	if err != nil {
		return fmt.Errorf(
			"load Organization Service configuration: %w",
			err,
		)
	}

	workConfig, err := bffconfig.WorkConfigFromEnv()
	if err != nil {
		return fmt.Errorf(
			"load Work Management Service configuration: %w",
			err,
		)
	}

	policyConfig, err := bffconfig.PolicyConfigFromEnv()
	if err != nil {
		return fmt.Errorf(
			"load Policy Service configuration: %w",
			err,
		)
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
		return fmt.Errorf(
			"create identity gRPC client: %w",
			err,
		)
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
		return fmt.Errorf(
			"create Tenant Service gRPC client: %w",
			err,
		)
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
		return fmt.Errorf(
			"create Organization Service gRPC client: %w",
			err,
		)
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
		return fmt.Errorf(
			"create Work Management Service gRPC client: %w",
			err,
		)
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

	policyClientConfig := grpcx.DefaultClientConfig(
		policyConfig.Target,
	)

	policyClientConfig.Insecure =
		policyConfig.Insecure

	policyConnection, err := grpcx.NewClient(
		policyClientConfig,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"create Policy Service gRPC client: %w",
			err,
		)
	}

	defer func() {
		if err := policyConnection.Close(); err != nil {
			slog.Warn(
				"close Policy Service gRPC client",
				"error",
				err,
			)
		}
	}()

	policyClient :=
		policyv1.NewPolicyManagementServiceClient(
			policyConnection,
		)

	policyHandler := policyhttp.New(
		policyClient,
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
			policyHandler.Register(
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

	return nil
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
