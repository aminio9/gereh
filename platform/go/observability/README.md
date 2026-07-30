# Gereh observability

This package initializes shared OpenTelemetry traces and metrics and enriches
standard-library slog records with active trace identifiers.

## Design

- OTLP over gRPC is the exporter protocol.
- W3C Trace Context and Baggage are propagated.
- Traces and metrics use stable OpenTelemetry SDKs.
- slog remains the application logging API.
- Telemetry is disabled when no OTLP endpoint is configured.
- Liveness and readiness requests are not traced.

## Environment

| Variable                              | Purpose                            |
| ------------------------------------- | ---------------------------------- |
| `OTEL_SDK_DISABLED`                   | Disable OpenTelemetry              |
| `OTEL_SERVICE_NAME`                   | Override service name              |
| `OTEL_EXPORTER_OTLP_ENDPOINT`         | Shared OTLP endpoint               |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`  | Trace-specific endpoint            |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | Metric-specific endpoint           |
| `OTEL_EXPORTER_OTLP_INSECURE`         | Allow plaintext OTLP locally       |
| `OTEL_TRACES_SAMPLER_ARG`             | Parent-based trace-ID sample ratio |
| `OTEL_METRIC_EXPORT_INTERVAL`         | Metric interval in milliseconds    |
| `OTEL_RESOURCE_ATTRIBUTES`            | Additional resource attributes     |
| `GEREH_ENVIRONMENT`                   | Deployment environment             |
| `GEREH_OTEL_SHUTDOWN_TIMEOUT`         | Telemetry shutdown timeout         |

## Local example

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
export OTEL_EXPORTER_OTLP_INSECURE=true
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=1.0
export GEREH_ENVIRONMENT=development
```

## Production rules

- Export through an OpenTelemetry Collector.
- Use encrypted OTLP transport.
- Use parent-based sampling.
- metric attributes.
- Keep metric attribute cardinality bounded.
- Do not place tenant IDs, user IDs, prompts, secrets, or model payloads in
