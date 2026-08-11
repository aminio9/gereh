package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ModelAccessConfig configures the BFF Model Access gRPC client.
type ModelAccessConfig struct {
	Target string

	Insecure bool

	ServerName string

	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string
}

// ModelAccessConfigFromEnv loads Model Access client configuration.
func ModelAccessConfigFromEnv() (ModelAccessConfig, error) {
	insecure := true

	rawInsecure := strings.TrimSpace(
		os.Getenv("MODEL_ACCESS_GRPC_INSECURE"),
	)

	if rawInsecure != "" {
		value, err := strconv.ParseBool(rawInsecure)
		if err != nil {
			return ModelAccessConfig{}, fmt.Errorf(
				"parse MODEL_ACCESS_GRPC_INSECURE: %w",
				err,
			)
		}

		insecure = value
	}

	return ModelAccessConfig{
		Target: envOrDefault(
			"MODEL_ACCESS_GRPC_TARGET",
			"passthrough:///127.0.0.1:18087",
		),

		Insecure: insecure,

		ServerName: strings.TrimSpace(
			os.Getenv("MODEL_ACCESS_GRPC_SERVER_NAME"),
		),

		TLSCertFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_CERT_FILE"),
		),

		TLSKeyFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_KEY_FILE"),
		),

		TLSCAFile: strings.TrimSpace(
			os.Getenv("GRPC_TLS_CA_FILE"),
		),
	}, nil
}
