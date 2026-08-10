package grpcx

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/aminio9/gereh/platform/go/observability"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

// NewClient creates an instrumented gRPC client connection.
//
// The connection is lazy and establishes transport connections when first used.
func NewClient(
	config ClientConfig,
	telemetry *observability.Telemetry,
	additionalOptions ...grpc.DialOption,
) (*grpc.ClientConn, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf(
			"validate gRPC client configuration: %w",
			err,
		)
	}

	transportCredentials, err := clientTransportCredentials(config)
	if err != nil {
		return nil, err
	}

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithStatsHandler(
			otelgrpc.NewClientHandler(
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
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(
				config.MaxReceiveMessageBytes,
			),
			grpc.MaxCallSendMsgSize(
				config.MaxSendMessageBytes,
			),
		),
		grpc.WithChainUnaryInterceptor(
			UnaryClientInterceptors(
				config.DefaultUnaryTimeout,
			)...,
		),
		grpc.WithChainStreamInterceptor(
			StreamClientInterceptors()...,
		),
	}

	if config.DefaultServiceConfig != "" {
		dialOptions = append(
			dialOptions,
			grpc.WithDefaultServiceConfig(
				config.DefaultServiceConfig,
			),
		)
	}

	if config.Dialer != nil {
		dialOptions = append(
			dialOptions,
			grpc.WithContextDialer(config.Dialer),
		)
	}

	dialOptions = append(
		dialOptions,
		additionalOptions...,
	)

	connection, err := grpc.NewClient(
		config.Target,
		dialOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("create gRPC client: %w", err)
	}

	return connection, nil
}

// CheckHealth checks the standard gRPC health service.
func CheckHealth(
	ctx context.Context,
	connection grpc.ClientConnInterface,
	service string,
) error {
	client := grpc_health_v1.NewHealthClient(connection)

	response, err := client.Check(
		ctx,
		&grpc_health_v1.HealthCheckRequest{
			Service: service,
		},
	)
	if err != nil {
		return fmt.Errorf("check gRPC health: %w", err)
	}

	if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf(
			"gRPC service %q is not serving: %s",
			service,
			response.GetStatus(),
		)
	}

	return nil
}

func clientTransportCredentials(
	config ClientConfig,
) (credentials.TransportCredentials, error) {
	if config.Insecure {
		return insecure.NewCredentials(), nil
	}

	tlsConfig := config.TLSConfig

	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: config.ServerName,
		}
	} else {
		tlsConfig = tlsConfig.Clone()

		if tlsConfig.MinVersion == 0 {
			tlsConfig.MinVersion = tls.VersionTLS12
		}

		if tlsConfig.ServerName == "" {
			tlsConfig.ServerName = config.ServerName
		}
	}

	return credentials.NewTLS(tlsConfig), nil
}
