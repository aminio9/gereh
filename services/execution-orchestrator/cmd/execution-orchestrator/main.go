// Package main runs the Gereh Execution Orchestrator.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	organizationadapter "github.com/aminio9/gereh/services/execution-orchestrator/internal/adapters/organization"
	policyadapter "github.com/aminio9/gereh/services/execution-orchestrator/internal/adapters/policy"
	runtimeadapter "github.com/aminio9/gereh/services/execution-orchestrator/internal/adapters/runtime"
	tenantadapter "github.com/aminio9/gereh/services/execution-orchestrator/internal/adapters/tenant"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/application"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/config"
	"github.com/aminio9/gereh/services/execution-orchestrator/internal/ports"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error(
			"execution orchestrator stopped",
			"error",
			err,
		)

		os.Exit(1)
	}
}

func run() (runErr error) {
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

		runErr = errors.Join(
			runErr,
			telemetry.Shutdown(shutdownContext),
		)
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

	temporalClient, err := client.Dial(
		client.Options{
			HostPort:  runtimeConfig.TemporalAddress,
			Namespace: runtimeConfig.TemporalNamespace,
			Logger:    temporalSlogAdapter{logger: logger},
		},
	)
	if err != nil {
		return fmt.Errorf(
			"connect to Temporal: %w",
			err,
		)
	}
	defer temporalClient.Close()

	tenantConnection, err := newInternalClient(
		telemetry,
		runtimeConfig.TenantGRPCTarget,
		runtimeConfig.TenantGRPCInsecure,
		runtimeConfig.TenantGRPCServerName,
		runtimeConfig,
	)
	if err != nil {
		return fmt.Errorf(
			"connect to Tenant Service: %w",
			err,
		)
	}
	defer func() {
		_ = tenantConnection.Close()
	}()

	tenantClient := tenantadapter.New(
		tenantConnection,
		runtimeConfig.InternalDevelopmentToken,
	)

	organizationConnection, err := newInternalClient(
		telemetry,
		runtimeConfig.OrganizationGRPCTarget,
		runtimeConfig.OrganizationGRPCInsecure,
		runtimeConfig.OrganizationGRPCServerName,
		runtimeConfig,
	)
	if err != nil {
		return fmt.Errorf(
			"connect to Organization Service: %w",
			err,
		)
	}
	defer func() {
		_ = organizationConnection.Close()
	}()

	organizationClient := organizationadapter.New(
		organizationConnection,
		runtimeConfig.InternalDevelopmentToken,
	)

	policyConnection, err := newInternalClient(
		telemetry,
		runtimeConfig.PolicyGRPCTarget,
		runtimeConfig.PolicyGRPCInsecure,
		runtimeConfig.PolicyGRPCServerName,
		runtimeConfig,
	)
	if err != nil {
		return fmt.Errorf(
			"connect to Policy Service: %w",
			err,
		)
	}
	defer func() {
		_ = policyConnection.Close()
	}()

	policyClient := policyadapter.New(
		policyConnection,
		runtimeConfig.InternalDevelopmentToken,
	)

	var runtimeProvisioner ports.RuntimeProvisioner

	switch runtimeConfig.RuntimeMode {
	case "noop":
		runtimeProvisioner = runtimeadapter.NoopProvisioner{}

	case "grpc":
		runtimeConnection, connectionErr := newInternalClient(
			telemetry,
			runtimeConfig.RuntimeGRPCTarget,
			runtimeConfig.RuntimeGRPCInsecure,
			runtimeConfig.RuntimeGRPCServerName,
			runtimeConfig,
		)
		if connectionErr != nil {
			return fmt.Errorf(
				"connect to Runtime Manager: %w",
				connectionErr,
			)
		}

		defer func() {
			_ = runtimeConnection.Close()
		}()

		runtimeProvisioner =
			runtimeadapter.NewGRPCProvisioner(
				runtimeConnection,
			)

	default:
		return fmt.Errorf(
			"unsupported runtime mode %q",
			runtimeConfig.RuntimeMode,
		)
	}

	activities := application.NewActivities(
		tenantClient,
		runtimeProvisioner,
		organizationClient,
		policyClient,
	)

	temporalWorker := worker.New(
		temporalClient,
		runtimeConfig.TemporalTaskQueue,
		worker.Options{
			MaxConcurrentActivityExecutionSize:     50,
			MaxConcurrentWorkflowTaskExecutionSize: 100,
			WorkerStopTimeout:                      runtimeConfig.ShutdownTimeout,
		},
	)

	temporalWorker.RegisterWorkflowWithOptions(
		application.ProvisionTenantWorkflow,
		workflow.RegisterOptions{
			Name: application.ProvisionTenantWorkflowName,
		},
	)

	temporalWorker.RegisterActivity(activities)

	if err := temporalWorker.Start(); err != nil {
		return fmt.Errorf(
			"start Temporal worker: %w",
			err,
		)
	}
	defer temporalWorker.Stop()

	consumer, err := platformkafka.NewConsumer(
		runtimeConfig.KafkaConsumer,
		telemetry,
		logger,
	)
	if err != nil {
		return err
	}
	defer consumer.Close()

	if err := consumer.Ping(ctx); err != nil {
		return err
	}

	trigger := application.NewTenantCreatedTrigger(
		temporalClient,
		runtimeConfig.TemporalTaskQueue,
	)

	var ready atomic.Bool

	consumerErrors := make(chan error, 1)

	go func() {
		consumerErrors <- consumer.Run(ctx, trigger.Handle)
	}()

	go func() {
		select {
		case <-consumer.Ready():
			ready.Store(true)
		case <-ctx.Done():
		}
	}()

	server := healthServer(runtimeConfig.HTTPAddress, &ready)

	serverErrors := make(chan error, 1)

	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		serverErrors <- err
	}()

	select {
	case err := <-consumerErrors:
		if err != nil {
			return fmt.Errorf(
				"consume tenant events: %w",
				err,
			)
		}

	case err := <-serverErrors:
		if err != nil {
			return fmt.Errorf(
				"serve orchestrator health API: %w",
				err,
			)
		}

	case <-ctx.Done():
	}

	ready.Store(false)

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		runtimeConfig.ShutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf(
			"shutdown health server: %w",
			err,
		)
	}

	return nil
}

