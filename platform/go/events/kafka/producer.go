package kafka

import (
	"context"
	"fmt"
	"log/slog"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// Producer publishes Gereh event envelopes.
type Producer struct {
	client *kgo.Client
	codec  Codec
}

// NewProducer creates an instrumented, idempotent Kafka producer.
func NewProducer(
	config Config,
	telemetry *observability.Telemetry,
	logger *slog.Logger,
) (*Producer, error) {
	if err := config.ValidateCommon(); err != nil {
		return nil, fmt.Errorf(
			"validate Kafka producer configuration: %w",
			err,
		)
	}

	options, _, err := baseClientOptions(
		config,
		telemetry,
		logger,
	)
	if err != nil {
		return nil, err
	}

	options = append(
		options,
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(
			kgo.ZstdCompression(),
		),
		kgo.ProducerLinger(config.ProducerLinger),
		kgo.ProducerBatchMaxBytes(
			config.ProducerBatchMaxBytes,
		),
		kgo.RecordDeliveryTimeout(
			config.RecordDeliveryTimeout,
		),
	)

	client, err := kgo.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create Kafka producer: %w", err)
	}

	return &Producer{
		client: client,
		codec:  ProtobufCodec{},
	}, nil
}

// Ping verifies that the Kafka cluster is reachable.
func (producer *Producer) Ping(ctx context.Context) error {
	if err := producer.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping Kafka: %w", err)
	}

	return nil
}

// Publish synchronously publishes an event.
//
// A nil key uses aggregate_id when available, otherwise event_id.
func (producer *Producer) Publish(
	ctx context.Context,
	topic string,
	key []byte,
	envelope *commonv1.EventEnvelope,
) (PublishResult, error) {
	if topic == "" {
		return PublishResult{}, fmt.Errorf("kafka topic is required")
	}

	if err := validateEnvelope(envelope); err != nil {
		return PublishResult{}, err
	}

	clonedEnvelope, ok := proto.Clone(
		envelope,
	).(*commonv1.EventEnvelope)
	if !ok {
		return PublishResult{}, fmt.Errorf(
			"clone event envelope",
		)
	}

	normalizeEnvelope(clonedEnvelope)

	spanContext := trace.SpanContextFromContext(ctx)
	if clonedEnvelope.TraceId == "" && spanContext.IsValid() {
		clonedEnvelope.TraceId = spanContext.TraceID().String()
	}

	value, err := producer.codec.Marshal(clonedEnvelope)
	if err != nil {
		return PublishResult{}, err
	}

	if len(key) == 0 {
		switch {
		case clonedEnvelope.GetAggregateId() != "":
			key = []byte(clonedEnvelope.GetAggregateId())
		default:
			key = []byte(clonedEnvelope.GetEventId())
		}
	}

	record := &kgo.Record{
		Topic:   topic,
		Key:     append([]byte(nil), key...),
		Value:   value,
		Headers: eventRecordHeaders(clonedEnvelope),
		Context: ctx,
	}

	result := producer.client.ProduceSync(ctx, record)

	if err := result.FirstErr(); err != nil {
		return PublishResult{}, fmt.Errorf(
			"publish event %q to %q: %w",
			clonedEnvelope.GetEventId(),
			topic,
			err,
		)
	}

	return PublishResult{
		Topic:     record.Topic,
		Partition: record.Partition,
		Offset:    record.Offset,
		Timestamp: record.Timestamp,
	}, nil
}

// Close flushes buffered records and closes the producer.
func (producer *Producer) Close() {
	if producer == nil || producer.client == nil {
		return
	}

	producer.client.Close()
}
