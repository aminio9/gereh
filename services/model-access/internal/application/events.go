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

func (service *Service) catalogRefreshedEvent(
	ctx context.Context,
	tenantID string,
	payload proto.Message,
	occurredAt time.Time,
) (domain.OutboxEvent, error) {
	eventID, err := uuid.NewV7()
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"generate catalog refreshed event ID: %w",
			err,
		)
	}

	payloadBytes, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"marshal catalog refreshed event payload: %w",
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
		EventType:        "model.catalog.refreshed",
		EventVersion:     1,
		TenantId:         tenantID,
		AggregateType:    "model_catalog",
		AggregateId:      tenantID,
		AggregateVersion: 1,
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
			"marshal catalog refreshed event envelope: %w",
			err,
		)
	}

	return domain.OutboxEvent{
		ID:         eventID.String(),
		Topic:      service.config.EventTopic,
		Key:        tenantID,
		Envelope:   envelopeBytes,
		OccurredAt: occurredAt,
	}, nil
}

func (service *Service) bindingEvent(
	ctx context.Context,
	eventType string,
	binding domain.AgentModelBinding,
	payload proto.Message,
	occurredAt time.Time,
) (domain.OutboxEvent, error) {
	eventID, err := uuid.NewV7()
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"generate model binding event ID: %w",
			err,
		)
	}

	payloadBytes, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf(
			"marshal model binding event payload: %w",
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
		TenantId:         binding.TenantID,
		AggregateType:    "agent_model_binding",
		AggregateId:      binding.AgentID,
		AggregateVersion: uint64(binding.Version),
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
			"marshal model binding event envelope: %w",
			err,
		)
	}

	return domain.OutboxEvent{
		ID:         eventID.String(),
		Topic:      service.config.EventTopic,
		Key:        binding.AgentID,
		Envelope:   envelopeBytes,
		OccurredAt: occurredAt,
	}, nil
}

