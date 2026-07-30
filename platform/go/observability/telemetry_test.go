package observability

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSetupExportsSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	reader := sdkmetric.NewManualReader()

	config := DefaultConfig("test-service", "1.0.0")
	config.Enabled = true
	config.TracesEnabled = true
	config.MetricsEnabled = true

	telemetry, err := Setup(
		context.Background(),
		config,
		WithSpanExporter(exporter),
		WithMetricReader(reader),
	)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	ctx, span := telemetry.Tracer("test").Start(
		context.Background(),
		"operation",
	)
	_ = ctx
	span.End()

	if err := telemetry.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() error = %v", err)
	}

	spans := exporter.GetSpans()

	if len(spans) != 1 {
		t.Fatalf("exported spans = %d, want 1", len(spans))
	}

	if spans[0].Name != "operation" {
		t.Fatalf(
			"span name = %q, want operation",
			spans[0].Name,
		)
	}

	if err := telemetry.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestSetupDisabledUsesNoopProviders(t *testing.T) {
	config := DefaultConfig("test-service", "dev")

	telemetry, err := Setup(context.Background(), config)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if telemetry.TracerProvider() == nil {
		t.Fatal("TracerProvider() = nil")
	}

	if telemetry.MeterProvider() == nil {
		t.Fatal("MeterProvider() = nil")
	}
}
