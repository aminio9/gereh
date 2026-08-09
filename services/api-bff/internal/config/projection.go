package config

// ProjectionConfig defines the Projection Service gRPC client.
type ProjectionConfig struct {
	Target   string
	Insecure bool
}

// ProjectionConfigFromEnv loads Projection Service client configuration.
func ProjectionConfigFromEnv() (ProjectionConfig, error) {
	insecure, err := boolEnvironment(
		"PROJECTION_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return ProjectionConfig{}, err
	}

	return ProjectionConfig{
		Target: envOrDefault(
			"PROJECTION_GRPC_TARGET",
			"passthrough:///127.0.0.1:18086",
		),
		Insecure: insecure,
	}, nil
}
