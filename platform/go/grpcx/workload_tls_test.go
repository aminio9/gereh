package grpcx

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func newTLSHealthServer(
	t *testing.T,
	tlsConfig *tls.Config,
) (*bufconn.Listener, *grpc.Server) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer(
		grpc.Creds(
			credentials.NewTLS(tlsConfig),
		),
	)

	healthServer := health.NewServer()
	healthServer.SetServingStatus(
		"",
		healthpb.HealthCheckResponse_SERVING,
	)
	healthpb.RegisterHealthServer(server, healthServer)

	go func() {
		_ = server.Serve(listener)
	}()

	return listener, server
}

func invokeHealthCheck(
	t *testing.T,
	listener *bufconn.Listener,
	tlsConfig *tls.Config,
) error {
	t.Helper()

	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(
			credentials.NewTLS(tlsConfig),
		),
		grpc.WithContextDialer(func(
			ctx context.Context,
			_ string,
		) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("create gRPC client: %v", err)
	}
	defer func() {
		_ = connection.Close()
	}()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)
	defer cancel()

	return connection.Invoke(
		ctx,
		"/grpc.health.v1.Health/Check",
		&healthpb.HealthCheckRequest{},
		&healthpb.HealthCheckResponse{},
	)
}

type testWorkloadPKI struct {
	caPool *x509.CertPool

	serverCertFile string
	serverKeyFile  string

	clientCertFile string
	clientKeyFile  string

	caFile string
}

func (pki *testWorkloadPKI) serverFiles() WorkloadTLSFiles {
	return WorkloadTLSFiles{
		CertificateFile: pki.serverCertFile,
		PrivateKeyFile:  pki.serverKeyFile,
		CAFile:          pki.caFile,
	}
}

func (pki *testWorkloadPKI) clientFiles() WorkloadTLSFiles {
	return WorkloadTLSFiles{
		CertificateFile: pki.clientCertFile,
		PrivateKeyFile:  pki.clientKeyFile,
		CAFile:          pki.caFile,
	}
}

func generateKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(
		elliptic.P256(),
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}

	return key
}

func writePEMFile(
	t *testing.T,
	directory string,
	name string,
	blockType string,
	der []byte,
) string {
	t.Helper()

	path := filepath.Join(directory, name)

	encoded := pem.EncodeToMemory(
		&pem.Block{
			Type:  blockType,
			Bytes: der,
		},
	)

	if err := os.WriteFile(
		path,
		encoded,
		0o600,
	); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	return path
}

func newTestWorkloadPKI(t *testing.T) *testWorkloadPKI {
	t.Helper()

	directory := t.TempDir()

	caKey := generateKey(t)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Gereh Workload Test CA",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		IsCA:      true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	caDER, err := x509.CreateCertificate(
		rand.Reader,
		caTemplate,
		caTemplate,
		&caKey.PublicKey,
		caKey,
	)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}

	caFile := writePEMFile(
		t,
		directory,
		"ca.crt",
		"CERTIFICATE",
		caDER,
	)

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	issue := func(
		commonName string,
		spiffeID string,
		dnsNames []string,
	) (string, string) {
		key := generateKey(t)

		template := &x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			Subject: pkix.Name{
				CommonName: commonName,
			},
			NotBefore: time.Now().Add(-time.Hour),
			NotAfter:  time.Now().Add(24 * time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature |
				x509.KeyUsageKeyEncipherment,
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
				x509.ExtKeyUsageClientAuth,
			},
			DNSNames: dnsNames,
		}

		if spiffeID != "" {
			parsedID, err := url.Parse(spiffeID)
			if err != nil {
				t.Fatalf("parse SPIFFE ID %q: %v", spiffeID, err)
			}

			template.URIs = []*url.URL{parsedID}
		}

		certDER, err := x509.CreateCertificate(
			rand.Reader,
			template,
			caCert,
			&key.PublicKey,
			caKey,
		)
		if err != nil {
			t.Fatalf("create %s certificate: %v", commonName, err)
		}

		certFile := writePEMFile(
			t,
			directory,
			commonName+".crt",
			"CERTIFICATE",
			certDER,
		)

		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatalf("marshal %s key: %v", commonName, err)
		}

		keyFile := writePEMFile(
			t,
			directory,
			commonName+".key",
			"EC PRIVATE KEY",
			keyDER,
		)

		return certFile, keyFile
	}

	serverCertFile, serverKeyFile := issue(
		"tenant.control-plane.svc",
		"",
		[]string{"tenant.control-plane.svc"},
	)

	clientCertFile, clientKeyFile := issue(
		"execution-orchestrator",
		"gereh.internal/ns/control-plane/sa/execution-orchestrator",
		nil,
	)

	return &testWorkloadPKI{
		caPool: caPool,

		serverCertFile: serverCertFile,
		serverKeyFile:  serverKeyFile,

		clientCertFile: clientCertFile,
		clientKeyFile:  clientKeyFile,

		caFile: caFile,
	}
}

