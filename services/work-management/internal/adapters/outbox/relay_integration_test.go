//go:build integration

package outbox

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestProcessBatchPublishesToKafka(t *testing.T) {
	brokers := splitBrokers(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		t.Skip("KAFKA_BROKERS is not configured")
	}

	topic := firstTopic(
		os.Getenv("KAFKA_INTEGRATION_TOPIC"),
		"gereh.work.events.v1",
	)

	repository := new(relayRepository)

	repository.claim = func(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.OutboxRecord, error) {
		return []domain.OutboxRecord{
			{
				OutboxID: 11,
				Event: domain.OutboxEvent{
					ID:       "event-ok",
					Topic:    topic,
					Key:      "goal-1",
					Envelope: validEnvelope(t),
				},
				Attempts: 1,
			},
		}, nil
	}

	var publishedOutboxID int64

	repository.markPublished = func(
		ctx context.Context,
		outboxID int64,
	) error {
		publishedOutboxID = outboxID

		return nil
	}

	repository.release = func(
		ctx context.Context,
		outboxID int64,
		retryAt time.Time,
		publishError string,
	) error {
		return nil
	}

	relay, err := New(
		Config{
			BatchSize:    5,
			PollInterval: time.Millisecond,
			Lease:        time.Minute,
			MaxBackoff:   30 * time.Second,
		},
		repository,
		integrationProducer(t, brokers),
		nil,
	)
	require.NoError(t, err)

	testContext, testCancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer testCancel()

	processed := relay.processBatch(testContext)
	require.True(t, processed)
	require.Equal(t, int64(11), publishedOutboxID)
}

func integrationProducer(
	t *testing.T,
	brokers []string,
) *kafka.Producer {
	t.Helper()

	telemetryConfig := observability.DefaultConfig(
		"work-outbox-relay-test",
		"dev",
	)
	telemetryConfig.Enabled = false

	telemetry, err := observability.Setup(
		context.Background(),
		telemetryConfig,
	)
	require.NoError(t, err)

	config := kafka.DefaultConfig("work-outbox-relay-test")
	config.Brokers = brokers
	config.RecordDeliveryTimeout = 20 * time.Second

	producer, err := kafka.NewProducer(
		config,
		telemetry,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	require.NoError(t, err)

	t.Cleanup(producer.Close)

	return producer
}

func splitBrokers(value string) []string {
	var brokers []string

	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)

		if item != "" {
			brokers = append(brokers, item)
		}
	}

	return brokers
}

func firstTopic(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}

	return fallback
}
