// Package observability provides shared OpenTelemetry configuration and
// instrumentation for Gereh services.
package observability

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMetricInterval  = 60 * time.Second
	defaultShutdownTimeout = 10 * time.Second
	defaultTraceSampleRate = 1.0
)

// Config defines the OpenTelemetry configuration for a Gereh service.
type Config struct {
	ServiceName        string
	ServiceVersion     string
	Environment        string
	Enabled            bool
	TracesEnabled      bool
	MetricsEnabled     bool
	TraceSampleRatio   float64
	MetricInterval     time.Duration
	ShutdownTimeout    time.Duration
	ResourceAttributes map[string]string
}

// DefaultConfig returns safe development defaults for a service.
//
// Telemetry remains disabled until an OTLP endpoint is configured.
func DefaultConfig(serviceName string, serviceVersion string) Config {
	return Config{
		ServiceName:        serviceName,
		ServiceVersion:     serviceVersion,
		Environment:        "development",
		Enabled:            false,
		TracesEnabled:      false,
		MetricsEnabled:     false,
		TraceSampleRatio:   defaultTraceSampleRate,
		MetricInterval:     defaultMetricInterval,
		ShutdownTimeout:    defaultShutdownTimeout,
		ResourceAttributes: make(map[string]string),
	}
}

// ConfigFromEnv creates telemetry configuration from OpenTelemetry environment
// variables and Gereh deployment metadata.
func ConfigFromEnv(serviceName string, serviceVersion string) (Config, error) {
	config := DefaultConfig(serviceName, serviceVersion)

	if value := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); value != "" {
		config.ServiceName = value
	}

	if value := strings.TrimSpace(os.Getenv("SERVICE_VERSION")); value != "" {
		config.ServiceVersion = value
	}

	config.Environment = firstNonEmpty(
		os.Getenv("GEREH_ENVIRONMENT"),
		os.Getenv("DEPLOYMENT_ENVIRONMENT"),
		config.Environment,
	)

	disabled, err := parseBoolEnvironment("OTEL_SDK_DISABLED", false)
	if err != nil {
		return Config{}, err
	}

	commonEndpointConfigured := strings.TrimSpace(
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	) != ""

	traceEndpointConfigured := strings.TrimSpace(
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
	) != ""

	metricEndpointConfigured := strings.TrimSpace(
		os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"),
	) != ""

	config.TracesEnabled = commonEndpointConfigured || traceEndpointConfigured
	config.MetricsEnabled = commonEndpointConfigured || metricEndpointConfigured
	config.Enabled = !disabled && (config.TracesEnabled || config.MetricsEnabled)

	if value := strings.TrimSpace(
		os.Getenv("OTEL_TRACES_SAMPLER_ARG"),
	); value != "" {
		ratio, parseErr := strconv.ParseFloat(value, 64)
		if parseErr != nil {
			return Config{}, fmt.Errorf(
				"parse OTEL_TRACES_SAMPLER_ARG: %w",
				parseErr,
			)
		}

		config.TraceSampleRatio = ratio
	}

	if value := strings.TrimSpace(
		os.Getenv("OTEL_METRIC_EXPORT_INTERVAL"),
	); value != "" {
		milliseconds, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			return Config{}, fmt.Errorf(
				"parse OTEL_METRIC_EXPORT_INTERVAL: %w",
				parseErr,
			)
		}

		config.MetricInterval = time.Duration(milliseconds) * time.Millisecond
	}

	if value := strings.TrimSpace(
		os.Getenv("GEREH_OTEL_SHUTDOWN_TIMEOUT"),
	); value != "" {
		timeout, parseErr := time.ParseDuration(value)
		if parseErr != nil {
			return Config{}, fmt.Errorf(
				"parse GEREH_OTEL_SHUTDOWN_TIMEOUT: %w",
				parseErr,
			)
		}

		config.ShutdownTimeout = timeout
	}

	config.ResourceAttributes = parseResourceAttributes(
		os.Getenv("OTEL_RESOURCE_ATTRIBUTES"),
	)

	if err := config.Validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

// Validate validates OpenTelemetry configuration.
func (config Config) Validate() error {
	if strings.TrimSpace(config.ServiceName) == "" {
		return fmt.Errorf("telemetry service name is required")
	}

	if config.TraceSampleRatio < 0 || config.TraceSampleRatio > 1 {
		return fmt.Errorf(
			"trace sample ratio must be between 0 and 1",
		)
	}

	if config.MetricInterval <= 0 {
		return fmt.Errorf("metric interval must be greater than zero")
	}

	if config.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be greater than zero")
	}

	return nil
}

func parseBoolEnvironment(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}

	return parsed, nil
}

func parseResourceAttributes(raw string) map[string]string {
	attributes := make(map[string]string)

	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		key, value, found := strings.Cut(item, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if key == "" {
			continue
		}

		if decoded, err := url.QueryUnescape(value); err == nil {
			value = decoded
		}

		attributes[key] = value
	}

	return attributes
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}

	return ""
}
