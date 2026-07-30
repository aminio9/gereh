package kafka

import (
	"fmt"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	"google.golang.org/protobuf/proto"
)

// Codec serializes and deserializes event envelopes.
type Codec interface {
	Marshal(envelope *commonv1.EventEnvelope) ([]byte, error)
	Unmarshal(value []byte) (*commonv1.EventEnvelope, error)
}

// ProtobufCodec encodes EventEnvelope messages using Protocol Buffers.
type ProtobufCodec struct{}

// Marshal serializes an event envelope deterministically.
func (ProtobufCodec) Marshal(
	envelope *commonv1.EventEnvelope,
) ([]byte, error) {
	if envelope == nil {
		return nil, fmt.Errorf("event envelope is nil")
	}

	value, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal event envelope: %w", err)
	}

	return value, nil
}

// Unmarshal deserializes an event envelope.
func (ProtobufCodec) Unmarshal(
	value []byte,
) (*commonv1.EventEnvelope, error) {
	envelope := new(commonv1.EventEnvelope)

	if err := proto.Unmarshal(value, envelope); err != nil {
		return nil, fmt.Errorf("unmarshal event envelope: %w", err)
	}

	return envelope, nil
}
