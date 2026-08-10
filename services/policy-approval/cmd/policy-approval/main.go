// Package main runs the Gereh Policy Approval Service.
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
	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	policyauthorization "github.com/aminio9/gereh/services/policy-approval/internal/adapters/authorization"
	"github.com/aminio9/gereh/services/policy-approval/internal/adapters/organization"
	"github.com/aminio9/gereh/services/policy-approval/internal/adapters/outbox"
	policypostgres "github.com/aminio9/gereh/services/policy-approval/internal/adapters/postgres"
	policyapplication "github.com/aminio9/gereh/services/policy-approval/internal/application"
	"github.com/aminio9/gereh/services/policy-approval/internal/config"
	"github.com/aminio9/gereh/services/policy-approval/internal/engine"
	"github.com/aminio9/gereh/services/policy-approval/internal/security"
	policygrpc "github.com/aminio9/gereh/services/policy-approval/internal/transport/grpc"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error(
			"policy-approval service stopped with an error",
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
		organizationv1.NewOrganizationPolicyContextServiceClient(
			organizationConnection,
		)

	signer, err := security.NewSigner(
		runtimeConfig.ServiceName,
		runtimeConfig.SigningKeyBase64,
	)
	if err != nil {
		return err
	}

	celEngine, err := engine.NewCEL()
	if err != nil {
		return err
	}

	repository := policypostgres.New(
		database.Pool(),
	)

	authorizer := policyauthorization.NewTenantAuthorizer(
		tenantClient,
		runtimeConfig.AuthorizerTimeout,
	)

	policyService, err := policyapplication.New(
		repository,
		authorizer,
		organization.NewClient(
			organizationClient,
			runtimeConfig.AuthorizerTimeout,
		),
		engine.NewEvaluator(celEngine),
		signer,
		policyapplication.Config{
			EventTopic: runtimeConfig.EventTopic,

			EvaluationServicePrincipalID: runtimeConfig.EvaluationServicePrincipalID,

			BootstrapServicePrincipalID: runtimeConfig.BootstrapServicePrincipalID,

			DecisionTTL: runtimeConfig.DecisionTTL,
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

	allowedSPIFFEIDs := policygrpc.
		ParseAllowedSPIFFEIDs(
			strings.Join(
				runtimeConfig.AllowedInternalSPIFFEIDs,
				",",
			),
		)

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
				"configure Policy Service workload TLS: %w",
				err,
			)
		}

		serverConfig.TLSConfig = tlsConfig
	}

	serverConfig.UnaryInterceptors = append(
		serverConfig.UnaryInterceptors,
		policygrpc.InternalWorkloadUnaryInterceptor(
			policygrpc.InternalAuthConfig{
				Environment:      runtimeConfig.Environment,
				DevelopmentToken: runtimeConfig.InternalDevelopmentToken,
				AllowedSPIFFEIDs: allowedSPIFFEIDs,
			},
		),
		policygrpc.ActorBindingUnaryInterceptor(),
	)

	server, err := grpcx.NewServer(
		serverConfig,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}

	policyv1.RegisterPolicyManagementServiceServer(
		server.GRPC(),
		policygrpc.NewManagement(policyService, celEngine),
	)

	policyv1.RegisterPolicyEvaluationServiceServer(
		server.GRPC(),
		policygrpc.NewEvaluation(policyService),
	)

	policyv1.RegisterPolicyBootstrapServiceServer(
		server.GRPC(),
		policygrpc.NewBootstrap(policyService),
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
			"policy-approval service listening",
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
			"policy-approval service shutdown requested",
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
