//go:build integration

package kafka

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"github.com/aminio9/gereh/platform/go/observability"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProducerConsumerIntegration(t *testing.T) {
	brokers := splitCommaSeparated(
		os.Getenv("KAFKA_BROKERS"),
	)

	if len(brokers) == 0 {
		t.Skip("KAFKA_BROKERS is not configured")
	}

	topic := firstIntegrationTopic(
		os.Getenv("KAFKA_INTEGRATION_TOPIC"),
		"gereh.audit.events.v1",
	)

	identifier := fmt.Sprintf(
		"integration-%d",
		time.Now().UTC().UnixNano(),
	)

	telemetryConfig := observability.DefaultConfig(
		"kafka-integration-test",
		"dev",
	)

	telemetry, err := observability.Setup(
		context.Background(),
		telemetryConfig,
	)
	if err != nil {
		t.Fatalf("observability.Setup() error = %v", err)
	}

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	producerConfig := DefaultConfig(
		identifier + "-producer",
	)
	producerConfig.Brokers = brokers

	producer, err := NewProducer(
		producerConfig,
		telemetry,
		logger,
	)
	if err != nil {
		t.Fatalf("NewProducer() error = %v", err)
	}
	defer producer.Close()

	consumerConfig := DefaultConfig(
		identifier + "-consumer",
	)
	consumerConfig.Brokers = brokers
	consumerConfig.GroupID = identifier
	consumerConfig.Topics = []string{topic}
	consumerConfig.MaxPollRecords = 1

	consumer, err := NewConsumer(
		consumerConfig,
		telemetry,
		logger,
	)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}
	defer consumer.Close()

	testContext, testCancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer testCancel()

	if err := producer.Ping(testContext); err != nil {
		t.Fatalf("Producer.Ping() error = %v", err)
	}

	if err := consumer.Ping(testContext); err != nil {
		t.Fatalf("Consumer.Ping() error = %v", err)
	}

	received := make(chan Message, 1)
	consumerErrors := make(chan error, 1)

	go func() {
		consumerErrors <- consumer.Run(
			testContext,
			func(
				_ context.Context,
				message Message,
			) error {
				if message.Envelope.GetEventId() != identifier {
					return nil
				}

				select {
				case received <- message:
				default:
				}

				return nil
			},
		)
	}()

	select {
	case <-consumer.Ready():
		t.Log("Kafka consumer received a partition assignment")

	case err := <-consumerErrors:
		if err == nil {
			t.Fatal(
				"Consumer.Run() stopped before partition assignment",
			)
		}

		t.Fatalf(
			"Consumer.Run() failed before partition assignment: %v",
			err,
		)

	case <-time.After(15 * time.Second):
		t.Fatal(
			"timed out waiting for Kafka partition assignment",
		)
	}

	envelope := &commonv1.EventEnvelope{
		EventId:          identifier,
		EventType:        "integration.tested",
		EventVersion:     1,
		TenantId:         "integration",
		AggregateType:    "integration",
		AggregateId:      identifier,
		AggregateVersion: 1,
		OccurredAt:       timestamppb.Now(),
		Producer:         "integration-test",
		Payload:          []byte(identifier),
	}

	if _, err := producer.Publish(
		testContext,
		topic,
		nil,
		envelope,
	); err != nil {
		t.Fatalf("Producer.Publish() error = %v", err)
	}

	select {
	case message := <-received:
		if message.Envelope.GetEventId() != identifier {
			t.Fatalf(
				"event ID = %q, want %q",
				message.Envelope.GetEventId(),
				identifier,
			)
		}

		if message.Topic != topic {
			t.Fatalf(
				"topic = %q, want %q",
				message.Topic,
				topic,
			)
		}

		t.Logf(
			"received Kafka event %s from %s[%d]@%d",
			message.Envelope.GetEventId(),
			message.Topic,
			message.Partition,
			message.Offset,
		)

	case err := <-consumerErrors:
		if err == nil {
			t.Fatal(
				"Consumer.Run() stopped before receiving the event",
			)
		}

		t.Fatalf("Consumer.Run() error = %v", err)

	case <-time.After(25 * time.Second):
		t.Fatal("timed out waiting for Kafka event")
	}
}

func firstIntegrationTopic(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}

	return ""
}
