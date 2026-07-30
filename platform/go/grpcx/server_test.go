package grpcx

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/aminio9/gereh/platform/go/observability"
	"google.golang.org/grpc/test/bufconn"
)

func TestServerHealth(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)

	telemetryConfig := observability.DefaultConfig(
		"grpc-test",
		"dev",
	)

	telemetry, err := observability.Setup(
		context.Background(),
		telemetryConfig,
	)
	if err != nil {
		t.Fatalf("observability.Setup() error = %v", err)
	}

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
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	serveErrors := make(chan error, 1)

	go func() {
		serveErrors <- server.Serve(listener)
	}()

	clientConfig := DefaultClientConfig(
		"passthrough:///bufnet",
	)
	clientConfig.Insecure = true
	clientConfig.Dialer = func(
		ctx context.Context,
		_ string,
	) (net.Conn, error) {
		return listener.DialContext(ctx)
	}

	connection, err := NewClient(
		clientConfig,
		telemetry,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf(
				"connection.Close() error = %v",
				err,
			)
		}
	})

	healthContext, healthCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer healthCancel()

	if err := CheckHealth(
		healthContext,
		connection,
		"",
	); err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}

	stopContext, stopCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer stopCancel()

	if err := server.GracefulStop(stopContext); err != nil {
		t.Fatalf("GracefulStop() error = %v", err)
	}

	select {
	case err := <-serveErrors:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return")
	}
}
