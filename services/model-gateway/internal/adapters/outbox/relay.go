// Package outbox implements transactional outbox relay to Kafka for Model Gateway.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	modelpostgres "github.com/aminio9/gereh/services/model-gateway/internal/adapters/postgres"
	"google.golang.org/protobuf/proto"
)

// Config configures the Model Gateway outbox relay.
type Config struct {
	BatchSize    int
	PollInterval time.Duration
	Lease        time.Duration
	MaxBackoff   time.Duration
}

// Relay polls the transactional outbox table and publishes events to Kafka.
type Relay struct {
	config     Config
	repository *modelpostgres.Repository
	producer   *platformkafka.Producer
	logger     *slog.Logger
}

// New creates a new outbox relay worker.
func New(
	config Config,
	repository *modelpostgres.Repository,
	producer *platformkafka.Producer,
	logger *slog.Logger,
) (*Relay, error) {
	if repository == nil {
		return nil, errors.New("model gateway repository is required")
	}
	if producer == nil {
		return nil, errors.New("kafka producer is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}

	return &Relay{
		config:     config,
		repository: repository,
		producer:   producer,
		logger:     logger,
	}, nil
}

// Run starts the background outbox polling loop until context cancellation.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processBatch(ctx)
		}
	}
}

func (r *Relay) processBatch(ctx context.Context) {
	records, err := r.repository.ClaimOutbox(ctx, r.config.BatchSize, r.config.Lease)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			r.logger.Warn("claim model gateway outbox failed", "error", err)
		}
		return
	}

	for _, record := range records {
		if err := r.publishRecord(ctx, record); err != nil {
			r.logger.Warn("publish outbox message failed", "event_id", record.EventID, "error", err)
			backoff := time.Duration(1<<record.Attempts) * time.Second
			if backoff > r.config.MaxBackoff {
				backoff = r.config.MaxBackoff
			}
			_ = r.repository.ReleaseOutbox(ctx, record.OutboxID, time.Now().Add(backoff), err.Error())
			continue
		}

		if err := r.repository.MarkOutboxPublished(ctx, record.OutboxID); err != nil {
			r.logger.Warn("mark outbox published failed", "outbox_id", record.OutboxID, "error", err)
		}
	}
}

func (r *Relay) publishRecord(ctx context.Context, record modelpostgres.OutboxRecord) error {
	envelope := new(commonv1.EventEnvelope)
	if err := proto.Unmarshal(record.Envelope, envelope); err != nil {
		return fmt.Errorf("decode outbox envelope: %w", err)
	}

	_, err := r.producer.Publish(
		ctx,
		record.Topic,
		[]byte(record.PartitionKey),
		envelope,
	)
	if err != nil {
		return fmt.Errorf("publish to kafka: %w", err)
	}
	return nil
}
