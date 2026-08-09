// Package main runs the Gereh Projection Service.
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

	projectionv1 "github.com/aminio9/gereh/gen/go/gereh/projection/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	projectionauthorization "github.com/aminio9/gereh/services/projection/internal/adapters/authorization"
	projectionpostgres "github.com/aminio9/gereh/services/projection/internal/adapters/postgres"
	"github.com/aminio9/gereh/services/projection/internal/adapters/projection"
	projectionapplication "github.com/aminio9/gereh/services/projection/internal/application"
	"github.com/aminio9/gereh/services/projection/internal/config"
	projectiongrpc "github.com/aminio9/gereh/services/projection/internal/transport/grpc"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error(
			"projection service stopped with an error",
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

	telemetry, err := observability.Setup(
		ctx,
		telemetryConfig,
	)
	if err != nil {
		return err
	}

	defer func() {
		shutdownContext, cancel := context.WithTimeout(
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

	consumer, err := platformkafka.NewConsumer(
		runtimeConfig.Kafka,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	defer consumer.Close()

	repository := projectionpostgres.New(
		database.Pool(),
	)

	authorizer := projectionauthorization.NewTenantAuthorizer(
		tenantClient,
		runtimeConfig.AuthorizerTimeout,
	)

	projectionService, err := projectionapplication.New(
		repository,
		authorizer,
		projectionapplication.Config{
			ServicePrincipalID: runtimeConfig.ServicePrincipalID,
		},
	)
	if err != nil {
		return err
	}

	worker, err := projection.NewWorker(
		consumer,
		projectionService,
		logger,
		telemetry.Meter("gereh.projection"),
	)
	if err != nil {
		return err
	}

	go func() {
		logger.Info(
			"projection worker starting",
		)

		if err := worker.Run(ctx); err != nil {
			logger.Error(
				"projection worker stopped with an error",
				"error",
				err,
			)
		}
	}()

	serverConfig := grpcx.DefaultServerConfig()

	server, err := grpcx.NewServer(
		serverConfig,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	projectionv1.RegisterProjectionServiceServer(
		server.GRPC(),
		projectiongrpc.NewServer(projectionService),
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

	// Readiness gating: stay NOT_SERVING until the Kafka consumer
	// receives its first partition assignment, then flip to SERVING.
	// The shared grpcx health server starts NOT_SERVING, so we serve the
	// raw gRPC listener and control the health status manually.
	go func() {
		select {
		case <-consumer.Ready():
			logger.Info(
				"projection consumer ready; enabling health",
			)

			server.SetServing("")

		case <-ctx.Done():
		}
	}()

	go func() {
		logger.Info(
			"projection service listening",
			"address",
			runtimeConfig.GRPCAddress,
		)

		serverErrors <- server.GRPC().Serve(listener)
	}()

	select {
	case serverErr := <-serverErrors:
		return serverErr

	case <-ctx.Done():
		logger.Info(
			"projection service shutdown requested",
		)
	}

	shutdownContext, cancel := context.WithTimeout(
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
