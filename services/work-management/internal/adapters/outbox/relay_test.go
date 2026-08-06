package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type relayRepository struct {
	ports.Repository

	claim func(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.OutboxRecord, error)

	markPublished func(
		ctx context.Context,
		outboxID int64,
	) error

	release func(
		ctx context.Context,
		outboxID int64,
		retryAt time.Time,
		publishError string,
	) error
}

func (repository *relayRepository) ClaimOutbox(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]domain.OutboxRecord, error) {
	return repository.claim(ctx, limit, lease)
}

func (repository *relayRepository) MarkOutboxPublished(
	ctx context.Context,
	outboxID int64,
) error {
	return repository.markPublished(ctx, outboxID)
}

func (repository *relayRepository) ReleaseOutbox(
	ctx context.Context,
	outboxID int64,
	retryAt time.Time,
	publishError string,
) error {
	return repository.release(ctx, outboxID, retryAt, publishError)
}

func newTestProducer(t *testing.T) *kafka.Producer {
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
	config.Brokers = []string{"127.0.0.1:1"}

	producer, err := kafka.NewProducer(
		config,
		telemetry,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	require.NoError(t, err)

	t.Cleanup(producer.Close)

	return producer
}

func newTestRelay(t *testing.T, repository ports.Repository) *Relay {
	t.Helper()

	relay, err := New(
		Config{
			BatchSize:    5,
			PollInterval: time.Millisecond,
			Lease:        time.Minute,
			MaxBackoff:   30 * time.Second,
		},
		repository,
		newTestProducer(t),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	require.NoError(t, err)

	return relay
}

func validEnvelope(t *testing.T) []byte {
	t.Helper()

	envelope, err := proto.Marshal(&commonv1.EventEnvelope{
		EventId:      "event-1",
		EventType:    "gereh.work.v1.GoalCreated",
		EventVersion: 1,
		AggregateId:  "goal-1",
		OccurredAt:   timestamppb.Now(),
		Producer:     "work-management",
	})
	require.NoError(t, err)

	return envelope
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	repository := new(relayRepository)
	producer := newTestProducer(t)

	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "zero batch size",
			config: Config{
				BatchSize:    0,
				PollInterval: time.Second,
				Lease:        time.Minute,
				MaxBackoff:   30 * time.Second,
			},
		},
		{
			name: "zero poll interval",
			config: Config{
				BatchSize:    1,
				PollInterval: 0,
				Lease:        time.Minute,
				MaxBackoff:   30 * time.Second,
			},
		},
		{
			name: "zero lease",
			config: Config{
				BatchSize:    1,
				PollInterval: time.Second,
				Lease:        0,
				MaxBackoff:   30 * time.Second,
			},
		},
		{
			name: "zero max backoff",
			config: Config{
				BatchSize:    1,
				PollInterval: time.Second,
				Lease:        time.Minute,
				MaxBackoff:   0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			relay, err := New(
				test.config,
				repository,
				producer,
				nil,
			)
			require.Error(t, err)
			require.Nil(t, relay)
		})
	}
}

func TestNewRejectsMissingDependencies(t *testing.T) {
	config := Config{
		BatchSize:    5,
		PollInterval: time.Second,
		Lease:        time.Minute,
		MaxBackoff:   30 * time.Second,
	}

	relay, err := New(config, nil, newTestProducer(t), nil)
	require.Error(t, err)
	require.Nil(t, relay)

	relay, err = New(config, new(relayRepository), nil, nil)
	require.Error(t, err)
	require.Nil(t, relay)
}

func TestProcessBatchReleasesUndecodableEnvelope(t *testing.T) {
	repository := new(relayRepository)

	repository.claim = func(
		_ context.Context,
		_ int,
		_ time.Duration,
	) ([]domain.OutboxRecord, error) {
		return []domain.OutboxRecord{
			{
				OutboxID: 7,
				Event: domain.OutboxEvent{
					ID:       "event-garbage",
					Topic:    "gereh.work.events.v1",
					Envelope: []byte("not a protobuf"),
				},
				Attempts: 2,
			},
		}, nil
	}

	var releasedOutboxID int64
	var releasedRetryAt time.Time
	var releasedError string

	repository.release = func(
		_ context.Context,
		outboxID int64,
		retryAt time.Time,
		publishError string,
	) error {
		releasedOutboxID = outboxID
		releasedRetryAt = retryAt
		releasedError = publishError

		return nil
	}

	relay := newTestRelay(t, repository)

	processed := relay.processBatch(context.Background())
	require.True(t, processed)
	require.Equal(t, int64(7), releasedOutboxID)
	require.Contains(t, releasedError, "decode outbox envelope")
	require.True(t, releasedRetryAt.After(time.Now()))
}

func TestProcessBatchReleasesPublishError(t *testing.T) {
	repository := new(relayRepository)

	repository.claim = func(
		_ context.Context,
		_ int,
		_ time.Duration,
	) ([]domain.OutboxRecord, error) {
		return []domain.OutboxRecord{
			{
				OutboxID: 9,
				Event: domain.OutboxEvent{
					ID:       "event-no-topic",
					Topic:    "",
					Key:      "goal-1",
					Envelope: validEnvelope(t),
				},
				Attempts: 0,
			},
		}, nil
	}

	var releasedOutboxID int64
	var releasedError string

	repository.release = func(
		_ context.Context,
		outboxID int64,
		_ time.Time,
		publishError string,
	) error {
		releasedOutboxID = outboxID
		releasedError = publishError

		return nil
	}

	relay := newTestRelay(t, repository)

	processed := relay.processBatch(context.Background())
	require.True(t, processed)
	require.Equal(t, int64(9), releasedOutboxID)
	require.Contains(t, releasedError, "kafka topic is required")
}

func TestProcessBatchClaimError(t *testing.T) {
	repository := new(relayRepository)

	claimError := errors.New("claim failed")

	repository.claim = func(
		_ context.Context,
		_ int,
		_ time.Duration,
	) ([]domain.OutboxRecord, error) {
		return nil, claimError
	}

	relay := newTestRelay(t, repository)

	require.False(t, relay.processBatch(context.Background()))
}

func TestProcessBatchReleaseErrorDoesNotAbortBatch(t *testing.T) {
	repository := new(relayRepository)

	repository.claim = func(
		_ context.Context,
		_ int,
		_ time.Duration,
	) ([]domain.OutboxRecord, error) {
		return []domain.OutboxRecord{
			{
				OutboxID: 13,
				Event: domain.OutboxEvent{
					ID:       "event-bad",
					Topic:    "",
					Key:      "goal-1",
					Envelope: validEnvelope(t),
				},
				Attempts: 0,
			},
		}, nil
	}

	repository.release = func(
		_ context.Context,
		_ int64,
		_ time.Time,
		_ string,
	) error {
		return errors.New("release failed")
	}

	relay := newTestRelay(t, repository)

	require.True(t, relay.processBatch(context.Background()))
}
