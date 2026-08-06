package config

// WorkConfig defines the Work Management Service gRPC client.
type WorkConfig struct {
	Target   string
	Insecure bool
}

// WorkConfigFromEnv loads Work Management Service client configuration.
func WorkConfigFromEnv() (WorkConfig, error) {
	insecure, err := boolEnvironment(
		"WORK_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return WorkConfig{}, err
	}

	return WorkConfig{
		Target: envOrDefault(
			"WORK_GRPC_TARGET",
			"passthrough:///127.0.0.1:18084",
		),
		Insecure: insecure,
	}, nil
}
