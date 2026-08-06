package config

// OrganizationConfig defines the Organization Service gRPC client.
type OrganizationConfig struct {
	Target   string
	Insecure bool
}

// OrganizationConfigFromEnv loads Organization Service client configuration.
func OrganizationConfigFromEnv() (OrganizationConfig, error) {
	insecure, err := boolEnvironment(
		"ORGANIZATION_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return OrganizationConfig{}, err
	}

	return OrganizationConfig{
		Target: envOrDefault(
			"ORGANIZATION_GRPC_TARGET",
			"passthrough:///127.0.0.1:18083",
		),
		Insecure: insecure,
	}, nil
}
