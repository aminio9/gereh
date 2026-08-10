package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func TestPeerSPIFFEExtractsSPIFFEURI(t *testing.T) {
	t.Parallel()

	expectedID :=
		"spiffe://gereh.internal/ns/control-plane/sa/tenant"

	parsedID, err := url.Parse(expectedID)
	require.NoError(t, err)

	ctx := peer.NewContext(
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

	extracted, ok := peerSPIFFEID(ctx)

	require.True(t, ok)
	require.Equal(t, expectedID, extracted)
}

func TestPeerSPIFFEMissing(t *testing.T) {
	t.Parallel()

	_, ok := peerSPIFFEID(context.Background())
	require.False(t, ok)

	ctx := peer.NewContext(
		context.Background(),
		&peer.Peer{},
	)

	_, ok = peerSPIFFEID(ctx)
	require.False(t, ok)
}

func TestPeerSPIFFEIgnoresNonSPIFFEURIs(t *testing.T) {
	t.Parallel()

	otherID, err := url.Parse("https://example.com/identity")
	require.NoError(t, err)

	ctx := peer.NewContext(
		context.Background(),
		&peer.Peer{
			AuthInfo: credentials.TLSInfo{
				State: tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							URIs: []*url.URL{
								otherID,
							},
						},
					},
				},
			},
		},
	)

	_, ok := peerSPIFFEID(ctx)
	require.False(t, ok)
}

func TestParseAllowedSPIFFEIDs(t *testing.T) {
	t.Parallel()

	allowed := ParseAllowedSPIFFEIDs(
		"spiffe://gereh.internal/ns/control-plane/sa/tenant, spiffe://gereh.internal/ns/control-plane/sa/execution-orchestrator",
	)

	require.Len(t, allowed, 2)

	_, tenantOK := allowed["spiffe://gereh.internal/ns/control-plane/sa/tenant"]
	require.True(t, tenantOK)

	_, orchestratorOK := allowed["spiffe://gereh.internal/ns/control-plane/sa/execution-orchestrator"]
	require.True(t, orchestratorOK)
}

func TestParseAllowedSPIFFEIDsEmpty(t *testing.T) {
	t.Parallel()

	allowed := ParseAllowedSPIFFEIDs("")

	require.Empty(t, allowed)
}
