package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceHandler enriches slog records with the active OpenTelemetry span.
type TraceHandler struct {
	next slog.Handler
}

// NewTraceHandler wraps a slog handler with trace-context enrichment.
func NewTraceHandler(next slog.Handler) slog.Handler {
	return &TraceHandler{
		next: next,
	}
}

// Enabled reports whether the wrapped handler accepts the log event.
func (handler *TraceHandler) Enabled(
	ctx context.Context,
	level slog.Level,
) bool {
	return handler.next.Enabled(ctx, level)
}

// Handle adds trace identifiers and delegates to the wrapped handler.
func (handler *TraceHandler) Handle(
	ctx context.Context,
	record slog.Record,
) error {
	spanContext := trace.SpanContextFromContext(ctx)

	if spanContext.IsValid() {
		record.AddAttrs(
			slog.String(
				"trace_id",
				spanContext.TraceID().String(),
			),
			slog.String(
				"span_id",
				spanContext.SpanID().String(),
			),
			slog.Bool(
				"trace_sampled",
				spanContext.IsSampled(),
			),
		)
	}

	return handler.next.Handle(ctx, record)
}

// WithAttrs returns a handler with additional attributes.
func (handler *TraceHandler) WithAttrs(
	attributes []slog.Attr,
) slog.Handler {
	return &TraceHandler{
		next: handler.next.WithAttrs(attributes),
	}
}

// WithGroup returns a handler with an additional group.
func (handler *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{
		next: handler.next.WithGroup(name),
	}
}
