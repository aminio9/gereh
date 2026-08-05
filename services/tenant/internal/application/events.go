package application

import (
	"context"
	"fmt"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/services/tenant/internal/domain"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newOutboxEvent(
	ctx context.Context,
	topic string,
	eventType string,
	tenantID string,
	aggregateVersion int64,
	payload proto.Message,
	now time.Time,
) (domain.OutboxEvent, error) {
	eventID, err := uuid.NewV7()
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"generate event ID: %w",
			err,
		)
	}

	payloadBytes, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"marshal event payload: %w",
			err,
		)
	}

	requestMetadata, _ := grpcx.RequestMetadataFromContext(
		ctx,
	)

	traceID := ""
	spanContext := trace.SpanContextFromContext(ctx)

	if spanContext.IsValid() {
		traceID = spanContext.TraceID().String()
	}

	envelope := &commonv1.EventEnvelope{
		EventId:          eventID.String(),
		EventType:        eventType,
		EventVersion:     1,
		TenantId:         tenantID,
		AggregateType:    "tenant",
		AggregateId:      tenantID,
		AggregateVersion: uint64(aggregateVersion),
		OccurredAt:       timestamppb.New(now),
		Producer:         "tenant",
		TraceId:          traceID,
		CorrelationId:    requestMetadata.CorrelationID,
		CausationId:      requestMetadata.RequestID,
		Payload:          payloadBytes,
	}

	envelopeBytes, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(envelope)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"marshal event envelope: %w",
			err,
		)
	}

	return domain.OutboxEvent{
		ID:         eventID.String(),
		Topic:      topic,
		Key:        tenantID,
		Envelope:   envelopeBytes,
		OccurredAt: now,
	}, nil
}
