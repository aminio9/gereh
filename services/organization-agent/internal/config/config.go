// Package config loads and validates organization-agent runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config defines organization-agent runtime configuration.
type Config struct {
	ServiceName            string
	ServiceVersion         string
	Environment            string
	GRPCAddress            string
	DatabaseURL            string
	PostgresMaxConnections int32
	PostgresMinConnections int32
	ShutdownTimeout        time.Duration

	CompanyEventTopic string
	AgentEventTopic   string

	TenantGRPCTarget            string
	TenantGRPCInsecure          bool
	AuthorizerTimeout           time.Duration
	BootstrapServicePrincipalID string

	InternalDevelopmentToken string
	AllowedInternalSPIFFEIDs []string

	OutboxBatchSize    int
	OutboxPollInterval time.Duration
	OutboxLease        time.Duration
	OutboxMaxBackoff   time.Duration

	Kafka platformkafka.Config
}

// FromEnv loads organization-agent configuration.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv(
		"organization",
	)
	if err != nil {
		return Config{}, err
	}

	maxConnections, err := int32Environment(
		"POSTGRES_MAX_CONNS",
		10,
	)
	if err != nil {
		return Config{}, err
	}

	minConnections, err := int32Environment(
		"POSTGRES_MIN_CONNS",
		2,
	)
	if err != nil {
		return Config{}, err
	}

	batchSize, err := integerEnvironment(
		"ORGANIZATION_OUTBOX_BATCH_SIZE",
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

	shutdownTimeout, err := durationEnvironment(
		"SHUTDOWN_TIMEOUT",
		15*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	pollInterval, err := durationEnvironment(
		"ORGANIZATION_OUTBOX_POLL_INTERVAL",
		500*time.Millisecond,
	)
	if err != nil {
		return Config{}, err
	}

	lease, err := durationEnvironment(
		"ORGANIZATION_OUTBOX_LEASE",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	maxBackoff, err := durationEnvironment(
		"ORGANIZATION_OUTBOX_MAX_BACKOFF",
		5*time.Minute,
	)
	if err != nil {
		return Config{}, err
	}

	authorizerTimeout, err := durationEnvironment(
		"ORGANIZATION_AUTHORIZER_TIMEOUT",
		3*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ServiceName:    "organization-agent",
		ServiceVersion: version,
		Environment: envOrDefault(
			"GEREH_ENVIRONMENT",
			"development",
		),
		GRPCAddress: envOrDefault(
			"ORGANIZATION_GRPC_ADDRESS",
			":18083",
		),
		DatabaseURL: strings.TrimSpace(
			os.Getenv("ORGANIZATION_DATABASE_URL"),
		),
		PostgresMaxConnections: maxConnections,
		PostgresMinConnections: minConnections,
		ShutdownTimeout:        shutdownTimeout,

		CompanyEventTopic: envOrDefault(
			"ORGANIZATION_COMPANY_EVENT_TOPIC",
			"gereh.organization.company.events.v1",
		),
		AgentEventTopic: envOrDefault(
			"ORGANIZATION_AGENT_EVENT_TOPIC",
			"gereh.organization.agent.events.v1",
		),

		TenantGRPCTarget: envOrDefault(
			"TENANT_GRPC_TARGET",
			"passthrough:///127.0.0.1:18082",
		),
		TenantGRPCInsecure: tenantInsecure,
		AuthorizerTimeout:  authorizerTimeout,

		BootstrapServicePrincipalID: strings.TrimSpace(
			os.Getenv("ORGANIZATION_BOOTSTRAP_SERVICE_PRINCIPAL_ID"),
		),
		InternalDevelopmentToken: strings.TrimSpace(
			os.Getenv("ORGANIZATION_INTERNAL_DEV_TOKEN"),
		),
		AllowedInternalSPIFFEIDs: splitCommaSeparatedPreserveCase(
			os.Getenv("ORGANIZATION_INTERNAL_ALLOWED_SPIFFE_IDS"),
		),

		OutboxBatchSize:    batchSize,
		OutboxPollInterval: pollInterval,
		OutboxLease:        lease,
		OutboxMaxBackoff:   maxBackoff,

		Kafka: kafkaConfig,
	}

	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf(
			"ORGANIZATION_DATABASE_URL is required",
		)
	}

	if config.BootstrapServicePrincipalID == "" {
		return Config{}, fmt.Errorf(
			"ORGANIZATION_BOOTSTRAP_SERVICE_PRINCIPAL_ID is required",
		)
	}

	if strings.EqualFold(
		config.Environment,
		"production",
	) && len(config.AllowedInternalSPIFFEIDs) == 0 {
		return Config{}, fmt.Errorf(
			"ORGANIZATION_INTERNAL_ALLOWED_SPIFFE_IDS is required in production",
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

	if config.PostgresMinConnections >
		config.PostgresMaxConnections {
		return Config{}, fmt.Errorf(
			"POSTGRES_MIN_CONNS cannot exceed POSTGRES_MAX_CONNS",
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

func splitCommaSeparatedPreserveCase(
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
