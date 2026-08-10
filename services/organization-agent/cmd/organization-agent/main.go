// Package main runs the Gereh Company and Agent Service.
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

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/organization-agent/internal/adapters/authorization"
	"github.com/aminio9/gereh/services/organization-agent/internal/adapters/outbox"
	organizationpostgres "github.com/aminio9/gereh/services/organization-agent/internal/adapters/postgres"
	"github.com/aminio9/gereh/services/organization-agent/internal/application"
	"github.com/aminio9/gereh/services/organization-agent/internal/config"
	organizationgrpc "github.com/aminio9/gereh/services/organization-agent/internal/transport/grpc"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error(
			"organization-agent service stopped with an error",
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

	telemetryConfig, err :=
		observability.ConfigFromEnv(
			runtimeConfig.ServiceName,
			runtimeConfig.ServiceVersion,
		)
	if err != nil {
		return err
	}

	telemetry, err := observability.Setup(
		ctx,
		telemetryConfig,
	)
	if err != nil {
		return err
	}

	defer func() {
		shutdownContext, cancel :=
			context.WithTimeout(
				context.Background(),
				telemetryConfig.ShutdownTimeout,
			)
		defer cancel()

		if err := telemetry.Shutdown(
			shutdownContext,
		); err != nil {
			slog.Warn(
				"telemetry shutdown failed",
				"error",
				err,
			)
		}
	}()

	logger := slog.New(
		observability.NewTraceHandler(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelInfo,
				},
			),
		),
	)

	slog.SetDefault(logger)

	database, err := platformpostgres.Open(
		ctx,
		platformpostgres.Config{
			URL: runtimeConfig.DatabaseURL,

			ApplicationName: runtimeConfig.ServiceName,

			MaxConnections: runtimeConfig.PostgresMaxConnections,

			MinConnections: runtimeConfig.PostgresMinConnections,

			MaxConnectionLifetime: 30 * time.Minute,

			MaxConnectionIdleTime: 5 * time.Minute,

			HealthCheckPeriod: 30 * time.Second,

			StatementTimeout: 15 * time.Second,

			LockTimeout: 3 * time.Second,

			IdleInTransactionTimeout: 15 * time.Second,

			OwnedTableSchemas: []string{"public"},
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

	tenantConnection, err := grpcx.NewClient(
		grpcx.ClientConfig{
			Target:   runtimeConfig.TenantGRPCTarget,
			Insecure: runtimeConfig.TenantGRPCInsecure,
		},
		telemetry,
	)
	if err != nil {
		return err
	}

	defer func() {
		if err := tenantConnection.Close(); err != nil {
			logger.Warn(
				"tenant gRPC connection close failed",
				"error",
				err,
			)
		}
	}()

	tenantClient := tenantv1.NewTenantServiceClient(
		tenantConnection,
	)

	repository := organizationpostgres.New(
		database.Pool(),
	)

	organizationService, err := application.New(
		repository,
		authorization.NewTenantAuthorizer(
			tenantClient,
			runtimeConfig.AuthorizerTimeout,
		),
		application.Config{
			CompanyEventTopic: runtimeConfig.CompanyEventTopic,
			AgentEventTopic:   runtimeConfig.AgentEventTopic,
			BootstrapServicePrincipalID: runtimeConfig.
				BootstrapServicePrincipalID,
		},
	)
	if err != nil {
		return err
	}

	relay, err := outbox.New(
		outbox.Config{
			BatchSize: runtimeConfig.OutboxBatchSize,
			PollInterval: runtimeConfig.
				OutboxPollInterval,
			Lease: runtimeConfig.OutboxLease,
			MaxBackoff: runtimeConfig.
				OutboxMaxBackoff,
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

	if strings.EqualFold(
		runtimeConfig.Environment,
		"production",
	) {
		tlsConfig, err := grpcx.LoadWorkloadServerTLS(
			grpcx.WorkloadTLSFiles{
				CertificateFile: runtimeConfig.GRPCTLSCertFile,
				PrivateKeyFile:  runtimeConfig.GRPCTLSKeyFile,
				CAFile:          runtimeConfig.GRPCTLSCAFile,
			},
		)
		if err != nil {
			return fmt.Errorf(
				"configure Organization Service workload TLS: %w",
				err,
			)
		}

		serverConfig.TLSConfig = tlsConfig
	}

	serverConfig.UnaryInterceptors = append(
		serverConfig.UnaryInterceptors,
		organizationgrpc.InternalWorkloadUnaryInterceptor(
			organizationgrpc.InternalAuthConfig{
				Environment: runtimeConfig.Environment,
				DevelopmentToken: runtimeConfig.
					InternalDevelopmentToken,
				AllowedSPIFFEIDs: organizationgrpc.ParseAllowedSPIFFEIDs(
					strings.Join(
						runtimeConfig.AllowedInternalSPIFFEIDs,
						",",
					),
				),
			},
		),
		organizationgrpc.ActorBindingUnaryInterceptor(),
	)

	server, err := grpcx.NewServer(
		serverConfig,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	organizationv1.RegisterOrganizationServiceServer(
		server.GRPC(),
		organizationgrpc.New(organizationService),
	)

	organizationv1.RegisterOrganizationBootstrapServiceServer(
		server.GRPC(),
		organizationgrpc.NewBootstrap(organizationService),
	)

	organizationv1.RegisterOrganizationPolicyContextServiceServer(
		server.GRPC(),
		organizationgrpc.NewPolicyContext(organizationService),
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
			"organization-agent service listening",
			"address",
			runtimeConfig.GRPCAddress,
		)

		serverErrors <- server.Serve(listener)
	}()

	select {
	case serverErr := <-serverErrors:
		return serverErr

	case <-ctx.Done():
		logger.Info(
			"organization-agent service shutdown requested",
		)
	}

	shutdownContext, cancel :=
		context.WithTimeout(
			context.Background(),
			runtimeConfig.ShutdownTimeout,
		)
	defer cancel()

	err = server.GracefulStop(shutdownContext)
	if err != nil &&
		!errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
