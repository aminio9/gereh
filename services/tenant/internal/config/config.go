// Package config loads and validates tenant-service runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config defines tenant-service runtime configuration.
type Config struct {
	ServiceName            string
	ServiceVersion         string
	Environment            string
	GRPCAddress            string
	DatabaseURL            string
	PostgresMaxConnections int32
	PostgresMinConnections int32
	ShutdownTimeout        time.Duration

	EventTopic           string
	DefaultRegion        string
	AllowedRegions       []string
	DefaultRetentionDays int32

	WorkflowServicePrincipalID string
	InternalDevelopmentToken   string
	AllowedInternalSPIFFEIDs   []string

	GRPCTLSCertFile string
	GRPCTLSKeyFile  string
	GRPCTLSCAFile   string

	OutboxBatchSize    int
	OutboxPollInterval time.Duration
	OutboxLease        time.Duration
	OutboxMaxBackoff   time.Duration

	Kafka platformkafka.Config
}

// FromEnv loads tenant-service configuration.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv(
		"tenant",
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

	retentionDays, err := int32Environment(
		"TENANT_DEFAULT_RETENTION_DAYS",
		90,
	)
	if err != nil {
		return Config{}, err
	}

	batchSize, err := integerEnvironment(
		"TENANT_OUTBOX_BATCH_SIZE",
		100,
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
		"TENANT_OUTBOX_POLL_INTERVAL",
		500*time.Millisecond,
	)
	if err != nil {
		return Config{}, err
	}

	lease, err := durationEnvironment(
		"TENANT_OUTBOX_LEASE",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	maxBackoff, err := durationEnvironment(
		"TENANT_OUTBOX_MAX_BACKOFF",
		5*time.Minute,
	)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ServiceName:    "tenant",
		ServiceVersion: version,
		Environment: envOrDefault(
			"GEREH_ENVIRONMENT",
			"development",
		),
		GRPCAddress: envOrDefault(
			"TENANT_GRPC_ADDRESS",
			":18082",
		),
		DatabaseURL: strings.TrimSpace(
			os.Getenv("TENANT_DATABASE_URL"),
		),
		PostgresMaxConnections: maxConnections,
		PostgresMinConnections: minConnections,
		ShutdownTimeout:        shutdownTimeout,

		EventTopic: envOrDefault(
			"TENANT_EVENT_TOPIC",
			"gereh.tenant.events.v1",
		),
		DefaultRegion: envOrDefault(
			"TENANT_DEFAULT_REGION",
			"local",
		),
		AllowedRegions: splitCommaSeparated(
			envOrDefault(
				"TENANT_ALLOWED_REGIONS",
				"local",
			),
		),
		DefaultRetentionDays: retentionDays,

		WorkflowServicePrincipalID: strings.TrimSpace(
			os.Getenv("TENANT_WORKFLOW_SERVICE_PRINCIPAL_ID"),
		),
		InternalDevelopmentToken: strings.TrimSpace(
			os.Getenv("TENANT_INTERNAL_DEV_TOKEN"),
		),
		AllowedInternalSPIFFEIDs: splitCommaSeparatedPreserveCase(
			os.Getenv("TENANT_INTERNAL_ALLOWED_SPIFFE_IDS"),
		),

		GRPCTLSCertFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_CERT_FILE"),
		),
		GRPCTLSKeyFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_KEY_FILE"),
		),
		GRPCTLSCAFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_CA_FILE"),
		),

		OutboxBatchSize:    batchSize,
		OutboxPollInterval: pollInterval,
		OutboxLease:        lease,
		OutboxMaxBackoff:   maxBackoff,

		Kafka: kafkaConfig,
	}

	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf(
			"TENANT_DATABASE_URL is required",
		)
	}

	if config.WorkflowServicePrincipalID == "" {
		return Config{}, fmt.Errorf(
			"TENANT_WORKFLOW_SERVICE_PRINCIPAL_ID is required",
		)
	}

	if strings.EqualFold(
		config.Environment,
		"production",
	) && len(config.AllowedInternalSPIFFEIDs) == 0 {
		return Config{}, fmt.Errorf(
			"TENANT_INTERNAL_ALLOWED_SPIFFE_IDS is required in production",
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

func splitCommaSeparated(
	value string,
) []string {
	var result []string

	for _, item := range strings.Split(value, ",") {
		item = strings.ToLower(
			strings.TrimSpace(item),
		)

		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

// splitCommaSeparatedPreserveCase splits a comma-separated list without
// lowercasing. SPIFFE IDs are case-sensitive.
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
