package kafka

import (
	"testing"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProtobufCodecRoundTrip(t *testing.T) {
	expected := &commonv1.EventEnvelope{
		EventId:          "event-1",
		EventType:        "tenant.created",
		EventVersion:     1,
		TenantId:         "tenant-1",
		AggregateType:    "tenant",
		AggregateId:      "tenant-1",
		AggregateVersion: 1,
		OccurredAt:       timestamppb.Now(),
		Producer:         "tenant",
		Payload:          []byte("payload"),
		Attributes: map[string]string{
			"region": "eu-central-1",
		},
	}

	codec := ProtobufCodec{}

	value, err := codec.Marshal(expected)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	actual, err := codec.Unmarshal(value)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !proto.Equal(actual, expected) {
		t.Fatalf(
			"round-trip envelope = %#v, want %#v",
			actual,
			expected,
		)
	}
}

func TestValidateEnvelope(t *testing.T) {
	envelope := &commonv1.EventEnvelope{
		EventId:      "event-1",
		EventType:    "tenant.created",
		EventVersion: 1,
	}

	if err := validateEnvelope(envelope); err != nil {
		t.Fatalf("validateEnvelope() error = %v", err)
	}
}
