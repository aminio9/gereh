package grpcx

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestRequestMetadataRoundTrip(t *testing.T) {
	expected := RequestMetadata{
		RequestID:     "request-1",
		CorrelationID: "correlation-1",
		ActorUserID:   "0198abc0-0000-7000-8000-000000000001",
		TenantID:      "tenant-1",
	}

	ctx := WithRequestMetadata(
		context.Background(),
		expected,
	)

	ctx = InjectOutgoingMetadata(ctx)

	outgoing, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("outgoing metadata is missing")
	}

	if got := outgoing.Get(RequestIDMetadataKey); len(got) != 1 ||
		got[0] != expected.RequestID {
		t.Fatalf("request ID = %v", got)
	}

	incomingContext := metadata.NewIncomingContext(
		context.Background(),
		outgoing,
	)

	actual := RequestMetadataFromIncoming(incomingContext)

	if actual != expected {
		t.Fatalf(
			"RequestMetadataFromIncoming() = %#v, want %#v",
			actual,
			expected,
		)
	}
}

func TestRequestMetadataGeneratesIdentifiers(t *testing.T) {
	actual := RequestMetadataFromIncoming(
		context.Background(),
	)

	if actual.RequestID == "" {
		t.Fatal("RequestID is empty")
	}

	if actual.CorrelationID != actual.RequestID {
		t.Fatalf(
			"CorrelationID = %q, want RequestID %q",
			actual.CorrelationID,
			actual.RequestID,
		)
	}
}
