package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel/codes"
)

// Handler processes one decoded event.
//
// Implementations must be idempotent because Kafka delivery is at least once.
type Handler func(ctx context.Context, message Message) error

// Consumer consumes and commits Gereh events.
type Consumer struct {
	client         *kgo.Client
	codec          Codec
	tracer         *kotel.Tracer
	maxPollRecords int
	ready          <-chan struct{}
}

// NewConsumer creates an instrumented Kafka group consumer.
func NewConsumer(
	config Config,
	telemetry *observability.Telemetry,
	logger *slog.Logger,
) (*Consumer, error) {
	if err := config.ValidateConsumer(); err != nil {
		return nil, fmt.Errorf(
			"validate Kafka consumer configuration: %w",
			err,
		)
	}

	options, tracer, err := baseClientOptions(
		config,
		telemetry,
		logger,
	)
	if err != nil {
		return nil, err
	}

	ready := make(chan struct{})
	var readyOnce sync.Once

	resetOffset := kgo.NewOffset().AtEnd()

	if config.ConsumerStartOffset ==
		ConsumerStartOffsetEarliest {
		resetOffset = kgo.NewOffset().AtStart()
	}

	options = append(
		options,
		kgo.ConsumerGroup(config.GroupID),
		kgo.ConsumeTopics(config.Topics...),
		kgo.ConsumeResetOffset(resetOffset),
		kgo.DisableAutoCommit(),
		kgo.Balancers(
			kgo.CooperativeStickyBalancer(),
		),
		kgo.BlockRebalanceOnPoll(),
		kgo.SessionTimeout(config.SessionTimeout),
		kgo.HeartbeatInterval(config.HeartbeatInterval),
		kgo.OnPartitionsAssigned(
			func(
				_ context.Context,
				_ *kgo.Client,
				partitions map[string][]int32,
			) {
				for _, assignedPartitions := range partitions {
					if len(assignedPartitions) == 0 {
						continue
					}

					readyOnce.Do(func() {
						close(ready)
					})

					return
				}
			},
		),
	)

	client, err := kgo.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}

	return &Consumer{
		client:         client,
		codec:          ProtobufCodec{},
		tracer:         tracer,
		maxPollRecords: config.MaxPollRecords,
		ready:          ready,
	}, nil
}

// Ready returns a channel that closes after the consumer receives its first
// non-empty partition assignment.
//
// Run must be active for group assignment to occur because polling starts the
// consumer-group join process.
func (consumer *Consumer) Ready() <-chan struct{} {
	return consumer.ready
}

// Ping verifies that the Kafka cluster is reachable.
func (consumer *Consumer) Ping(ctx context.Context) error {
	if err := consumer.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping Kafka: %w", err)
	}

	return nil
}

// Run polls, processes, and manually commits records.
//
// Run returns when the context is canceled or when processing fails.
func (consumer *Consumer) Run(
	ctx context.Context,
	handler Handler,
) error {
	if handler == nil {
		return fmt.Errorf("kafka consumer handler is required")
	}

	for {
		fetches := consumer.client.PollRecords(
			ctx,
			consumer.maxPollRecords,
		)

		select {
		case <-ctx.Done():
			consumer.client.AllowRebalance()

			return nil
		default:
		}

		if err := fetchErrors(fetches); err != nil {
			consumer.client.AllowRebalance()

			return err
		}

		records := fetches.Records()

		if len(records) == 0 {
			consumer.client.AllowRebalance()

			continue
		}

		for _, record := range records {
			processContext, span := consumer.tracer.WithProcessSpan(
				record,
			)

			message, err := consumer.decodeRecord(record)
			if err == nil {
				err = handler(processContext, message)
			}

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.End()

				consumer.client.AllowRebalance()

				return fmt.Errorf(
					"process Kafka record %s[%d]@%d: %w",
					record.Topic,
					record.Partition,
					record.Offset,
					err,
				)
			}

			if err := consumer.client.CommitRecords(
				processContext,
				record,
			); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.End()

				consumer.client.AllowRebalance()

				return fmt.Errorf(
					"commit Kafka record %s[%d]@%d: %w",
					record.Topic,
					record.Partition,
					record.Offset,
					err,
				)
			}

			span.End()
		}

		consumer.client.AllowRebalance()
	}
}

// Close allows a final rebalance and closes the consumer.
func (consumer *Consumer) Close() {
	if consumer == nil || consumer.client == nil {
		return
	}

	consumer.client.CloseAllowingRebalance()
}

func (consumer *Consumer) decodeRecord(
	record *kgo.Record,
) (Message, error) {
	envelope, err := consumer.codec.Unmarshal(record.Value)
	if err != nil {
		return Message{}, err
	}

	if err := validateEnvelope(envelope); err != nil {
		return Message{}, err
	}

	return Message{
		Topic:     record.Topic,
		Key:       append([]byte(nil), record.Key...),
		Envelope:  envelope,
		Headers:   recordHeaders(record.Headers),
		Partition: record.Partition,
		Offset:    record.Offset,
		Timestamp: record.Timestamp,
	}, nil
}

func fetchErrors(fetches kgo.Fetches) error {
	var errorsFound []error

	for _, fetchError := range fetches.Errors() {
		if errors.Is(fetchError.Err, context.Canceled) {
			continue
		}

		errorsFound = append(
			errorsFound,
			fmt.Errorf(
				"fetch Kafka partition %s[%d]: %w",
				fetchError.Topic,
				fetchError.Partition,
				fetchError.Err,
			),
		)
	}

	return errors.Join(errorsFound...)
}
