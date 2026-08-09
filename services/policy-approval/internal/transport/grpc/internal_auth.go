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

const evaluationServicePrefix = "/gereh.policy.v1.PolicyEvaluationService/"
const bootstrapServicePrefix = "/gereh.policy.v1.PolicyBootstrapService/"

type callerServiceContextKey struct{}

// InternalAuthConfig configures workload identity for the internal policy
// services.
type InternalAuthConfig struct {
	Environment      string
	DevelopmentToken string
	AllowedSPIFFEIDs map[string]struct{}
}

// InternalWorkloadUnaryInterceptor authenticates trusted workload callers of
// PolicyEvaluationService and PolicyBootstrapService. All other methods pass
// through unchanged.
func InternalWorkloadUnaryInterceptor(
	config InternalAuthConfig,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !isInternalService(info.FullMethod) {
			return handler(ctx, request)
		}

		caller, ok := authorizedCaller(ctx, config)
		if !ok {
			return nil, status.Error(
				codes.Unauthenticated,
				"trusted workload identity is required",
			)
		}

		ctx = context.WithValue(
			ctx,
			callerServiceContextKey{},
			caller,
		)

		return handler(ctx, request)
	}
}

func isInternalService(
	fullMethod string,
) bool {
	return strings.HasPrefix(
		fullMethod,
		evaluationServicePrefix,
	) || strings.HasPrefix(
		fullMethod,
		bootstrapServicePrefix,
	)
}

func authorizedCaller(
	ctx context.Context,
	config InternalAuthConfig,
) (string, bool) {
	if spiffeID, ok := peerSPIFFEID(ctx); ok {
		if _, allowed :=
			config.AllowedSPIFFEIDs[spiffeID]; allowed {
			return spiffeID, true
		}

		return "", false
	}

	if strings.EqualFold(
		config.Environment,
		"production",
	) {
		return "", false
	}

	if config.DevelopmentToken == "" {
		return "", false
	}

	metadataValue, _ := metadata.FromIncomingContext(ctx)

	values := metadataValue.Get("authorization")
	if len(values) != 1 {
		return "", false
	}

	expected := "Bearer " + config.DevelopmentToken

	if subtle.ConstantTimeCompare(
		[]byte(values[0]),
		[]byte(expected),
	) != 1 {
		return "", false
	}

	return "policy-evaluation-client", true
}

// callerService returns the authenticated workload identity for the current
// request.
func callerService(
	ctx context.Context,
) string {
	caller, ok := ctx.Value(
		callerServiceContextKey{},
	).(string)
	if !ok || caller == "" {
		return "internal"
	}

	return caller
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
