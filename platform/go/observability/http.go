package observability

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPHandler instruments an inbound HTTP handler.
func (telemetry *Telemetry) HTTPHandler(
	operation string,
	next http.Handler,
) http.Handler {
	return otelhttp.NewHandler(
		next,
		operation,
		otelhttp.WithTracerProvider(
			telemetry.TracerProvider(),
		),
		otelhttp.WithMeterProvider(
			telemetry.MeterProvider(),
		),
		otelhttp.WithPropagators(
			telemetry.Propagator(),
		),
		otelhttp.WithFilter(func(request *http.Request) bool {
			switch request.URL.Path {
			case "/health/live", "/health/ready":
				return false
			default:
				return true
			}
		}),
	)
}

// HTTPTransport instruments an outbound HTTP transport.
func (telemetry *Telemetry) HTTPTransport(
	base http.RoundTripper,
) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return otelhttp.NewTransport(
		base,
		otelhttp.WithTracerProvider(
			telemetry.TracerProvider(),
		),
		otelhttp.WithMeterProvider(
			telemetry.MeterProvider(),
		),
		otelhttp.WithPropagators(
			telemetry.Propagator(),
		),
	)
}