func newInternalClient(
	telemetry *observability.Telemetry,
	target string,
	insecureTransport bool,
	serverName string,
	runtimeConfig config.Config,
) (*grpc.ClientConn, error) {
	clientConfig := grpcx.DefaultClientConfig(
		target,
	)

	clientConfig.Insecure = insecureTransport

	if !insecureTransport {
		tlsConfig, err :=
			grpcx.LoadWorkloadClientTLS(
				grpcx.WorkloadTLSFiles{
					CertificateFile: runtimeConfig.GRPCTLSCertFile,
					PrivateKeyFile:  runtimeConfig.GRPCTLSKeyFile,
					CAFile:          runtimeConfig.GRPCTLSCAFile,
				},
				serverName,
			)
		if err != nil {
			return nil, err
		}

		clientConfig.TLSConfig = tlsConfig
	}

	return grpcx.NewClient(
		clientConfig,
		telemetry,
	)
}

func healthServer(
	address string,
	ready *atomic.Bool,
) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"/health/live",
		func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(
				writer,
				http.StatusOK,
				map[string]string{
					"status": "ok",
				},
			)
		},
	)

	mux.HandleFunc(
		"/health/ready",
		func(writer http.ResponseWriter, _ *http.Request) {
			if !ready.Load() {
				writeJSON(
					writer,
					http.StatusServiceUnavailable,
					map[string]string{
						"status": "not_ready",
					},
				)
				return
			}

			writeJSON(
				writer,
				http.StatusOK,
				map[string]string{
					"status": "ready",
				},
			)
		},
	)

	return &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func writeJSON(
	writer http.ResponseWriter,
	statusCode int,
	value any,
) {
	writer.Header().Set(
		"Content-Type",
		"application/json",
	)
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

// temporalSlogAdapter adapts slog to the Temporal client logger interface.
type temporalSlogAdapter struct {
	logger *slog.Logger
}

func (adapter temporalSlogAdapter) Debug(
	message string,
	keyValues ...any,
) {
	adapter.logger.Debug(message, keyValues...)
}

func (adapter temporalSlogAdapter) Info(
	message string,
	keyValues ...any,
) {
	adapter.logger.Info(message, keyValues...)
}

func (adapter temporalSlogAdapter) Warn(
	message string,
	keyValues ...any,
) {
	adapter.logger.Warn(message, keyValues...)
}

func (adapter temporalSlogAdapter) Error(
	message string,
	keyValues ...any,
) {
	adapter.logger.Error(message, keyValues...)
}
