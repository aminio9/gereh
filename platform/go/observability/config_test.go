package observability

import (
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	clearTelemetryEnvironment(t)

	t.Setenv("OTEL_SERVICE_NAME", "tenant")
	t.Setenv("SERVICE_VERSION", "1.2.3")
	t.Setenv("GEREH_ENVIRONMENT", "test")
	t.Setenv(
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"http://collector:4317",
	)
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "15000")
	t.Setenv(
		"OTEL_RESOURCE_ATTRIBUTES",
		"region=eu-central-1,tenant.mode=shared",
	)

	config, err := ConfigFromEnv("fallback", "dev")
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if !config.Enabled {
		t.Fatal("ConfigFromEnv() telemetry is disabled")
	}

	if !config.TracesEnabled {
		t.Fatal("ConfigFromEnv() traces are disabled")
	}

	if !config.MetricsEnabled {
		t.Fatal("ConfigFromEnv() metrics are disabled")
	}

	if config.ServiceName != "tenant" {
		t.Fatalf(
			"ServiceName = %q, want tenant",
			config.ServiceName,
		)
	}

	if config.ServiceVersion != "1.2.3" {
		t.Fatalf(
			"ServiceVersion = %q, want 1.2.3",
			config.ServiceVersion,
		)
	}

	if config.Environment != "test" {
		t.Fatalf(
			"Environment = %q, want test",
			config.Environment,
		)
	}

	if config.TraceSampleRatio != 0.25 {
		t.Fatalf(
			"TraceSampleRatio = %f, want 0.25",
			config.TraceSampleRatio,
		)
	}

	if config.MetricInterval != 15*time.Second {
		t.Fatalf(
			"MetricInterval = %s, want 15s",
			config.MetricInterval,
		)
	}

	if config.ResourceAttributes["region"] != "eu-central-1" {
		t.Fatalf(
			"region = %q, want eu-central-1",
			config.ResourceAttributes["region"],
		)
	}

	if config.ResourceAttributes["tenant.mode"] != "shared" {
		t.Fatalf(
			"tenant.mode = %q, want shared",
			config.ResourceAttributes["tenant.mode"],
		)
	}
}

func TestConfigFromEnvDisabledWithoutEndpoint(t *testing.T) {
	clearTelemetryEnvironment(t)

	config, err := ConfigFromEnv("tenant", "dev")
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if config.Enabled {
		t.Fatal(
			"ConfigFromEnv() telemetry is enabled without an endpoint",
		)
	}

	if config.TracesEnabled {
		t.Fatal(
			"ConfigFromEnv() traces are enabled without an endpoint",
		)
	}

	if config.MetricsEnabled {
		t.Fatal(
			"ConfigFromEnv() metrics are enabled without an endpoint",
		)
	}
}

func TestConfigFromEnvDisabledBySDKFlag(t *testing.T) {
	clearTelemetryEnvironment(t)

	t.Setenv(
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"http://collector:4317",
	)
	t.Setenv("OTEL_SDK_DISABLED", "true")

	config, err := ConfigFromEnv("tenant", "dev")
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if config.Enabled {
		t.Fatal(
			"ConfigFromEnv() telemetry is enabled when SDK is disabled",
		)
	}
}

func TestConfigFromEnvEnablesOnlyTraces(t *testing.T) {
	clearTelemetryEnvironment(t)

	t.Setenv(
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"http://collector:4317",
	)

	config, err := ConfigFromEnv("tenant", "dev")
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if !config.Enabled {
		t.Fatal("ConfigFromEnv() telemetry is disabled")
	}

	if !config.TracesEnabled {
		t.Fatal("ConfigFromEnv() traces are disabled")
	}

	if config.MetricsEnabled {
		t.Fatal(
			"ConfigFromEnv() metrics are unexpectedly enabled",
		)
	}
}

func TestConfigFromEnvEnablesOnlyMetrics(t *testing.T) {
	clearTelemetryEnvironment(t)

	t.Setenv(
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"http://collector:4317",
	)

	config, err := ConfigFromEnv("tenant", "dev")
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if !config.Enabled {
		t.Fatal("ConfigFromEnv() telemetry is disabled")
	}

	if config.TracesEnabled {
		t.Fatal(
			"ConfigFromEnv() traces are unexpectedly enabled",
		)
	}

	if !config.MetricsEnabled {
		t.Fatal("ConfigFromEnv() metrics are disabled")
	}
}

func TestConfigValidateRejectsInvalidSampling(t *testing.T) {
	config := DefaultConfig("tenant", "dev")
	config.TraceSampleRatio = 1.1

	if err := config.Validate(); err == nil {
		t.Fatal(
			"Validate() error = nil, want sampling error",
		)
	}
}

func clearTelemetryEnvironment(t *testing.T) {
	t.Helper()

	variables := []string{
		"OTEL_SDK_DISABLED",
		"OTEL_SERVICE_NAME",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_INSECURE",
		"OTEL_EXPORTER_OTLP_TRACES_INSECURE",
		"OTEL_EXPORTER_OTLP_METRICS_INSECURE",
		"OTEL_TRACES_SAMPLER",
		"OTEL_TRACES_SAMPLER_ARG",
		"OTEL_METRIC_EXPORT_INTERVAL",
		"OTEL_RESOURCE_ATTRIBUTES",
		"SERVICE_VERSION",
		"GEREH_ENVIRONMENT",
		"DEPLOYMENT_ENVIRONMENT",
		"GEREH_OTEL_SHUTDOWN_TIMEOUT",
	}

	for _, variable := range variables {
		t.Setenv(variable, "")
	}
}
