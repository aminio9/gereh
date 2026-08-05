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

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	identityoidc "github.com/aminio9/gereh/services/identity-access/internal/adapters/oidc"
	identitypostgres "github.com/aminio9/gereh/services/identity-access/internal/adapters/postgres"
	identityredis "github.com/aminio9/gereh/services/identity-access/internal/adapters/redis"
	"github.com/aminio9/gereh/services/identity-access/internal/application"
	"github.com/aminio9/gereh/services/identity-access/internal/config"
	identitygrpc "github.com/aminio9/gereh/services/identity-access/internal/transport/grpc"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error(
			"identity-access stopped with an error",
			"error",
			err,
		)

		os.Exit(1)
	}
}

func run() (runErr error) {
	runtimeConfig, err := config.FromEnv()
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
	// observability.ConfigureErrorHandler(logger)

	poolConfig, err := pgxpool.ParseConfig(
		runtimeConfig.DatabaseURL,
	)
	if err != nil {
		return err
	}

	poolConfig.MaxConns = runtimeConfig.PostgresMaxConns
	poolConfig.MinConns = runtimeConfig.PostgresMinConns
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

	redisOptions, err := goredis.ParseURL(
		runtimeConfig.RedisURL,
	)
	if err != nil {
		return err
	}

	redisClient := goredis.NewClient(redisOptions)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn(
				"close Redis client",
				"error",
				err,
			)
		}
	}()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return err
	}

	provider, err := identityoidc.New(
		ctx,
		runtimeConfig.OIDC,
		telemetry.HTTPTransport(nil),
	)
	if err != nil {
		return err
	}

	redisStore := identityredis.New(
		redisClient,
		runtimeConfig.RedisPrefix,
	)

	userRepository := identitypostgres.New(database)

	identityService, err := application.New(
		application.Config{
			TransactionTTL: runtimeConfig.TransactionTTL,
			SessionTTL:     runtimeConfig.SessionTTL,
		},
		provider,
		redisStore,
		redisStore,
		userRepository,
	)
	if err != nil {
		return err
	}

	grpcServer, err := grpcx.NewServer(
		grpcx.DefaultServerConfig(),
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	identityv1.RegisterIdentityServiceServer(
		grpcServer.GRPC(),
		identitygrpc.New(identityService),
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
			"identity-access listening",
			"address",
			runtimeConfig.GRPCAddress,
		)

		serverErrors <- grpcServer.Serve(listener)
	}()

	select {
	case err := <-serverErrors:
		if err != nil {
			return err
		}

		return nil

	case <-ctx.Done():
		logger.Info(
			"identity-access shutdown requested",
		)
	}

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		runtimeConfig.ShutdownTimeout,
	)
	defer cancel()

	if err := grpcServer.GracefulStop(
		shutdownContext,
	); err != nil &&
		!errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
