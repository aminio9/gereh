package grpcx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"google.golang.org/grpc/metadata"
)

const (
	// RequestIDMetadataKey carries the identifier of the current request.
	RequestIDMetadataKey = "x-request-id"

	// CorrelationIDMetadataKey connects related requests across services.
	CorrelationIDMetadataKey = "x-correlation-id"

	// TenantIDMetadataKey carries the tenant routing context.
	//
	// This value must not be treated as authenticated identity until an
	// authorization layer has validated or replaced it.
	TenantIDMetadataKey = "x-tenant-id"
)

type requestMetadataContextKey struct{}

// RequestMetadata contains bounded internal request-routing metadata.
//
// TenantID must only be trusted after authentication and authorization layers
// have validated or replaced the incoming value.
type RequestMetadata struct {
	RequestID     string
	CorrelationID string
	TenantID      string
}

// WithRequestMetadata stores request metadata in a context.
func WithRequestMetadata(
	ctx context.Context,
	requestMetadata RequestMetadata,
) context.Context {
	return context.WithValue(
		ctx,
		requestMetadataContextKey{},
		requestMetadata,
	)
}

// RequestMetadataFromContext retrieves request metadata.
func RequestMetadataFromContext(
	ctx context.Context,
) (RequestMetadata, bool) {
	requestMetadata, ok := ctx.Value(
		requestMetadataContextKey{},
	).(RequestMetadata)

	return requestMetadata, ok
}

// RequestMetadataFromIncoming extracts supported incoming gRPC metadata.
func RequestMetadataFromIncoming(
	ctx context.Context,
) RequestMetadata {
	incoming, _ := metadata.FromIncomingContext(ctx)

	requestMetadata := RequestMetadata{
		RequestID: firstMetadataValue(
			incoming,
			RequestIDMetadataKey,
		),
		CorrelationID: firstMetadataValue(
			incoming,
			CorrelationIDMetadataKey,
		),
		TenantID: firstMetadataValue(
			incoming,
			TenantIDMetadataKey,
		),
	}

	if requestMetadata.RequestID == "" {
		requestMetadata.RequestID = NewRequestID()
	}

	if requestMetadata.CorrelationID == "" {
		requestMetadata.CorrelationID = requestMetadata.RequestID
	}

	return requestMetadata
}

// InjectOutgoingMetadata adds request metadata to an outgoing gRPC context.
func InjectOutgoingMetadata(ctx context.Context) context.Context {
	requestMetadata, ok := RequestMetadataFromContext(ctx)
	if !ok {
		return ctx
	}

	outgoing, _ := metadata.FromOutgoingContext(ctx)
	outgoing = outgoing.Copy()

	if requestMetadata.RequestID != "" {
		outgoing.Set(
			RequestIDMetadataKey,
			requestMetadata.RequestID,
		)
	}

	if requestMetadata.CorrelationID != "" {
		outgoing.Set(
			CorrelationIDMetadataKey,
			requestMetadata.CorrelationID,
		)
	}

	if requestMetadata.TenantID != "" {
		outgoing.Set(
			TenantIDMetadataKey,
			requestMetadata.TenantID,
		)
	}

	return metadata.NewOutgoingContext(ctx, outgoing)
}

// NewRequestID creates a cryptographically random request identifier.
func NewRequestID() string {
	value := make([]byte, 16)

	if _, err := rand.Read(value); err == nil {
		return hex.EncodeToString(value)
	}

	return strconv.FormatInt(
		time.Now().UTC().UnixNano(),
		36,
	)
}

func firstMetadataValue(
	values metadata.MD,
	key string,
) string {
	items := values.Get(key)

	if len(items) == 0 {
		return ""
	}

	return items[0]
}
