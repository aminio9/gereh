package projection

import (
	"testing"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDecodeEventUnknownTypeIsIgnored(t *testing.T) {
	apply, err := decodeEvent(
		"billing.usage.recorded",
		[]byte("{}"),
		domain.EventMeta{},
		time.Now(),
	)
	require.NoError(t, err)
	require.Nil(t, apply)
}

func TestDecodeEventMalformedPayloadFails(t *testing.T) {
	meta := domain.EventMeta{
		TenantID: uuid.NewString(),
	}

	apply, err := decodeEvent(
		"company.created",
		[]byte("not-a-protobuf"),
		meta,
		time.Now(),
	)
	require.Error(t, err)
	require.Nil(t, apply)
}

func TestDecodeEventTenantMismatchFails(t *testing.T) {
	envelopeTenant := uuid.NewString()
	payloadTenant := uuid.NewString()

	company := &organizationv1.Company{
		TenantId:    payloadTenant,
		CompanyId:   uuid.NewString(),
		Slug:        "acme",
		DisplayName: "Acme",
		Status:      organizationv1.CompanyStatus_COMPANY_STATUS_ACTIVE,
	}

	payload, err := proto.Marshal(
		&organizationv1.CompanyCreated{
			Company: company,
		},
	)
	require.NoError(t, err)

	meta := domain.EventMeta{
		TenantID: envelopeTenant,
	}

	apply, err := decodeEvent(
		"company.created",
		payload,
		meta,
		time.Now(),
	)
	require.Error(t, err)
	require.Nil(t, apply)
}

func TestDecodeEventTenantMatchSucceeds(t *testing.T) {
	tenant := uuid.NewString()

	company := &organizationv1.Company{
		TenantId:    tenant,
		CompanyId:   uuid.NewString(),
		Slug:        "acme",
		DisplayName: "Acme",
		Status:      organizationv1.CompanyStatus_COMPANY_STATUS_ACTIVE,
	}

	payload, err := proto.Marshal(
		&organizationv1.CompanyCreated{
			Company: company,
		},
	)
	require.NoError(t, err)

	meta := domain.EventMeta{
		TenantID: tenant,
	}

	apply, err := decodeEvent(
		"company.created",
		payload,
		meta,
		time.Now(),
	)
	require.NoError(t, err)
	require.NotNil(t, apply)
}

func TestEventHashDeterministicAndStable(t *testing.T) {
	envelope := &commonv1.EventEnvelope{
		EventId:      uuid.NewString(),
		EventType:    "task.created",
		EventVersion: 1,
		TenantId:     uuid.NewString(),
		Payload:      []byte("payload"),
	}

	first, err := eventHash(envelope)
	require.NoError(t, err)
	require.Len(t, first, 32)

	second, err := eventHash(envelope)
	require.NoError(t, err)
	require.Equal(t, first, second)

	envelope.Payload = []byte("changed")

	third, err := eventHash(envelope)
	require.NoError(t, err)
	require.NotEqual(t, first, third)
}

func TestBuildEventMetaUsesUTC(t *testing.T) {
	occurredAt := time.Now()

	envelope := &commonv1.EventEnvelope{
		EventId:      uuid.NewString(),
		EventType:    "agent.created",
		EventVersion: 1,
		TenantId:     uuid.NewString(),
		OccurredAt:   timestamppb.New(occurredAt),
	}

	message := platformkafka.Message{
		Topic:     "gereh.organization.agent.events.v1",
		Partition: 2,
		Offset:    99,
		Envelope:  envelope,
	}

	meta, err := buildEventMeta(
		message,
		time.Now(),
	)
	require.NoError(t, err)
	require.Equal(t, "gereh.organization.agent.events.v1", meta.Topic)
	require.EqualValues(t, 2, meta.Partition)
	require.EqualValues(t, 99, meta.Offset)
	require.Equal(t, envelope.GetTenantId(), meta.TenantID)
	require.True(t, meta.OccurredAt.Location() == time.UTC)
}
