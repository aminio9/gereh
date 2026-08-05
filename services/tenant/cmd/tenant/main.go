// Package main runs the Gereh Tenant Service.
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

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/aminio9/gereh/services/tenant/internal/adapters/outbox"
	tenantpostgres "github.com/aminio9/gereh/services/tenant/internal/adapters/postgres"
	"github.com/aminio9/gereh/services/tenant/internal/application"
	"github.com/aminio9/gereh/services/tenant/internal/config"
	tenantgrpc "github.com/aminio9/gereh/services/tenant/internal/transport/grpc"
	"github.com/jackc/pgx/v5/pgxpool"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error(
			"tenant service stopped with an error",
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

	poolConfig, err := pgxpool.ParseConfig(
		runtimeConfig.DatabaseURL,
	)
	if err != nil {
		return err
	}

	poolConfig.MaxConns =
		runtimeConfig.PostgresMaxConnections

	poolConfig.MinConns =
		runtimeConfig.PostgresMinConnections

	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second

	database, err := pgxpool.NewWithConfig(
		ctx,
		poolConfig,
	)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := database.Ping(ctx); err != nil {
		return err
	}

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

	repository := tenantpostgres.New(database)

	tenantService, err := application.New(
		repository,
		application.Config{
			EventTopic:     runtimeConfig.EventTopic,
			DefaultRegion:  runtimeConfig.DefaultRegion,
			AllowedRegions: runtimeConfig.AllowedRegions,
			DefaultRetentionDays: runtimeConfig.
				DefaultRetentionDays,
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
		tenantgrpc.ActorBindingUnaryInterceptor(),
	)

	server, err := grpcx.NewServer(
		serverConfig,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	tenantv1.RegisterTenantServiceServer(
		server.GRPC(),
		tenantgrpc.New(tenantService),
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
			"tenant service listening",
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
			"tenant service shutdown requested",
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
