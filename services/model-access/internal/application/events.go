package application

import (
	"context"
	"fmt"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// connectionEvent builds a deterministic common-envelope outbox event for a
// connection state change. Payloads never contain secret material.
func (service *Service) connectionEvent(
	ctx context.Context,
	eventType string,
	connection domain.Connection,
	payload proto.Message,
	occurredAt time.Time,
) (domain.OutboxEvent, error) {
	eventID, err := uuid.NewV7()
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"generate model event ID: %w",
			err,
		)
	}

	payloadBytes, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"marshal model event payload: %w",
			err,
		)
	}

	requestMetadata, _ := grpcx.RequestMetadataFromContext(ctx)

	traceID := ""

	spanContext := trace.SpanContextFromContext(ctx)

	if spanContext.IsValid() {
		traceID = spanContext.TraceID().String()
	}

	envelope := &commonv1.EventEnvelope{
		EventId:          eventID.String(),
		EventType:        eventType,
		EventVersion:     1,
		TenantId:         connection.TenantID,
		AggregateType:    "model_connection",
		AggregateId:      connection.ID,
		AggregateVersion: uint64(connection.Version),
		OccurredAt:       timestamppb.New(occurredAt),
		Producer:         "model-access",
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
			"marshal model event envelope: %w",
			err,
		)
	}

	return domain.OutboxEvent{
		ID:         eventID.String(),
		Topic:      service.config.EventTopic,
		Key:        connection.ID,
		Envelope:   envelopeBytes,
		OccurredAt: occurredAt,
	}, nil
}
