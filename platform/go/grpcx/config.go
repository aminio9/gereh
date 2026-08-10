// Package grpcx provides shared, secure gRPC server and client construction.
package grpcx

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
)

const (
	defaultMessageSize = 4 * 1024 * 1024

	defaultUnaryTimeout = 5 * time.Second
)

// ServerConfig defines shared gRPC server behavior.
type ServerConfig struct {
	MaxReceiveMessageBytes int
	MaxSendMessageBytes    int
	EnableReflection       bool
	GracefulStopTimeout    time.Duration

	// TLSConfig, when set, enables mTLS on the server. Production
	// service configuration must provide workload TLS credentials;
	// development may leave this nil.
	TLSConfig *tls.Config

	UnaryInterceptors  []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor
}

// DefaultServerConfig returns conservative gRPC server defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		MaxReceiveMessageBytes: defaultMessageSize,
		MaxSendMessageBytes:    defaultMessageSize,
		EnableReflection:       false,
		GracefulStopTimeout:    15 * time.Second,
	}
}

// Validate validates server configuration.
func (config ServerConfig) Validate() error {
	if config.MaxReceiveMessageBytes <= 0 {
		return fmt.Errorf(
			"maximum receive message size must be greater than zero",
		)
	}

	if config.MaxSendMessageBytes <= 0 {
		return fmt.Errorf(
			"maximum send message size must be greater than zero",
		)
	}

	if config.GracefulStopTimeout <= 0 {
		return fmt.Errorf(
			"graceful stop timeout must be greater than zero",
		)
	}

	for index, interceptor := range config.UnaryInterceptors {
		if interceptor == nil {
			return fmt.Errorf(
				"unary interceptor %d is nil",
				index,
			)
		}
	}

	for index, interceptor := range config.StreamInterceptors {
		if interceptor == nil {
			return fmt.Errorf(
				"stream interceptor %d is nil",
				index,
			)
		}
	}

	return nil
}

// ContextDialer provides custom transport dialing, primarily for tests.
type ContextDialer func(
	ctx context.Context,
	address string,
) (net.Conn, error)

// ClientConfig defines shared gRPC client behavior.
type ClientConfig struct {
	Target                 string
	Insecure               bool
	ServerName             string
	TLSConfig              *tls.Config
	MaxReceiveMessageBytes int
	MaxSendMessageBytes    int

	// DefaultUnaryTimeout is applied to unary calls that do not
	// already carry a deadline. Zero resolves to the conservative
	// five-second fallback for backward compatibility.
	DefaultUnaryTimeout time.Duration

	DefaultServiceConfig string
	Dialer               ContextDialer
}

// DefaultClientConfig returns conservative gRPC client defaults.
func DefaultClientConfig(target string) ClientConfig {
	return ClientConfig{
		Target:                 target,
		Insecure:               false,
		MaxReceiveMessageBytes: defaultMessageSize,
		MaxSendMessageBytes:    defaultMessageSize,
		DefaultUnaryTimeout:    defaultUnaryTimeout,
	}
}

// Validate validates client configuration.
func (config ClientConfig) Validate() error {
	if strings.TrimSpace(config.Target) == "" {
		return fmt.Errorf("gRPC target is required")
	}

	if config.MaxReceiveMessageBytes <= 0 {
		return fmt.Errorf(
			"maximum receive message size must be greater than zero",
		)
	}

	if config.MaxSendMessageBytes <= 0 {
		return fmt.Errorf(
			"maximum send message size must be greater than zero",
		)
	}

	return nil
}