func TestWorkloadTLSFilesValidation(t *testing.T) {
	t.Parallel()

	err := (WorkloadTLSFiles{}).Validate()
	require.Error(t, err)

	err = (WorkloadTLSFiles{
		CertificateFile: "a",
		PrivateKeyFile:  "b",
		CAFile:          "c",
	}).Validate()
	require.NoError(t, err)
}

func TestLoadWorkloadServerTLSRequiresClientCertificate(
	t *testing.T,
) {
	pki := newTestWorkloadPKI(t)

	serverTLS, err := LoadWorkloadServerTLS(
		pki.serverFiles(),
	)
	require.NoError(t, err)
	require.Equal(
		t,
		tls.RequireAndVerifyClientCert,
		serverTLS.ClientAuth,
	)

	listener, server := newTLSHealthServer(t, serverTLS)
	defer server.Stop()

	// A client without a certificate must be rejected at the TLS layer.
	err = invokeHealthCheck(
		t,
		listener,
		&tls.Config{
			MinVersion: tls.VersionTLS13,
			RootCAs:    pki.caPool,
			ServerName: "tenant.control-plane.svc",
		},
	)
	require.Error(t, err)
}

func TestWorkloadTLSUnknownCARejected(t *testing.T) {
	pki := newTestWorkloadPKI(t)

	serverTLS, err := LoadWorkloadServerTLS(
		pki.serverFiles(),
	)
	require.NoError(t, err)

	// A client chained to an unrelated CA must be rejected.
	unrelatedCAKey := generateKey(t)

	unrelatedCATemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "Unrelated CA",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	unrelatedCADER, err := x509.CreateCertificate(
		rand.Reader,
		unrelatedCATemplate,
		unrelatedCATemplate,
		&unrelatedCAKey.PublicKey,
		unrelatedCAKey,
	)
	require.NoError(t, err)

	unrelatedCACert, err := x509.ParseCertificate(
		unrelatedCADER,
	)
	require.NoError(t, err)

	directory := t.TempDir()

	clientKey := generateKey(t)

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName: "rogue",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	clientDER, err := x509.CreateCertificate(
		rand.Reader,
		clientTemplate,
		unrelatedCACert,
		&clientKey.PublicKey,
		unrelatedCAKey,
	)
	require.NoError(t, err)

	clientCertFile := writePEMFile(
		t,
		directory,
		"rogue.crt",
		"CERTIFICATE",
		clientDER,
	)

	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	require.NoError(t, err)

	clientKeyFile := writePEMFile(
		t,
		directory,
		"rogue.key",
		"EC PRIVATE KEY",
		clientKeyDER,
	)

	clientCert, err := tls.LoadX509KeyPair(
		clientCertFile,
		clientKeyFile,
	)
	require.NoError(t, err)

	listener, server := newTLSHealthServer(t, serverTLS)
	defer server.Stop()

	err = invokeHealthCheck(
		t,
		listener,
		&tls.Config{
			MinVersion:   tls.VersionTLS13,
			RootCAs:      pki.caPool,
			ServerName:   "tenant.control-plane.svc",
			Certificates: []tls.Certificate{clientCert},
		},
	)
	require.Error(t, err)
}

