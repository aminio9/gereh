// Package main runs the Gereh Model Access Service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"

	modelauthorization "github.com/aminio9/gereh/services/model-access/internal/adapters/authorization"
	modelcatalog "github.com/aminio9/gereh/services/model-access/internal/adapters/catalog"
	modelorganization "github.com/aminio9/gereh/services/model-access/internal/adapters/organization"
	"github.com/aminio9/gereh/services/model-access/internal/adapters/outbox"
	modelpostgres "github.com/aminio9/gereh/services/model-access/internal/adapters/postgres"
	modelprovider "github.com/aminio9/gereh/services/model-access/internal/adapters/provider"
	modelsecrets "github.com/aminio9/gereh/services/model-access/internal/adapters/secrets"
	modelapplication "github.com/aminio9/gereh/services/model-access/internal/application"
	"github.com/aminio9/gereh/services/model-access/internal/config"
	modelsecurity "github.com/aminio9/gereh/services/model-access/internal/security"
	modelgrpc "github.com/aminio9/gereh/services/model-access/internal/transport/grpc"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error(
			"model-access service stopped with an error",
			"error",
			err,
		)

		os.Exit(1)
	}
}

func run() error {
	runtimeConfig, err := config.FromEnv(version)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	telemetryConfig, err := observability.ConfigFromEnv(
		runtimeConfig.ServiceName,
		runtimeConfig.ServiceVersion,
	)
	if err != nil {
		return err
	}

	telemetry, err := observability.Setup(ctx, telemetryConfig)
	if err != nil {
		return err
	}

	defer func() {
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			telemetryConfig.ShutdownTimeout,
		)
		defer cancel()

		if err := telemetry.Shutdown(shutdownContext); err != nil {
			slog.Warn("telemetry shutdown failed", "error", err)
		}
	}()

	logger := slog.New(
		observability.NewTraceHandler(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{Level: slog.LevelInfo},
			),
		),
	)

	slog.SetDefault(logger)

	database, err := platformpostgres.Open(
		ctx,
		platformpostgres.Config{
			URL:                      runtimeConfig.DatabaseURL,
			ApplicationName:          runtimeConfig.ServiceName,
			MaxConnections:           runtimeConfig.PostgresMaxConnections,
			MinConnections:           runtimeConfig.PostgresMinConnections,
			MaxConnectionLifetime:    30 * time.Minute,
			MaxConnectionIdleTime:    5 * time.Minute,
			HealthCheckPeriod:        30 * time.Second,
			StatementTimeout:         15 * time.Second,
			LockTimeout:              3 * time.Second,
			IdleInTransactionTimeout: 15 * time.Second,
			OwnedTableSchemas:        []string{"public"},
		},
	)
	if err != nil {
		return err
	}

	defer database.Close()

	producer, err := platformkafka.NewProducer(
		runtimeConfig.Kafka,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}
	defer producer.Close()

	if err := producer.Ping(ctx); err != nil {
		return err
	}

	tenantClientConfig := grpcx.DefaultClientConfig(
		runtimeConfig.TenantGRPCTarget,
	)

	tenantClientConfig.Insecure = runtimeConfig.TenantGRPCInsecure

	if !tenantClientConfig.Insecure {
		tlsConfig, err := grpcx.LoadWorkloadClientTLS(
			grpcx.WorkloadTLSFiles{
				CertificateFile: runtimeConfig.GRPCTLSCertFile,
				PrivateKeyFile:  runtimeConfig.GRPCTLSKeyFile,
				CAFile:          runtimeConfig.GRPCTLSCAFile,
			},
			runtimeConfig.TenantGRPCServerName,
		)
		if err != nil {
			return fmt.Errorf(
				"configure Tenant Service workload TLS: %w",
				err,
			)
		}

		tenantClientConfig.TLSConfig = tlsConfig
	}

	tenantConnection, err := grpcx.NewClient(
		tenantClientConfig,
		telemetry,
	)
	if err != nil {
		return fmt.Errorf(
			"create Tenant Service gRPC client: %w",
			err,
		)
	}

	defer func() {
		if err := tenantConnection.Close(); err != nil {
			logger.Warn("close Tenant Service connection", "error", err)
		}
	}()

	tenantClient := tenantv1.NewTenantServiceClient(tenantConnection)

	repository := modelpostgres.New(database.Pool())

	authorizer := modelauthorization.NewTenantAuthorizer(
		tenantClient,
		runtimeConfig.AuthorizerTimeout,
	)

	fingerprinter, err :=
		modelsecurity.
			NewFingerprinterFromFile(
				runtimeConfig.FingerprintKeyFile,
				runtimeConfig.FingerprintKeyID,
			)
	if err != nil {
		return fmt.Errorf(
			"configure credential fingerprinting: %w",
			err,
		)
	}

	vaultStore, err :=
		modelsecrets.NewVaultStore(
			modelsecrets.VaultConfig{
				Address:           runtimeConfig.VaultAddress,
				Mount:             runtimeConfig.VaultMount,
				Namespace:         runtimeConfig.VaultNamespace,
				TokenFile:         runtimeConfig.VaultTokenFile,
				StaticToken:       runtimeConfig.VaultStaticToken,
				CAFile:            runtimeConfig.VaultCAFile,
				AllowInsecureHTTP: runtimeConfig.VaultAllowInsecureHTTP,
				Timeout:           runtimeConfig.VaultTimeout,
			},
		)
	if err != nil {
		return fmt.Errorf(
			"configure Vault secret store: %w",
			err,
		)
	}

	credentialVerifier, err :=
		modelprovider.NewVerifier(
			runtimeConfig.ProviderVerifyTimeout,
		)
	if err != nil {
		return err
	}

	orgClientConfig := grpcx.DefaultClientConfig(
		runtimeConfig.OrganizationGRPCTarget,
	)
	orgClientConfig.Insecure = runtimeConfig.OrganizationGRPCInsecure
	if !orgClientConfig.Insecure {
		tlsConfig, err := grpcx.LoadWorkloadClientTLS(
			grpcx.WorkloadTLSFiles{
				CertificateFile: runtimeConfig.GRPCTLSCertFile,
				PrivateKeyFile:  runtimeConfig.GRPCTLSKeyFile,
				CAFile:          runtimeConfig.GRPCTLSCAFile,
			},
			runtimeConfig.OrganizationGRPCServerName,
		)
		if err != nil {
			return fmt.Errorf(
				"configure Organization Service workload TLS: %w",
				err,
			)
		}
		orgClientConfig.TLSConfig = tlsConfig
	}

	orgConnection, err := grpcx.NewClient(
		orgClientConfig,
		telemetry,
	)
	if err != nil {
		return fmt.Errorf(
			"create Organization Service gRPC client: %w",
			err,
		)
	}
	defer func() {
		if err := orgConnection.Close(); err != nil {
			logger.Warn("close Organization Service connection", "error", err)
		}
	}()

	orgClient := organizationv1.NewOrganizationServiceClient(orgConnection)
	agentDirectory := modelorganization.NewClient(orgClient, runtimeConfig.AuthorizerTimeout)

	staticCatalogLoader, err := modelcatalog.NewStaticLoader(runtimeConfig.PlatformCatalogPath)
	if err != nil {
		logger.Warn("load platform static catalog", "path", runtimeConfig.PlatformCatalogPath, "error", err)
	}

	catalogClient := modelprovider.NewCatalogClient(runtimeConfig.ProviderVerifyTimeout)

	modelService, err := modelapplication.New(
		repository,
		authorizer,
		vaultStore,
		credentialVerifier,
		fingerprinter,
		modelapplication.Config{
			EventTopic:     runtimeConfig.EventTopic,
			IdempotencyTTL: runtimeConfig.IdempotencyTTL,
		},
		modelapplication.WithCatalogClient(catalogClient),
		modelapplication.WithStaticCatalog(staticCatalogLoader),
		modelapplication.WithAgentDirectory(agentDirectory),
	)
	if err != nil {
		return err
	}

	catalogWorker := modelapplication.NewCatalogWorker(
		modelapplication.CatalogWorkerConfig{
			BatchSize:    runtimeConfig.CatalogWorkerBatchSize,
			PollInterval: runtimeConfig.CatalogWorkerPollInterval,
			Lease:        runtimeConfig.CatalogWorkerLease,
			MaxBackoff:   runtimeConfig.CatalogWorkerMaxBackoff,
		},
		repository,
		modelService,
		logger,
	)
	go catalogWorker.Run(ctx)

	cleanupWorker, err :=
		modelsecrets.NewCleanupWorker(
			modelsecrets.CleanupConfig{
				BatchSize:    runtimeConfig.SecretCleanupBatchSize,
				PollInterval: runtimeConfig.SecretCleanupPollInterval,
				Lease:        runtimeConfig.SecretCleanupLease,
				MaxBackoff:   runtimeConfig.SecretCleanupMaxBackoff,
			},
			repository,
			vaultStore,
			logger,
		)
	if err != nil {
		return err
	}

	go cleanupWorker.Run(ctx)

	relay, err := outbox.New(
		outbox.Config{
			BatchSize:    runtimeConfig.OutboxBatchSize,
			PollInterval: runtimeConfig.OutboxPollInterval,
			Lease:        runtimeConfig.OutboxLease,
			MaxBackoff:   runtimeConfig.OutboxMaxBackoff,
		},
		repository,
		producer,
		logger,
	)
	if err != nil {
		return err
	}

	go relay.Run(ctx)

	serverConfig := grpcx.DefaultServerConfig()

	if strings.EqualFold(runtimeConfig.Environment, "production") {
		tlsConfig, err := grpcx.LoadWorkloadServerTLS(
			grpcx.WorkloadTLSFiles{
				CertificateFile: runtimeConfig.GRPCTLSCertFile,
				PrivateKeyFile:  runtimeConfig.GRPCTLSKeyFile,
				CAFile:          runtimeConfig.GRPCTLSCAFile,
			},
		)
		if err != nil {
			return fmt.Errorf(
				"configure Model Access workload TLS: %w",
				err,
			)
		}

		serverConfig.TLSConfig = tlsConfig
	}

	serverConfig.UnaryInterceptors = append(
		serverConfig.UnaryInterceptors,
		modelgrpc.ActorBindingUnaryInterceptor(),
	)

	server, err := grpcx.NewServer(
		serverConfig,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	modelv1.RegisterModelAccessServiceServer(
		server.GRPC(),
		modelgrpc.New(modelService),
	)

	listenConfig := net.ListenConfig{}

	listener, err := listenConfig.Listen(
		ctx,
		"tcp",
		runtimeConfig.GRPCAddress,
	)
	if err != nil {
		return err
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info(
			"model-access service listening",
			"address",
			runtimeConfig.GRPCAddress,
		)

		serverErrors <- server.Serve(listener)
	}()

	select {
	case serverErr := <-serverErrors:
		return serverErr

	case <-ctx.Done():
		logger.Info("model-access service shutdown requested")
	}

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		runtimeConfig.ShutdownTimeout,
	)
	defer cancel()

	err = server.GracefulStop(shutdownContext)

	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
