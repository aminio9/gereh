package observability

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Telemetry contains the providers and propagator shared by a process.
type Telemetry struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	propagator     propagation.TextMapPropagator

	shutdownFunctions []func(context.Context) error
	shutdownOnce      sync.Once
	shutdownError     error
}

// Option customizes telemetry initialization.
type Option func(*setupOptions)

type setupOptions struct {
	spanExporter sdktrace.SpanExporter
	metricReader sdkmetric.Reader
}

// WithSpanExporter overrides the OTLP trace exporter.
//
// This option is primarily intended for tests.
func WithSpanExporter(exporter sdktrace.SpanExporter) Option {
	return func(options *setupOptions) {
		options.spanExporter = exporter
	}
}

// WithMetricReader overrides the periodic OTLP metric reader.
//
// This option is primarily intended for tests.
func WithMetricReader(reader sdkmetric.Reader) Option {
	return func(options *setupOptions) {
		options.metricReader = reader
	}
}

// Setup initializes global and process-local OpenTelemetry providers.
func Setup(
	ctx context.Context,
	config Config,
	options ...Option,
) (*Telemetry, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate telemetry configuration: %w", err)
	}

	setup := setupOptions{}

	for _, option := range options {
		option(&setup)
	}

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	if !config.Enabled {
		telemetry := newNoopTelemetry(propagator)
		telemetry.installGlobals()

		return telemetry, nil
	}

	if !config.TracesEnabled && !config.MetricsEnabled {
		config.TracesEnabled = true
		config.MetricsEnabled = true
	}

	serviceResource, err := newResource(config)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	telemetry := &Telemetry{
		tracerProvider: tracenoop.NewTracerProvider(),
		meterProvider:  metricnoop.NewMeterProvider(),
		propagator:     propagator,
	}

	if config.TracesEnabled {
		spanExporter := setup.spanExporter

		if spanExporter == nil {
			spanExporter, err = otlptracegrpc.New(ctx)
			if err != nil {
				return nil, fmt.Errorf(
					"create OTLP trace exporter: %w",
					err,
				)
			}
		}

		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithResource(serviceResource),
			sdktrace.WithSampler(
				sdktrace.ParentBased(
					sdktrace.TraceIDRatioBased(
						config.TraceSampleRatio,
					),
				),
			),
			sdktrace.WithBatcher(spanExporter),
		)

		telemetry.tracerProvider = tracerProvider
		telemetry.shutdownFunctions = append(
			telemetry.shutdownFunctions,
			tracerProvider.Shutdown,
		)
	}

	if config.MetricsEnabled {
		metricReader := setup.metricReader

		if metricReader == nil {
			metricExporter, exporterErr := otlpmetricgrpc.New(ctx)
			if exporterErr != nil {
				return nil, fmt.Errorf(
					"create OTLP metric exporter: %w",
					exporterErr,
				)
			}

			metricReader = sdkmetric.NewPeriodicReader(
				metricExporter,
				sdkmetric.WithInterval(config.MetricInterval),
			)
		}

		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(serviceResource),
			sdkmetric.WithReader(metricReader),
		)

		telemetry.meterProvider = meterProvider
		telemetry.shutdownFunctions = append(
			telemetry.shutdownFunctions,
			meterProvider.Shutdown,
		)
	}

	telemetry.installGlobals()

	return telemetry, nil
}

// TracerProvider returns the process tracer provider.
func (telemetry *Telemetry) TracerProvider() trace.TracerProvider {
	if telemetry == nil {
		return otel.GetTracerProvider()
	}

	return telemetry.tracerProvider
}

// MeterProvider returns the process meter provider.
func (telemetry *Telemetry) MeterProvider() metric.MeterProvider {
	if telemetry == nil {
		return otel.GetMeterProvider()
	}

	return telemetry.meterProvider
}

// Propagator returns the process text-map propagator.
func (telemetry *Telemetry) Propagator() propagation.TextMapPropagator {
	if telemetry == nil {
		return otel.GetTextMapPropagator()
	}

	return telemetry.propagator
}

// Tracer returns a tracer from the configured provider.
func (telemetry *Telemetry) Tracer(name string) trace.Tracer {
	return telemetry.TracerProvider().Tracer(name)
}

// Meter returns a meter from the configured provider.
func (telemetry *Telemetry) Meter(name string) metric.Meter {
	return telemetry.MeterProvider().Meter(name)
}

// ForceFlush flushes any provider that supports explicit flushing.
func (telemetry *Telemetry) ForceFlush(ctx context.Context) error {
	if telemetry == nil {
		return nil
	}

	var flushErrors []error

	if provider, ok := telemetry.tracerProvider.(interface {
		ForceFlush(context.Context) error
	}); ok {
		if err := provider.ForceFlush(ctx); err != nil {
			flushErrors = append(flushErrors, err)
		}
	}

	if provider, ok := telemetry.meterProvider.(interface {
		ForceFlush(context.Context) error
	}); ok {
		if err := provider.ForceFlush(ctx); err != nil {
			flushErrors = append(flushErrors, err)
		}
	}

	return errors.Join(flushErrors...)
}

// Shutdown flushes and shuts down all telemetry providers.
//
// Shutdown is idempotent.
func (telemetry *Telemetry) Shutdown(ctx context.Context) error {
	if telemetry == nil {
		return nil
	}

	telemetry.shutdownOnce.Do(func() {
		var shutdownErrors []error

		for index := len(telemetry.shutdownFunctions) - 1; index >= 0; index-- {
			if err := telemetry.shutdownFunctions[index](ctx); err != nil {
				shutdownErrors = append(shutdownErrors, err)
			}
		}

		telemetry.shutdownError = errors.Join(shutdownErrors...)
	})

	return telemetry.shutdownError
}

func newNoopTelemetry(
	propagator propagation.TextMapPropagator,
) *Telemetry {
	return &Telemetry{
		tracerProvider: tracenoop.NewTracerProvider(),
		meterProvider:  metricnoop.NewMeterProvider(),
		propagator:     propagator,
	}
}

func (telemetry *Telemetry) installGlobals() {
	otel.SetTracerProvider(telemetry.tracerProvider)
	otel.SetMeterProvider(telemetry.meterProvider)
	otel.SetTextMapPropagator(telemetry.propagator)
}

func newResource(config Config) (*resource.Resource, error) {
	attributes := []attribute.KeyValue{
		attribute.String("service.name", config.ServiceName),
		attribute.String("service.namespace", "gereh"),
		attribute.String("service.version", config.ServiceVersion),
		attribute.String(
			"deployment.environment.name",
			config.Environment,
		),
	}

	keys := make([]string, 0, len(config.ResourceAttributes))

	for key := range config.ResourceAttributes {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		attributes = append(
			attributes,
			attribute.String(key, config.ResourceAttributes[key]),
		)
	}

	serviceResource := resource.NewWithAttributes(
		"",
		attributes...,
	)

	mergedResource, err := resource.Merge(
		resource.Default(),
		serviceResource,
	)
	if err != nil {
		return nil, err
	}

	return mergedResource, nil
}
