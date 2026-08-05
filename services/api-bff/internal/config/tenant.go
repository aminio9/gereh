package config

// TenantConfig defines the Tenant Service gRPC client.
type TenantConfig struct {
	Target   string
	Insecure bool
}

// TenantConfigFromEnv loads Tenant Service client configuration.
func TenantConfigFromEnv() (TenantConfig, error) {
	insecure, err := boolEnvironment(
		"TENANT_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return TenantConfig{}, err
	}

	return TenantConfig{
		Target: envOrDefault(
			"TENANT_GRPC_TARGET",
			"passthrough:///127.0.0.1:18082",
		),
		Insecure: insecure,
	}, nil
}
