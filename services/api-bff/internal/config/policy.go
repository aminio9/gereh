package config

// PolicyConfig defines the Policy Service gRPC client.
type PolicyConfig struct {
	Target   string
	Insecure bool
}

// PolicyConfigFromEnv loads Policy Service client configuration.
func PolicyConfigFromEnv() (PolicyConfig, error) {
	insecure, err := boolEnvironment(
		"POLICY_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return PolicyConfig{}, err
	}

	return PolicyConfig{
		Target: envOrDefault(
			"POLICY_GRPC_TARGET",
			"passthrough:///127.0.0.1:18085",
		),
		Insecure: insecure,
	}, nil
}
