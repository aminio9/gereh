package kafka

import (
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "kafka-1:9092,kafka-2:9092")
	t.Setenv("KAFKA_CLIENT_ID", "tenant")
	t.Setenv("KAFKA_GROUP_ID", "tenant-projection")
	t.Setenv(
		"KAFKA_TOPICS",
		"gereh.tenant.events.v1,g ereh.audit.events.v1",
	)
	t.Setenv("KAFKA_DIAL_TIMEOUT", "5s")
	t.Setenv("KAFKA_MAX_POLL_RECORDS", "50")
	t.Setenv("KAFKA_SASL_MECHANISM", "scram-sha-256")
	t.Setenv("KAFKA_SASL_USERNAME", "gereh")
	t.Setenv("KAFKA_SASL_PASSWORD", "secret")

	config, err := ConfigFromEnv("fallback")
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if len(config.Brokers) != 2 {
		t.Fatalf(
			"Brokers length = %d, want 2",
			len(config.Brokers),
		)
	}

	if config.ClientID != "tenant" {
		t.Fatalf(
			"ClientID = %q, want tenant",
			config.ClientID,
		)
	}

	if config.DialTimeout != 5*time.Second {
		t.Fatalf(
			"DialTimeout = %s, want 5s",
			config.DialTimeout,
		)
	}

	if config.MaxPollRecords != 50 {
		t.Fatalf(
			"MaxPollRecords = %d, want 50",
			config.MaxPollRecords,
		)
	}
}

func TestValidateConsumerRequiresGroup(t *testing.T) {
	config := DefaultConfig("tenant")
	config.Brokers = []string{"localhost:9092"}
	config.Topics = []string{"events"}

	if err := config.ValidateConsumer(); err == nil {
		t.Fatal("ValidateConsumer() error = nil, want group error")
	}
}
