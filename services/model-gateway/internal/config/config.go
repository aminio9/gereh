// Package config handles environment configuration loading for Model Gateway.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config holds runtime configuration for the Model Gateway service.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string

	HTTPAddress string

	DatabaseURL string

	PostgresMaxConnections int32
	PostgresMinConnections int32

	ResolverGRPCTarget     string
	ResolverGRPCInsecure   bool
	ResolverGRPCServerName string

	RuntimePublicKeyFile string
	RuntimeTokenIssuer   string

	VaultAddress           string
	VaultMount             string
	VaultNamespace         string
	VaultTokenFile         string
	VaultStaticToken       string
	VaultCAFile            string
	VaultAllowInsecureHTTP bool
	VaultTimeout           time.Duration

	EventTopic               string
	RequireBudgetReservation bool

	ProviderTimeout time.Duration

	OutboxBatchSize    int
	OutboxPollInterval time.Duration
	OutboxLease        time.Duration
	OutboxMaxBackoff   time.Duration

	ShutdownTimeout time.Duration

	Kafka platformkafka.Config
}

// FromEnv loads Model Gateway configuration from the environment.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv("model-gateway")
	if err != nil {
		return Config{}, err
	}

	maxConnections, err := int32Environment("MODEL_GATEWAY_POSTGRES_MAX_CONNECTIONS", 20)
	if err != nil {
		return Config{}, err
	}

	minConnections, err := int32Environment("MODEL_GATEWAY_POSTGRES_MIN_CONNECTIONS", 2)
	if err != nil {
		return Config{}, err
	}

	resolverInsecure, err := boolEnvironment("MODEL_GATEWAY_RESOLVER_GRPC_INSECURE", true)
	if err != nil {
		return Config{}, err
	}

	requireBudget, err := boolEnvironment("MODEL_GATEWAY_REQUIRE_BUDGET_RESERVATION", false)
	if err != nil {
		return Config{}, err
	}

	vaultAllowInsecureHTTP, err := boolEnvironment("MODEL_GATEWAY_VAULT_ALLOW_INSECURE_HTTP", true)
	if err != nil {
		return Config{}, err
	}

	providerTimeout, err := durationEnvironment("MODEL_GATEWAY_PROVIDER_TIMEOUT", 120*time.Second)
	if err != nil {
		return Config{}, err
	}

	vaultTimeout, err := durationEnvironment("MODEL_GATEWAY_VAULT_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	pollInterval, err := durationEnvironment("MODEL_GATEWAY_OUTBOX_POLL_INTERVAL", 250*time.Millisecond)
	if err != nil {
		return Config{}, err
	}

	lease, err := durationEnvironment("MODEL_GATEWAY_OUTBOX_LEASE", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	maxBackoff, err := durationEnvironment("MODEL_GATEWAY_OUTBOX_MAX_BACKOFF", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationEnvironment("MODEL_GATEWAY_SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	batchSize, err := integerEnvironment("MODEL_GATEWAY_OUTBOX_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ServiceName:    "model-gateway",
		ServiceVersion: version,
		Environment:    envOrDefault("GEREH_ENVIRONMENT", "development"),

		HTTPAddress: envOrDefault("MODEL_GATEWAY_HTTP_ADDRESS", ":8087"),

		DatabaseURL: strings.TrimSpace(os.Getenv("MODEL_GATEWAY_DATABASE_URL")),

		PostgresMaxConnections: maxConnections,
		PostgresMinConnections: minConnections,

		ResolverGRPCTarget: envOrDefault(
			"MODEL_GATEWAY_RESOLVER_GRPC_TARGET",
			"passthrough:///127.0.0.1:18087",
		),
		ResolverGRPCInsecure:   resolverInsecure,
		ResolverGRPCServerName: strings.TrimSpace(os.Getenv("MODEL_GATEWAY_RESOLVER_GRPC_SERVER_NAME")),

		RuntimePublicKeyFile: strings.TrimSpace(os.Getenv("MODEL_GATEWAY_RUNTIME_PUBLIC_KEY_FILE")),
		RuntimeTokenIssuer:   envOrDefault("MODEL_GATEWAY_RUNTIME_TOKEN_ISSUER", "gereh-runtime"),

		VaultAddress: envOrDefault(
			"MODEL_GATEWAY_VAULT_ADDRESS",
			"http://127.0.0.1:8200",
		),
		VaultMount: envOrDefault(
			"MODEL_GATEWAY_VAULT_MOUNT",
			"model-byok",
		),
		VaultNamespace:         strings.TrimSpace(os.Getenv("MODEL_GATEWAY_VAULT_NAMESPACE")),
		VaultTokenFile:         strings.TrimSpace(os.Getenv("MODEL_GATEWAY_VAULT_TOKEN_FILE")),
		VaultStaticToken:       strings.TrimSpace(os.Getenv("MODEL_GATEWAY_VAULT_TOKEN")),
		VaultCAFile:            strings.TrimSpace(os.Getenv("MODEL_GATEWAY_VAULT_CA_FILE")),
		VaultAllowInsecureHTTP: vaultAllowInsecureHTTP,
		VaultTimeout:           vaultTimeout,

		EventTopic: envOrDefault(
			"MODEL_GATEWAY_EVENT_TOPIC",
			"gereh.model.usage.v1",
		),
		RequireBudgetReservation: requireBudget,
		ProviderTimeout:          providerTimeout,

		OutboxBatchSize:    batchSize,
		OutboxPollInterval: pollInterval,
		OutboxLease:        lease,
		OutboxMaxBackoff:   maxBackoff,

		ShutdownTimeout: shutdownTimeout,

		Kafka: kafkaConfig,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("MODEL_GATEWAY_DATABASE_URL is required")
	}

	if strings.EqualFold(cfg.Environment, "production") {
		if cfg.ResolverGRPCInsecure {
			return Config{}, fmt.Errorf("MODEL_GATEWAY_RESOLVER_GRPC_INSECURE must be false in production")
		}

		vaultURL, err := url.Parse(cfg.VaultAddress)
		if err != nil || vaultURL.Scheme != "https" {
			return Config{}, fmt.Errorf("MODEL_GATEWAY_VAULT_ADDRESS must use HTTPS in production")
		}

		if cfg.RuntimePublicKeyFile == "" {
			return Config{}, fmt.Errorf("MODEL_GATEWAY_RUNTIME_PUBLIC_KEY_FILE is required in production")
		}
	}

	return cfg, nil
}

func envOrDefault(name string, fallback string) string {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return fallback
	}
	return val
}

func boolEnvironment(name string, fallback bool) (bool, error) {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return fallback, nil
	}
	res, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}
	return res, nil
}

func durationEnvironment(name string, fallback time.Duration) (time.Duration, error) {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return fallback, nil
	}
	res, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return res, nil
}

func integerEnvironment(name string, fallback int) (int, error) {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return fallback, nil
	}
	res, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return res, nil
}

func int32Environment(name string, fallback int32) (int32, error) {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return fallback, nil
	}
	res, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return int32(res), nil
}
