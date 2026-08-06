// Package main runs the Gereh Work Management Service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/work-management/internal/adapters/authorization"
	"github.com/aminio9/gereh/services/work-management/internal/adapters/organization"
	"github.com/aminio9/gereh/services/work-management/internal/adapters/outbox"
	workpostgres "github.com/aminio9/gereh/services/work-management/internal/adapters/postgres"
	"github.com/aminio9/gereh/services/work-management/internal/application"
	"github.com/aminio9/gereh/services/work-management/internal/config"
	workgrpc "github.com/aminio9/gereh/services/work-management/internal/transport/grpc"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error(
			"work-management service stopped with an error",
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

	organizationConnection, err := grpcx.NewClient(
		grpcx.ClientConfig{
			Target:   runtimeConfig.OrganizationGRPCTarget,
			Insecure: runtimeConfig.OrganizationGRPCInsecure,
		},
		telemetry,
	)
	if err != nil {
		return err
	}

	defer func() {
		if err := organizationConnection.Close(); err != nil {
			logger.Warn(
				"organization gRPC connection close failed",
				"error",
				err,
			)
		}
	}()

	organizationClient :=
		organizationv1.NewOrganizationServiceClient(
			organizationConnection,
		)

	repository := workpostgres.New(
		database.Pool(),
	)

	workService, err := application.New(
		repository,
		authorization.NewTenantAuthorizer(
			tenantClient,
			runtimeConfig.AuthorizerTimeout,
		),
		organization.NewClient(
			organizationClient,
			runtimeConfig.OrganizationClientTimeout,
		),
		application.Config{
			EventTopic: runtimeConfig.EventTopic,
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

	serverConfig.UnaryInterceptors = append(
		serverConfig.UnaryInterceptors,
		workgrpc.ActorBindingUnaryInterceptor(),
	)

	server, err := grpcx.NewServer(
		serverConfig,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	workv1.RegisterWorkManagementServiceServer(
		server.GRPC(),
		workgrpc.New(workService),
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
			"work-management service listening",
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
			"work-management service shutdown requested",
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
