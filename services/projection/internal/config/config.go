// Package config loads and validates projection runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config defines projection runtime configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	GRPCAddress    string
	DatabaseURL    string

	PostgresMaxConnections int32
	PostgresMinConnections int32
	ShutdownTimeout        time.Duration

	ServicePrincipalID string

	TenantGRPCTarget   string
	TenantGRPCInsecure bool
	AuthorizerTimeout  time.Duration

	GRPCTLSCertFile string
	GRPCTLSKeyFile  string
	GRPCTLSCAFile   string

	Kafka platformkafka.Config
}

// FromEnv loads projection configuration.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv(
		"projection",
	)
	if err != nil {
		return Config{}, err
	}

	maxConnections, err := int32Environment(
		"PROJECTION_POSTGRES_MAX_CONNECTIONS",
		20,
	)
	if err != nil {
		return Config{}, err
	}

	minConnections, err := int32Environment(
		"PROJECTION_POSTGRES_MIN_CONNECTIONS",
		2,
	)
	if err != nil {
		return Config{}, err
	}

	tenantInsecure, err := boolEnvironment(
		"TENANT_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationEnvironment(
		"PROJECTION_SHUTDOWN_TIMEOUT",
		15*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	authorizerTimeout, err := durationEnvironment(
		"PROJECTION_AUTHORIZER_TIMEOUT",
		3*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	if len(kafkaConfig.Topics) == 0 {
		kafkaConfig.Topics = splitCommaSeparated(
			envOrDefault(
				"PROJECTION_KAFKA_TOPICS",
				"gereh.tenant.events.v1,"+
					"gereh.organization.company.events.v1,"+
					"gereh.organization.agent.events.v1,"+
					"gereh.work.events.v1",
			),
		)
	}

	if kafkaConfig.GroupID == "" {
		kafkaConfig.GroupID = envOrDefault(
			"PROJECTION_KAFKA_GROUP_ID",
			"gereh-projection",
		)
	}

	if kafkaConfig.ConsumerStartOffset == "" {
		kafkaConfig.ConsumerStartOffset =
			platformkafka.ConsumerStartOffsetEarliest
	}

	config := Config{
		ServiceName:    "projection",
		ServiceVersion: version,
		Environment: envOrDefault(
			"GEREH_ENVIRONMENT",
			"development",
		),
		GRPCAddress: envOrDefault(
			"PROJECTION_GRPC_ADDRESS",
			":18086",
		),
		DatabaseURL: strings.TrimSpace(
			os.Getenv("PROJECTION_DATABASE_URL"),
		),
		PostgresMaxConnections: maxConnections,
		PostgresMinConnections: minConnections,
		ShutdownTimeout:        shutdownTimeout,

		ServicePrincipalID: strings.TrimSpace(
			os.Getenv("PROJECTION_SERVICE_PRINCIPAL_ID"),
		),

		TenantGRPCTarget: envOrDefault(
			"TENANT_GRPC_TARGET",
			"passthrough:///127.0.0.1:18082",
		),
		TenantGRPCInsecure: tenantInsecure,
		AuthorizerTimeout:  authorizerTimeout,

		GRPCTLSCertFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_CERT_FILE"),
		),
		GRPCTLSKeyFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_KEY_FILE"),
		),
		GRPCTLSCAFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_CA_FILE"),
		),

		Kafka: kafkaConfig,
	}

	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf(
			"PROJECTION_DATABASE_URL is required",
		)
	}

	if config.ServicePrincipalID == "" {
		return Config{}, fmt.Errorf(
			"PROJECTION_SERVICE_PRINCIPAL_ID is required",
		)
	}

	if strings.EqualFold(
		config.Environment,
		"production",
	) && config.TenantGRPCInsecure {
		return Config{}, fmt.Errorf(
			"TENANT_GRPC_INSECURE must be false in production",
		)
	}

	if strings.EqualFold(config.Environment, "production") &&
		(strings.TrimSpace(config.GRPCTLSCertFile) == "" ||
			strings.TrimSpace(config.GRPCTLSKeyFile) == "" ||
			strings.TrimSpace(config.GRPCTLSCAFile) == "") {
		return Config{}, fmt.Errorf(
			"GRPC_TLS_CERT_FILE, GRPC_TLS_KEY_FILE and GRPC_TLS_CA_FILE are required in production",
		)
	}

	if config.PostgresMinConnections >
		config.PostgresMaxConnections {
		return Config{}, fmt.Errorf(
			"PROJECTION_POSTGRES_MIN_CONNECTIONS cannot exceed PROJECTION_POSTGRES_MAX_CONNECTIONS",
		)
	}

	return config, nil
}

func envOrDefault(
	name string,
	fallback string,
) string {
	if value := strings.TrimSpace(
		os.Getenv(name),
	); value != "" {
		return value
	}

	return fallback
}

func boolEnvironment(
	name string,
	fallback bool,
) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	result, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf(
			"parse %s: %w",
			name,
			err,
		)
	}

	return result, nil
}

func durationEnvironment(
	name string,
	fallback time.Duration,
) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf(
			"parse %s: %w",
			name,
			err,
		)
	}

	return result, nil
}

func int32Environment(
	name string,
	fallback int32,
) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	result, err := strconv.ParseInt(
		value,
		10,
		32,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"parse %s: %w",
			name,
			err,
		)
	}

	return int32(result), nil
}

func splitCommaSeparated(
	value string,
) []string {
	var result []string

	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)

		if item != "" {
			result = append(result, item)
		}
	}

	return result
}
