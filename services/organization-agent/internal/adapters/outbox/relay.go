// Package outbox relays committed Company and Agent Service events to Kafka.
package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/services/organization-agent/internal/ports"
	"google.golang.org/protobuf/proto"
)

// Config defines outbox relay behavior.
type Config struct {
	BatchSize    int
	PollInterval time.Duration
	Lease        time.Duration
	MaxBackoff   time.Duration
}

// Relay publishes committed outbox events to Kafka.
type Relay struct {
	config     Config
	repository ports.Repository
	producer   *platformkafka.Producer
	logger     *slog.Logger
}

// New creates an outbox relay.
func New(
	config Config,
	repository ports.Repository,
	producer *platformkafka.Producer,
	logger *slog.Logger,
) (*Relay, error) {
	if config.BatchSize <= 0 {
		return nil, fmt.Errorf(
			"outbox batch size must be greater than zero",
		)
	}

	if config.PollInterval <= 0 ||
		config.Lease <= 0 ||
		config.MaxBackoff <= 0 {
		return nil, fmt.Errorf(
			"outbox durations must be greater than zero",
		)
	}

	if repository == nil || producer == nil {
		return nil, fmt.Errorf(
			"outbox repository and producer are required",
		)
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

// Run processes outbox records until the context is cancelled.
func (relay *Relay) Run(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			processed := relay.processBatch(ctx)

			delay := relay.config.PollInterval
			if processed {
				delay = 0
			}

			timer.Reset(delay)
		}
	}
}

func (relay *Relay) processBatch(
	ctx context.Context,
) bool {
	records, err := relay.repository.ClaimOutbox(
		ctx,
		relay.config.BatchSize,
		relay.config.Lease,
	)
	if err != nil {
		relay.logger.ErrorContext(
			ctx,
			"claim organization outbox batch",
			"error",
			err,
		)

		return false
	}

	for _, record := range records {
		envelope := new(commonv1.EventEnvelope)

		if err := proto.Unmarshal(
			record.Event.Envelope,
			envelope,
		); err != nil {
			relay.release(
				ctx,
				record.OutboxID,
				record.Attempts,
				fmt.Errorf(
					"decode outbox envelope: %w",
					err,
				),
			)

			continue
		}

		_, err := relay.producer.Publish(
			ctx,
			record.Event.Topic,
			[]byte(record.Event.Key),
			envelope,
		)
		if err != nil {
			relay.release(
				ctx,
				record.OutboxID,
				record.Attempts,
				err,
			)

			continue
		}

		if err := relay.repository.MarkOutboxPublished(
			ctx,
			record.OutboxID,
		); err != nil {
			relay.logger.ErrorContext(
				ctx,
				"mark organization event published",
				"outbox_id",
				record.OutboxID,
				"event_id",
				record.Event.ID,
				"error",
				err,
			)
		}
	}

	return len(records) > 0
}

func (relay *Relay) release(
	ctx context.Context,
	outboxID int64,
	attempts int,
	publishError error,
) {
	exponent := math.Min(
		float64(attempts),
		10,
	)

	backoff := time.Second *
		time.Duration(math.Pow(2, exponent))

	if backoff > relay.config.MaxBackoff {
		backoff = relay.config.MaxBackoff
	}

	err := relay.repository.ReleaseOutbox(
		ctx,
		outboxID,
		time.Now().UTC().Add(backoff),
		publishError.Error(),
	)
	if err != nil {
		relay.logger.ErrorContext(
			ctx,
			"release organization outbox event",
			"outbox_id",
			outboxID,
			"error",
			err,
		)
	}
}