func TestWorkloadTLSValidPlatformCAAccepted(t *testing.T) {
	pki := newTestWorkloadPKI(t)

	serverTLS, err := LoadWorkloadServerTLS(
		pki.serverFiles(),
	)
	require.NoError(t, err)

	clientTLS, err := LoadWorkloadClientTLS(
		pki.clientFiles(),
		"tenant.control-plane.svc",
	)
	require.NoError(t, err)

	listener, server := newTLSHealthServer(t, serverTLS)
	defer server.Stop()

	// Health RPC succeeds: the client certificate chains to the
	// platform CA and the server verifies it.
	err = invokeHealthCheck(
		t,
		listener,
		clientTLS,
	)
	require.NoError(t, err)
}

func TestWorkloadTLSMissingServerNameRejected(t *testing.T) {
	t.Parallel()

	pki := newTestWorkloadPKI(t)

	_, err := LoadWorkloadClientTLS(
		pki.clientFiles(),
		"",
	)
	require.Error(t, err)
}

func TestWorkloadTLSMissingFilesRejected(t *testing.T) {
	t.Parallel()

	_, err := LoadWorkloadServerTLS(
		WorkloadTLSFiles{},
	)
	require.Error(t, err)

	_, err = LoadWorkloadClientTLS(
		WorkloadTLSFiles{},
		"tenant.control-plane.svc",
	)
	require.Error(t, err)
}

func TestNewServerWithTLSConfig(t *testing.T) {
	pki := newTestWorkloadPKI(t)

	serverTLS, err := LoadWorkloadServerTLS(
		pki.serverFiles(),
	)
	require.NoError(t, err)

	telemetryConfig := observability.DefaultConfig(
		"grpc-tls-test",
		"dev",
	)

	telemetry, err := observability.Setup(
		context.Background(),
		telemetryConfig,
	)
	require.NoError(t, err)

	logger := slog.New(
		slog.NewTextHandler(
			io.Discard,
			nil,
		),
	)

	serverConfig := DefaultServerConfig()
	serverConfig.TLSConfig = serverTLS

	server, err := NewServer(
		serverConfig,
		telemetry,
		logger,
	)
	require.NoError(t, err)
	require.NotNil(t, server)
}

func TestNewServerWithoutTLS(t *testing.T) {
	t.Parallel()

	telemetryConfig := observability.DefaultConfig(
		"grpc-plain-test",
		"dev",
	)

	telemetry, err := observability.Setup(
		context.Background(),
		telemetryConfig,
	)
	require.NoError(t, err)

	logger := slog.New(
		slog.NewTextHandler(
			io.Discard,
			nil,
		),
	)

	server, err := NewServer(
		DefaultServerConfig(),
		telemetry,
		logger,
	)
	require.NoError(t, err)
	require.NotNil(t, server)
}

// TestClientInsecurePreservesDevelopmentBehavior ensures the shared
// client still supports the insecure development path.
func TestClientInsecurePreservesDevelopmentBehavior(t *testing.T) {
	t.Parallel()

	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer(
		grpc.Creds(
			insecure.NewCredentials(),
		),
	)

	healthServer := health.NewServer()
	healthServer.SetServingStatus(
		"",
		healthpb.HealthCheckResponse_SERVING,
	)
	healthpb.RegisterHealthServer(server, healthServer)

	defer server.Stop()

	go func() {
		_ = server.Serve(listener)
	}()

	config := DefaultClientConfig(
		"passthrough:///bufnet",
	)
	config.Insecure = true
	config.Dialer = func(
		ctx context.Context,
		_ string,
	) (net.Conn, error) {
		return listener.DialContext(ctx)
	}

	telemetryConfig := observability.DefaultConfig(
		"grpc-client-test",
		"dev",
	)

	telemetry, err := observability.Setup(
		context.Background(),
		telemetryConfig,
	)
	require.NoError(t, err)

	connection, err := NewClient(
		config,
		telemetry,
	)
	require.NoError(t, err)
	defer func() {
		_ = connection.Close()
	}()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)
	defer cancel()

	err = connection.Invoke(
		ctx,
		"/grpc.health.v1.Health/Check",
		&healthpb.HealthCheckRequest{},
		&healthpb.HealthCheckResponse{},
	)
	require.NoError(t, err)
}
