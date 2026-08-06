// Package config loads and validates execution-orchestrator configuration.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
)

// Config defines execution-orchestrator runtime configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	HTTPAddress    string

	TemporalAddress   string
	TemporalNamespace string
	TemporalTaskQueue string

	TenantTopic      string
	KafkaConsumer    platformkafka.Config
	KafkaDLQTopic    string
	TenantGRPCTarget string

	RuntimeMode       string
	RuntimeGRPCTarget string

	InternalDevelopmentToken string

	ShutdownTimeout time.Duration
}

// FromEnv loads execution-orchestrator configuration.
func FromEnv(version string) (Config, error) {
	kafkaConfig, err := platformkafka.ConfigFromEnv(
		"execution-orchestrator",
	)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ServiceName:    "execution-orchestrator",
		ServiceVersion: version,
		Environment: envOrDefault(
			"GEREH_ENVIRONMENT",
			"development",
		),
		HTTPAddress: envOrDefault(
			"ORCHESTRATOR_HTTP_ADDRESS",
			":8088",
		),
		TemporalAddress: envOrDefault(
			"TEMPORAL_ADDRESS",
			"127.0.0.1:7233",
		),
		TemporalNamespace: envOrDefault(
			"TEMPORAL_NAMESPACE",
			"default",
		),
		TemporalTaskQueue: envOrDefault(
			"TEMPORAL_TENANT_TASK_QUEUE",
			"gereh-tenant-onboarding",
		),
		TenantTopic: envOrDefault(
			"ORCHESTRATOR_TENANT_TOPIC",
			"gereh.tenant.events.v1",
		),
		KafkaDLQTopic: envOrDefault(
			"ORCHESTRATOR_DLQ_TOPIC",
			"gereh.execution-orchestrator.dlq.v1",
		),
		TenantGRPCTarget: envOrDefault(
			"TENANT_GRPC_TARGET",
			"127.0.0.1:18082",
		),
		RuntimeMode: envOrDefault(
			"ORCHESTRATOR_RUNTIME_MODE",
			"noop",
		),
		RuntimeGRPCTarget: strings.TrimSpace(
			os.Getenv("RUNTIME_MANAGER_GRPC_TARGET"),
		),
		InternalDevelopmentToken: strings.TrimSpace(
			os.Getenv("TENANT_INTERNAL_DEV_TOKEN"),
		),
		ShutdownTimeout: 30 * time.Second,
		KafkaConsumer:   kafkaConfig,
	}

	config.KafkaConsumer.GroupID = envOrDefault(
		"ORCHESTRATOR_KAFKA_GROUP_ID",
		"execution-orchestrator-tenant-onboarding-v1",
	)

	config.KafkaConsumer.Topics = []string{
		config.TenantTopic,
	}

	config.KafkaConsumer.ConsumerStartOffset =
		platformkafka.ConsumerStartOffsetEarliest

	if strings.EqualFold(
		config.Environment,
		"production",
	) && config.RuntimeMode != "grpc" {
		return Config{}, fmt.Errorf(
			"ORCHESTRATOR_RUNTIME_MODE must be grpc in production",
		)
	}

	if config.RuntimeMode == "grpc" &&
		config.RuntimeGRPCTarget == "" {
		return Config{}, fmt.Errorf(
			"RUNTIME_MANAGER_GRPC_TARGET is required in grpc mode",
		)
	}

	return config, nil
}

func envOrDefault(
	name string,
	fallback string,
) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	return value
}
