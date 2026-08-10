package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"testing"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func peerContextWithSPIFFE(t *testing.T, spiffeID string) context.Context {
	t.Helper()

	parsedID, err := url.Parse(spiffeID)
	require.NoError(t, err)

	return peer.NewContext(
		context.Background(),
		&peer.Peer{
			AuthInfo: credentials.TLSInfo{
				State: tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							URIs: []*url.URL{
								parsedID,
							},
						},
					},
				},
			},
		},
	)
}

func contextWithBearerToken(token string) context.Context {
	return metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(
			"authorization",
			"Bearer "+token,
		),
	)
}

func onboardingInterceptorConfig(
	environment string,
) InternalAuthConfig {
	return InternalAuthConfig{
		Environment:      environment,
		DevelopmentToken: "dev-token-change-me",
		AllowedSPIFFEIDs: ParseAllowedSPIFFEIDs(
			"spiffe://gereh.internal/ns/control-plane/sa/execution-orchestrator",
		),
	}
}

func requireUnauthenticated(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)
	require.Equal(
		t,
		codes.Unauthenticated,
		status.Code(err),
	)
}

func TestInternalAuthAllowsAllowedSPIFFE(t *testing.T) {
	t.Parallel()

	interceptor := InternalWorkloadUnaryInterceptor(
		onboardingInterceptorConfig("production"),
	)

	ctx := peerContextWithSPIFFE(
		t,
		"spiffe://gereh.internal/ns/control-plane/sa/execution-orchestrator",
	)

	called := false

	_, err := interceptor(
		ctx,
		&tenantv1.MarkOnboardingRunningRequest{},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantOnboardingService/MarkOnboardingRunning",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			called = true

			return &tenantv1.MarkOnboardingRunningResponse{}, nil
		},
	)

	require.NoError(t, err)
	require.True(t, called)
}

func TestInternalAuthRejectsUnauthorizedSPIFFE(t *testing.T) {
	t.Parallel()

	interceptor := InternalWorkloadUnaryInterceptor(
		onboardingInterceptorConfig("production"),
	)

	// A trusted platform CA certificate whose SPIFFE ID is not on the
	// allowlist must be rejected at the interceptor layer.
	ctx := peerContextWithSPIFFE(
		t,
		"spiffe://gereh.internal/ns/control-plane/sa/rogue-service",
	)

	_, err := interceptor(
		ctx,
		&tenantv1.MarkOnboardingRunningRequest{},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantOnboardingService/MarkOnboardingRunning",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal("handler was called for unauthorized SPIFFE identity")

			return nil, nil
		},
	)

	requireUnauthenticated(t, err)
}

func TestInternalAuthRejectsProductionBearerFallback(t *testing.T) {
	t.Parallel()

	interceptor := InternalWorkloadUnaryInterceptor(
		onboardingInterceptorConfig("production"),
	)

	// The development bearer token must never be accepted in production,
	// even when it matches the configured development token.
	ctx := contextWithBearerToken("dev-token-change-me")

	_, err := interceptor(
		ctx,
		&tenantv1.MarkOnboardingRunningRequest{},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantOnboardingService/MarkOnboardingRunning",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal("handler was called without workload identity in production")

			return nil, nil
		},
	)

	requireUnauthenticated(t, err)
}

func TestInternalAuthAllowsDevelopmentBearerToken(t *testing.T) {
	t.Parallel()

	interceptor := InternalWorkloadUnaryInterceptor(
		onboardingInterceptorConfig("development"),
	)

	ctx := contextWithBearerToken("dev-token-change-me")

	called := false

	_, err := interceptor(
		ctx,
		&tenantv1.MarkOnboardingRunningRequest{},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantOnboardingService/MarkOnboardingRunning",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			called = true

			return &tenantv1.MarkOnboardingRunningResponse{}, nil
		},
	)

	require.NoError(t, err)
	require.True(t, called)
}

func TestInternalAuthRejectsInvalidDevelopmentToken(t *testing.T) {
	t.Parallel()

	interceptor := InternalWorkloadUnaryInterceptor(
		onboardingInterceptorConfig("development"),
	)

	ctx := contextWithBearerToken("wrong-token")

	_, err := interceptor(
		ctx,
		&tenantv1.MarkOnboardingRunningRequest{},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantOnboardingService/MarkOnboardingRunning",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal("handler was called with invalid development token")

			return nil, nil
		},
	)

	requireUnauthenticated(t, err)
}

func TestInternalAuthRejectsMissingCredentials(t *testing.T) {
	t.Parallel()

	interceptor := InternalWorkloadUnaryInterceptor(
		onboardingInterceptorConfig("production"),
	)

	_, err := interceptor(
		context.Background(),
		&tenantv1.MarkOnboardingRunningRequest{},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantOnboardingService/MarkOnboardingRunning",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal("handler was called without credentials")

			return nil, nil
		},
	)

	requireUnauthenticated(t, err)
}

func TestInternalAuthPassesThroughOtherMethods(t *testing.T) {
	t.Parallel()

	interceptor := InternalWorkloadUnaryInterceptor(
		onboardingInterceptorConfig("production"),
	)

	// Methods outside the onboarding service are not protected by the
	// workload interceptor; the actor-binding layer handles them.
	called := false

	_, err := interceptor(
		context.Background(),
		&tenantv1.GetTenantRequest{},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantService/GetTenant",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			called = true

			return &tenantv1.GetTenantResponse{}, nil
		},
	)

	require.NoError(t, err)
	require.True(t, called)
}
