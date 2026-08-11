// Package outbox relays Model Access events to Kafka.
package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"google.golang.org/protobuf/proto"
)

// Config configures the relay loop.
type Config struct {
	BatchSize int

	PollInterval time.Duration
	Lease        time.Duration
	MaxBackoff   time.Duration
}

// Relay publishes transactional outbox events to Kafka.
type Relay struct {
	config Config

	repository ports.Repository
	producer   *platformkafka.Producer

	logger *slog.Logger
}

// New validates and constructs the relay.
func New(
	config Config,
	repository ports.Repository,
	producer *platformkafka.Producer,
	logger *slog.Logger,
) (*Relay, error) {
	if config.BatchSize <= 0 {
		return nil, fmt.Errorf("outbox batch size must be positive")
	}

	if config.PollInterval <= 0 ||
		config.Lease <= 0 ||
		config.MaxBackoff <= 0 {
		return nil, fmt.Errorf("outbox durations must be positive")
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Relay{
		config:     config,
		repository: repository,
		producer:   producer,
		logger:     logger,
	}, nil
}

// Run polls and publishes until the context is canceled.
func (relay *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(relay.config.PollInterval)
	defer ticker.Stop()

	for {
		if err := relay.publishBatch(ctx); err != nil {
			relay.logger.ErrorContext(
				ctx,
				"Model Access outbox batch failed",
				"error",
				err,
			)
		}

		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
		}
	}
}

func (relay *Relay) publishBatch(ctx context.Context) error {
	records, err := relay.repository.ClaimOutbox(
		ctx,
		relay.config.BatchSize,
		relay.config.Lease,
	)
	if err != nil {
		return err
	}

	for _, record := range records {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := relay.publishOne(ctx, record); err != nil {
			relay.logger.WarnContext(
				ctx,
				"Model Access event publish failed",
				"event_id",
				record.EventID,
				"attempts",
				record.Attempts,
				"error",
				err,
			)
		}
	}

	return nil
}

func (relay *Relay) publishOne(
	ctx context.Context,
	record domain.OutboxRecord,
) error {
	envelope := new(commonv1.EventEnvelope)

	if err := proto.Unmarshal(record.Envelope, envelope); err != nil {
		return relay.release(
			ctx,
			record,
			fmt.Errorf("decode outbox envelope: %w", err),
		)
	}

	_, err := relay.producer.Publish(
		ctx,
		record.Topic,
		[]byte(record.Key),
		envelope,
	)
	if err != nil {
		return relay.release(ctx, record, err)
	}

	return relay.repository.MarkOutboxPublished(ctx, record.OutboxID)
}

func (relay *Relay) release(
	ctx context.Context,
	record domain.OutboxRecord,
	publishError error,
) error {
	exponent := min(record.Attempts-1, 8)

	backoff := time.Duration(math.Pow(2, float64(exponent))) * time.Second

	if backoff > relay.config.MaxBackoff {
		backoff = relay.config.MaxBackoff
	}

	retryAt := time.Now().UTC().Add(backoff)

	if err := relay.repository.ReleaseOutbox(
		ctx,
		record.OutboxID,
		retryAt,
		publishError.Error(),
	); err != nil {
		return fmt.Errorf(
			"release failed Model Access event: %w",
			err,
		)
	}

	return publishError
}
