// Package config loads and validates policy-approval runtime configuration.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config defines policy-approval runtime configuration.
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

	AuthorizerTimeout time.Duration

	SigningKeyBase64 string

	EvaluationServicePrincipalID string
	BootstrapServicePrincipalID  string

	DecisionTTL time.Duration

	AllowedInternalSPIFFEIDs []string

	InternalDevelopmentToken string

	GRPCTLSCertFile string
	GRPCTLSKeyFile  string
	GRPCTLSCAFile   string

	OutboxBatchSize    int
	OutboxPollInterval time.Duration
	OutboxLease        time.Duration
	OutboxMaxBackoff   time.Duration

	Kafka platformkafka.Config
}

// FromEnv loads policy-approval configuration.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv(
		"policy",
	)
	if err != nil {
		return Config{}, err
	}

	maxConnections, err := int32Environment(
		"POLICY_POSTGRES_MAX_CONNECTIONS",
		20,
	)
	if err != nil {
		return Config{}, err
	}

	minConnections, err := int32Environment(
		"POLICY_POSTGRES_MIN_CONNECTIONS",
		2,
	)
	if err != nil {
		return Config{}, err
	}

	batchSize, err := integerEnvironment(
		"POLICY_OUTBOX_BATCH_SIZE",
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
		"POLICY_SHUTDOWN_TIMEOUT",
		15*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	pollInterval, err := durationEnvironment(
		"POLICY_OUTBOX_POLL_INTERVAL",
		250*time.Millisecond,
	)
	if err != nil {
		return Config{}, err
	}

	lease, err := durationEnvironment(
		"POLICY_OUTBOX_LEASE",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	maxBackoff, err := durationEnvironment(
		"POLICY_OUTBOX_MAX_BACKOFF",
		30*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	authorizerTimeout, err := durationEnvironment(
		"POLICY_AUTHORIZER_TIMEOUT",
		3*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	decisionTTL, err := durationEnvironment(
		"POLICY_DECISION_TTL",
		5*time.Minute,
	)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ServiceName:    "policy-approval",
		ServiceVersion: version,
		Environment: envOrDefault(
			"GEREH_ENVIRONMENT",
			"development",
		),
		GRPCAddress: envOrDefault(
			"POLICY_GRPC_ADDRESS",
			":18085",
		),
		DatabaseURL: strings.TrimSpace(
			os.Getenv("POLICY_DATABASE_URL"),
		),
		PostgresMaxConnections: maxConnections,
		PostgresMinConnections: minConnections,
		ShutdownTimeout:        shutdownTimeout,

		EventTopic: envOrDefault(
			"POLICY_EVENT_TOPIC",
			"gereh.policy.events.v1",
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

		AuthorizerTimeout: authorizerTimeout,

		SigningKeyBase64: strings.TrimSpace(
			os.Getenv("POLICY_SIGNING_KEY_BASE64"),
		),

		EvaluationServicePrincipalID: strings.TrimSpace(
			os.Getenv("POLICY_EVALUATION_SERVICE_PRINCIPAL_ID"),
		),
		BootstrapServicePrincipalID: strings.TrimSpace(
			os.Getenv("POLICY_BOOTSTRAP_SERVICE_PRINCIPAL_ID"),
		),

		DecisionTTL: decisionTTL,

		AllowedInternalSPIFFEIDs: splitCommaSeparatedPreserveCase(
			os.Getenv("POLICY_INTERNAL_ALLOWED_SPIFFE_IDS"),
		),

		InternalDevelopmentToken: strings.TrimSpace(
			os.Getenv("POLICY_INTERNAL_DEV_TOKEN"),
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
			"POLICY_DATABASE_URL is required",
		)
	}

	if config.SigningKeyBase64 == "" {
		return Config{}, fmt.Errorf(
			"POLICY_SIGNING_KEY_BASE64 is required",
		)
	}

	signingKey, err := base64.StdEncoding.DecodeString(
		config.SigningKeyBase64,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"decode POLICY_SIGNING_KEY_BASE64: %w",
			err,
		)
	}

	if len(signingKey) < 32 {
		return Config{}, fmt.Errorf(
			"POLICY_SIGNING_KEY_BASE64 must decode to at least 32 bytes",
		)
	}

	if config.EvaluationServicePrincipalID == "" {
		return Config{}, fmt.Errorf(
			"POLICY_EVALUATION_SERVICE_PRINCIPAL_ID is required",
		)
	}

	if config.BootstrapServicePrincipalID == "" {
		return Config{}, fmt.Errorf(
			"POLICY_BOOTSTRAP_SERVICE_PRINCIPAL_ID is required",
		)
	}

	if config.DecisionTTL <= 0 {
		return Config{}, fmt.Errorf(
			"POLICY_DECISION_TTL must be positive",
		)
	}

	if strings.EqualFold(
		config.Environment,
		"production",
	) && len(config.AllowedInternalSPIFFEIDs) == 0 {
		return Config{}, fmt.Errorf(
			"POLICY_INTERNAL_ALLOWED_SPIFFE_IDS is required in production",
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
			"POLICY_POSTGRES_MIN_CONNECTIONS cannot exceed POLICY_POSTGRES_MAX_CONNECTIONS",
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
