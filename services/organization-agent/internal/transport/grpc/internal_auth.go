package grpc

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const bootstrapServicePrefix = "/gereh.organization.v1.OrganizationBootstrapService/"

// InternalAuthConfig configures workload identity for the internal bootstrap
// service.
type InternalAuthConfig struct {
	Environment      string
	DevelopmentToken string
	AllowedSPIFFEIDs map[string]struct{}
}

// InternalWorkloadUnaryInterceptor authenticates trusted workload callers of
// OrganizationBootstrapService. All other methods pass through unchanged.
func InternalWorkloadUnaryInterceptor(
	config InternalAuthConfig,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !strings.HasPrefix(
			info.FullMethod,
			bootstrapServicePrefix,
		) {
			return handler(ctx, request)
		}

		if spiffeID, ok := peerSPIFFEID(ctx); ok {
			if _, allowed :=
				config.AllowedSPIFFEIDs[spiffeID]; allowed {
				return handler(ctx, request)
			}
		}

		if strings.EqualFold(
			config.Environment,
			"production",
		) {
			return nil, status.Error(
				codes.Unauthenticated,
				"trusted workload identity is required",
			)
		}

		if config.DevelopmentToken == "" {
			return nil, status.Error(
				codes.Unauthenticated,
				"internal development token is not configured",
			)
		}

		metadataValue, _ := metadata.FromIncomingContext(ctx)

		values := metadataValue.Get("authorization")
		if len(values) != 1 {
			return nil, status.Error(
				codes.Unauthenticated,
				"internal authorization is required",
			)
		}

		expected := "Bearer " + config.DevelopmentToken

		if subtle.ConstantTimeCompare(
			[]byte(values[0]),
			[]byte(expected),
		) != 1 {
			return nil, status.Error(
				codes.Unauthenticated,
				"invalid internal authorization",
			)
		}

		return handler(ctx, request)
	}
}

// peerSPIFFEID returns the mTLS peer SPIFFE identity when present.
func peerSPIFFEID(
	ctx context.Context,
) (string, bool) {
	peerValue, ok := peer.FromContext(ctx)
	if !ok {
		return "", false
	}

	tlsInfo, ok := peerValue.AuthInfo.(credentials.TLSInfo)
	if !ok ||
		len(tlsInfo.State.PeerCertificates) == 0 {
		return "", false
	}

	for _, uri := range tlsInfo.State.PeerCertificates[0].URIs {
		if uri.Scheme == "spiffe" {
			return uri.String(), true
		}
	}

	return "", false
}

// ParseAllowedSPIFFEIDs parses a comma-separated SPIFFE ID list.
func ParseAllowedSPIFFEIDs(
	value string,
) map[string]struct{} {
	result := make(map[string]struct{})

	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = struct{}{}
		}
	}

	return result
}
