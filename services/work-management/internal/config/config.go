// Package config loads and validates work-management runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config defines work-management runtime configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	GRPCAddress    string
	DatabaseURL    string

	PostgresMaxConnections int32
	PostgresMinConnections int32
	ShutdownTimeout        time.Duration

	EventTopic string

	TenantGRPCTarget         string
	TenantGRPCInsecure       bool
	OrganizationGRPCTarget   string
	OrganizationGRPCInsecure bool

	AuthorizerTimeout         time.Duration
	OrganizationClientTimeout time.Duration

	OutboxBatchSize    int
	OutboxPollInterval time.Duration
	OutboxLease        time.Duration
	OutboxMaxBackoff   time.Duration

	Kafka platformkafka.Config
}

// FromEnv loads work-management configuration.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv(
		"work",
	)
	if err != nil {
		return Config{}, err
	}

	maxConnections, err := int32Environment(
		"WORK_POSTGRES_MAX_CONNECTIONS",
		20,
	)
	if err != nil {
		return Config{}, err
	}

	minConnections, err := int32Environment(
		"WORK_POSTGRES_MIN_CONNECTIONS",
		2,
	)
	if err != nil {
		return Config{}, err
	}

	batchSize, err := integerEnvironment(
		"WORK_OUTBOX_BATCH_SIZE",
		100,
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

	organizationInsecure, err := boolEnvironment(
		"ORGANIZATION_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationEnvironment(
		"WORK_SHUTDOWN_TIMEOUT",
		15*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	pollInterval, err := durationEnvironment(
		"WORK_OUTBOX_POLL_INTERVAL",
		250*time.Millisecond,
	)
	if err != nil {
		return Config{}, err
	}

	lease, err := durationEnvironment(
		"WORK_OUTBOX_LEASE",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	maxBackoff, err := durationEnvironment(
		"WORK_OUTBOX_MAX_BACKOFF",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	authorizerTimeout, err := durationEnvironment(
		"WORK_AUTHORIZER_TIMEOUT",
		3*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	organizationTimeout, err := durationEnvironment(
		"WORK_DOWNSTREAM_TIMEOUT",
		3*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ServiceName:    "work-management",
		ServiceVersion: version,
		Environment: envOrDefault(
			"GEREH_ENVIRONMENT",
			"development",
		),
		GRPCAddress: envOrDefault(
			"WORK_GRPC_ADDRESS",
			":18084",
		),
		DatabaseURL: strings.TrimSpace(
			os.Getenv("WORK_DATABASE_URL"),
		),
		PostgresMaxConnections: maxConnections,
		PostgresMinConnections: minConnections,
		ShutdownTimeout:        shutdownTimeout,

		EventTopic: envOrDefault(
			"WORK_EVENT_TOPIC",
			"gereh.work.events.v1",
		),

		TenantGRPCTarget: envOrDefault(
			"TENANT_GRPC_TARGET",
			"passthrough:///127.0.0.1:18082",
		),
		TenantGRPCInsecure: tenantInsecure,

		OrganizationGRPCTarget: envOrDefault(
			"ORGANIZATION_GRPC_TARGET",
			"passthrough:///127.0.0.1:18083",
		),
		OrganizationGRPCInsecure: organizationInsecure,

		AuthorizerTimeout:         authorizerTimeout,
		OrganizationClientTimeout: organizationTimeout,

		OutboxBatchSize:    batchSize,
		OutboxPollInterval: pollInterval,
		OutboxLease:        lease,
		OutboxMaxBackoff:   maxBackoff,

		Kafka: kafkaConfig,
	}

	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf(
			"WORK_DATABASE_URL is required",
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

	if strings.EqualFold(
		config.Environment,
		"production",
	) && config.OrganizationGRPCInsecure {
		return Config{}, fmt.Errorf(
			"ORGANIZATION_GRPC_INSECURE must be false in production",
		)
	}

	if config.PostgresMinConnections >
		config.PostgresMaxConnections {
		return Config{}, fmt.Errorf(
			"WORK_POSTGRES_MIN_CONNECTIONS cannot exceed WORK_POSTGRES_MAX_CONNECTIONS",
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

func integerEnvironment(
	name string,
	fallback int,
) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	result, err := strconv.Atoi(value)
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
