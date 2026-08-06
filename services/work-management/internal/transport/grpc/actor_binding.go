// Package grpc exposes the Work Management Service over gRPC.
package grpc

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type actorBoundRequest interface {
	GetActorUserId() string
}

// ActorBindingUnaryInterceptor prevents request messages from claiming a user
// ID different from the authenticated internal actor metadata.
//
// Requests without an actor field pass through.
func ActorBindingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		actorRequest, ok :=
			request.(actorBoundRequest)
		if !ok {
			return handler(ctx, request)
		}

		requestActorID := strings.ToLower(
			strings.TrimSpace(
				actorRequest.GetActorUserId(),
			),
		)

		if _, err := uuid.Parse(
			requestActorID,
		); err != nil {
			return nil, status.Error(
				codes.InvalidArgument,
				"actor user ID must be a UUID",
			)
		}

		requestMetadata, ok :=
			grpcx.RequestMetadataFromContext(ctx)

		if !ok ||
			strings.TrimSpace(
				requestMetadata.ActorUserID,
			) == "" {
			return nil, status.Error(
				codes.Unauthenticated,
				"authenticated actor metadata is required",
			)
		}

		metadataActorID := strings.ToLower(
			strings.TrimSpace(
				requestMetadata.ActorUserID,
			),
		)

		if len(metadataActorID) !=
			len(requestActorID) ||
			subtle.ConstantTimeCompare(
				[]byte(metadataActorID),
				[]byte(requestActorID),
			) != 1 {
			return nil, status.Error(
				codes.PermissionDenied,
				"request actor does not match authenticated actor",
			)
		}

		return handler(ctx, request)
	}
}
