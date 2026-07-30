package kafka

import (
	"fmt"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// HeaderContentType identifies the serialized record format.
	HeaderContentType = "content-type"

	// HeaderEventID carries the globally unique Gereh event identifier.
	HeaderEventID = "gereh-event-id"

	// HeaderEventType carries the version-independent event type.
	HeaderEventType = "gereh-event-type"

	// HeaderEventVersion carries the event schema version.
	HeaderEventVersion = "gereh-event-version"

	// HeaderTenantID carries the tenant associated with the event.
	//
	// Consumers must not treat this transport header as authenticated identity.
	HeaderTenantID = "gereh-tenant-id"

	// ProtobufContentType identifies Protocol Buffers record payloads.
	ProtobufContentType = "application/x-protobuf"
)

// Message is a decoded Kafka event with transport metadata.
type Message struct {
	Topic     string
	Key       []byte
	Envelope  *commonv1.EventEnvelope
	Headers   map[string][]string
	Partition int32
	Offset    int64
	Timestamp time.Time
}

// PublishResult identifies the Kafka location of a published event.
type PublishResult struct {
	Topic     string
	Partition int32
	Offset    int64
	Timestamp time.Time
}

func validateEnvelope(
	envelope *commonv1.EventEnvelope,
) error {
	if envelope == nil {
		return fmt.Errorf("event envelope is required")
	}

	if envelope.GetEventId() == "" {
		return fmt.Errorf("event ID is required")
	}

	if envelope.GetEventType() == "" {
		return fmt.Errorf("event type is required")
	}

	if envelope.GetEventVersion() == 0 {
		return fmt.Errorf("event version must be greater than zero")
	}

	if occurredAt := envelope.GetOccurredAt(); occurredAt != nil {
		if err := occurredAt.CheckValid(); err != nil {
			return fmt.Errorf(
				"event occurrence timestamp is invalid: %w",
				err,
			)
		}
	}

	return nil
}

func normalizeEnvelope(
	envelope *commonv1.EventEnvelope,
) {
	if envelope.OccurredAt == nil {
		envelope.OccurredAt = timestamppb.Now()
	}
}

func eventRecordHeaders(
	envelope *commonv1.EventEnvelope,
) []kgo.RecordHeader {
	return []kgo.RecordHeader{
		{
			Key:   HeaderContentType,
			Value: []byte(ProtobufContentType),
		},
		{
			Key:   HeaderEventID,
			Value: []byte(envelope.GetEventId()),
		},
		{
			Key:   HeaderEventType,
			Value: []byte(envelope.GetEventType()),
		},
		{
			Key: HeaderEventVersion,
			Value: []byte(
				fmt.Sprintf("%d", envelope.GetEventVersion()),
			),
		},
		{
			Key:   HeaderTenantID,
			Value: []byte(envelope.GetTenantId()),
		},
	}
}

func recordHeaders(
	headers []kgo.RecordHeader,
) map[string][]string {
	values := make(map[string][]string)

	for _, header := range headers {
		values[header.Key] = append(
			values[header.Key],
			string(header.Value),
		)
	}

	return values
}
