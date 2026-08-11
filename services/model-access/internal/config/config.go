// Package config loads Model Access runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config is the Model Access runtime configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string

	GRPCAddress string

	DatabaseURL string

	PostgresMaxConnections int32
	PostgresMinConnections int32

	TenantGRPCTarget     string
	TenantGRPCInsecure   bool
	TenantGRPCServerName string

	AuthorizerTimeout time.Duration

	EventTopic string

	IdempotencyTTL time.Duration

	OutboxBatchSize int

	OutboxPollInterval time.Duration
	OutboxLease        time.Duration
	OutboxMaxBackoff   time.Duration

	ShutdownTimeout time.Duration

	GRPCTLSCertFile string
	GRPCTLSKeyFile  string
	GRPCTLSCAFile   string

	Kafka platformkafka.Config
}

// FromEnv loads configuration from the environment.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv("model-access")
	if err != nil {
		return Config{}, err
	}

	maxConnections, err := int32Environment(
		"MODEL_ACCESS_POSTGRES_MAX_CONNECTIONS",
		20,
	)
	if err != nil {
		return Config{}, err
	}

	minConnections, err := int32Environment(
		"MODEL_ACCESS_POSTGRES_MIN_CONNECTIONS",
		2,
	)
	if err != nil {
		return Config{}, err
	}

	tenantInsecure, err := boolEnvironment("TENANT_GRPC_INSECURE", true)
	if err != nil {
		return Config{}, err
	}

	authorizerTimeout, err := durationEnvironment(
		"MODEL_ACCESS_AUTHORIZER_TIMEOUT",
		3*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	idempotencyTTL, err := durationEnvironment(
		"MODEL_ACCESS_IDEMPOTENCY_TTL",
		24*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}

	pollInterval, err := durationEnvironment(
		"MODEL_ACCESS_OUTBOX_POLL_INTERVAL",
		250*time.Millisecond,
	)
	if err != nil {
		return Config{}, err
	}

	lease, err := durationEnvironment(
		"MODEL_ACCESS_OUTBOX_LEASE",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	maxBackoff, err := durationEnvironment(
		"MODEL_ACCESS_OUTBOX_MAX_BACKOFF",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationEnvironment(
		"MODEL_ACCESS_SHUTDOWN_TIMEOUT",
		15*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	batchSize, err := integerEnvironment(
		"MODEL_ACCESS_OUTBOX_BATCH_SIZE",
		100,
	)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ServiceName:    "model-access",
		ServiceVersion: version,
		Environment:    envOrDefault("GEREH_ENVIRONMENT", "development"),

		GRPCAddress: envOrDefault("MODEL_ACCESS_GRPC_ADDRESS", ":18087"),

		DatabaseURL: strings.TrimSpace(os.Getenv("MODEL_ACCESS_DATABASE_URL")),

		PostgresMaxConnections: maxConnections,
		PostgresMinConnections: minConnections,

		TenantGRPCTarget: envOrDefault(
			"TENANT_GRPC_TARGET",
			"passthrough:///127.0.0.1:18082",
		),
		TenantGRPCInsecure:   tenantInsecure,
		TenantGRPCServerName: strings.TrimSpace(os.Getenv("TENANT_GRPC_SERVER_NAME")),

		AuthorizerTimeout: authorizerTimeout,

		EventTopic: envOrDefault(
			"MODEL_ACCESS_EVENT_TOPIC",
			"gereh.model.events.v1",
		),

		IdempotencyTTL: idempotencyTTL,

		OutboxBatchSize: batchSize,

		OutboxPollInterval: pollInterval,
		OutboxLease:        lease,
		OutboxMaxBackoff:   maxBackoff,

		ShutdownTimeout: shutdownTimeout,

		GRPCTLSCertFile: strings.TrimSpace(os.Getenv("GRPC_TLS_CERT_FILE")),
		GRPCTLSKeyFile:  strings.TrimSpace(os.Getenv("GRPC_TLS_KEY_FILE")),
		GRPCTLSCAFile:   strings.TrimSpace(os.Getenv("GRPC_TLS_CA_FILE")),

		Kafka: kafkaConfig,
	}

	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf("MODEL_ACCESS_DATABASE_URL is required")
	}

	if config.PostgresMinConnections > config.PostgresMaxConnections {
		return Config{}, fmt.Errorf(
			"Model Access minimum PostgreSQL connections cannot exceed maximum",
		)
	}

	if config.IdempotencyTTL < time.Hour {
		return Config{}, fmt.Errorf(
			"MODEL_ACCESS_IDEMPOTENCY_TTL must be at least one hour",
		)
	}

	if strings.EqualFold(config.Environment, "production") {
		if config.TenantGRPCInsecure {
			return Config{}, fmt.Errorf(
				"TENANT_GRPC_INSECURE must be false in production",
			)
		}

		if config.TenantGRPCServerName == "" {
			return Config{}, fmt.Errorf(
				"TENANT_GRPC_SERVER_NAME is required in production",
			)
		}

		if config.GRPCTLSCertFile == "" ||
			config.GRPCTLSKeyFile == "" ||
			config.GRPCTLSCAFile == "" {
			return Config{}, fmt.Errorf(
				"GRPC_TLS_CERT_FILE, GRPC_TLS_KEY_FILE and GRPC_TLS_CA_FILE are required in production",
			)
		}
	}

	return config, nil
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))

	if value == "" {
		return fallback
	}

	return value
}

func boolEnvironment(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))

	if value == "" {
		return fallback, nil
	}

	result, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
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
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return result, nil
}

func integerEnvironment(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))

	if value == "" {
		return fallback, nil
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return result, nil
}

func int32Environment(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))

	if value == "" {
		return fallback, nil
	}

	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return int32(result), nil
}
