// Package projection implements the Kafka-backed Projection worker.
package projection

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/services/projection/internal/application"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/proto"
)

// Worker consumes Gereh domain events and projects them into read models.
type Worker struct {
	consumer *platformkafka.Consumer
	service  *application.Service
	logger   *slog.Logger
	metrics  *Metrics
	now      func() time.Time
}

// NewWorker creates the projection event worker.
//
// The meter is optional; a nil meter records no custom metrics.
func NewWorker(
	consumer *platformkafka.Consumer,
	service *application.Service,
	logger *slog.Logger,
	meter metric.Meter,
) (*Worker, error) {
	metrics, err := NewMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf(
			"create projection metrics: %w",
			err,
		)
	}

	return &Worker{
		consumer: consumer,
		service:  service,
		logger:   logger,
		metrics:  metrics,
		now:      time.Now,
	}, nil
}

// Run consumes and projects events until the context is canceled.
func (worker *Worker) Run(ctx context.Context) error {
	return worker.consumer.Run(
		ctx,
		worker.handle,
	)
}

func (worker *Worker) handle(
	ctx context.Context,
	message platformkafka.Message,
) error {
	envelope := message.Envelope

	projectedAt := worker.now().UTC()

	eventType := envelope.GetEventType()

	worker.metrics.Consumed(eventType)

	eventMeta, err := buildEventMeta(
		message,
		projectedAt,
	)
	if err != nil {
		worker.metrics.Failed(eventType)
		return err
	}

	if !eventMeta.OccurredAt.IsZero() {
		worker.metrics.ObserveLag(
			projectedAt.Sub(
				eventMeta.OccurredAt,
			),
			eventType,
		)
	}

	apply, err := decodeEvent(
		eventType,
		envelope.GetPayload(),
		eventMeta,
		projectedAt,
	)
	if err != nil {
		worker.metrics.Failed(eventType)
		return fmt.Errorf(
			"decode event %q: %w",
			eventType,
			err,
		)
	}

	started := worker.now()

	if apply == nil {
		// Unknown but well-formed event: checkpoint and ignore.
		worker.metrics.Unknown(eventType)
		return nil
	}

	applied, err := worker.service.Project(
		ctx,
		eventMeta,
		apply,
	)
	if err != nil {
		if errors.Is(
			err,
			domain.ErrEventIdentityConflict,
		) {
			worker.metrics.IdentityConflict()
		}

		worker.metrics.Failed(eventType)
		return err
	}

	worker.metrics.ObserveApplyDuration(
		worker.now().Sub(started).Seconds(),
		eventType,
	)

	if applied {
		worker.metrics.Applied(eventType)
	} else {
		worker.metrics.Duplicate(eventType)
	}

	return nil
}

func buildEventMeta(
	message platformkafka.Message,
	projectedAt time.Time,
) (domain.EventMeta, error) {
	envelope := message.Envelope

	var occurredAt time.Time

	if value := envelope.GetOccurredAt(); value != nil {
		if err := value.CheckValid(); err != nil {
			return domain.EventMeta{}, fmt.Errorf(
				"invalid event occurrence timestamp: %w",
				err,
			)
		}

		occurredAt = value.AsTime().UTC()
	}

	hash, err := eventHash(envelope)
	if err != nil {
		return domain.EventMeta{}, err
	}

	return domain.EventMeta{
		EventID:  envelope.GetEventId(),
		TenantID: envelope.GetTenantId(),

		Topic:     message.Topic,
		Partition: message.Partition,
		Offset:    message.Offset,

		EventType:    envelope.GetEventType(),
		EventVersion: envelope.GetEventVersion(),

		AggregateType:    envelope.GetAggregateType(),
		AggregateID:      envelope.GetAggregateId(),
		AggregateVersion: envelope.GetAggregateVersion(),

		EventHash: hash,

		OccurredAt:  occurredAt,
		ProcessedAt: projectedAt,
	}, nil
}

// eventHash computes a stable content hash used to detect event-ID reuse
// with different content.
func eventHash(
	envelope *commonv1.EventEnvelope,
) ([]byte, error) {
	value, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal event envelope for hashing: %w",
			err,
		)
	}

	sum := sha256.Sum256(value)

	return sum[:], nil
}
