package grpcx

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/aminio9/gereh/platform/go/observability"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server wraps a configured gRPC server and its health service.
type Server struct {
	server       *grpc.Server
	healthServer *health.Server
	config       ServerConfig
}

// NewServer creates a production-oriented gRPC server.
func NewServer(
	config ServerConfig,
	telemetry *observability.Telemetry,
	logger *slog.Logger,
	additionalOptions ...grpc.ServerOption,
) (*Server, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf(
			"validate gRPC server configuration: %w",
			err,
		)
	}

	if logger == nil {
		logger = slog.Default()
	}

	serverOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxReceiveMessageBytes),
		grpc.MaxSendMsgSize(config.MaxSendMessageBytes),
		grpc.StatsHandler(
			otelgrpc.NewServerHandler(
				otelgrpc.WithTracerProvider(
					telemetry.TracerProvider(),
				),
				otelgrpc.WithMeterProvider(
					telemetry.MeterProvider(),
				),
				otelgrpc.WithPropagators(
					telemetry.Propagator(),
				),
			),
		),
		grpc.ChainUnaryInterceptor(
			UnaryServerInterceptors(logger)...,
		),
		grpc.ChainStreamInterceptor(
			StreamServerInterceptors(logger)...,
		),
	}

	serverOptions = append(
		serverOptions,
		additionalOptions...,
	)

	grpcServer := grpc.NewServer(serverOptions...)
	healthServer := health.NewServer()

	grpc_health_v1.RegisterHealthServer(
		grpcServer,
		healthServer,
	)

	healthServer.SetServingStatus(
		"",
		grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	)

	if config.EnableReflection {
		reflection.Register(grpcServer)
	}

	return &Server{
		server:       grpcServer,
		healthServer: healthServer,
		config:       config,
	}, nil
}

// GRPC returns the underlying gRPC server for service registration.
func (server *Server) GRPC() *grpc.Server {
	return server.server
}

// SetServing marks the server and optionally one service as serving.
func (server *Server) SetServing(service string) {
	server.healthServer.SetServingStatus(
		service,
		grpc_health_v1.HealthCheckResponse_SERVING,
	)
}

// SetNotServing marks the server and optionally one service as not serving.
func (server *Server) SetNotServing(service string) {
	server.healthServer.SetServingStatus(
		service,
		grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	)
}

// Serve marks the process as serving and starts accepting requests.
func (server *Server) Serve(listener net.Listener) error {
	server.SetServing("")

	if err := server.server.Serve(listener); err != nil {
		return fmt.Errorf("serve gRPC: %w", err)
	}

	return nil
}

// GracefulStop stops accepting traffic and waits for active calls.
//
// When the context expires, the server is force-stopped.
func (server *Server) GracefulStop(ctx context.Context) error {
	server.SetNotServing("")

	stopped := make(chan struct{})

	go func() {
		server.server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		return nil
	case <-ctx.Done():
		server.server.Stop()

		return fmt.Errorf(
			"force-stop gRPC server: %w",
			ctx.Err(),
		)
	}
}
