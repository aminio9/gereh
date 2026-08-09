package projection

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics instruments the Projection worker with the counters and
// histograms defined in the phase-14 observability plan.
type Metrics struct {
	eventsConsumed   metric.Int64Counter
	eventsApplied    metric.Int64Counter
	eventsDuplicate  metric.Int64Counter
	eventsUnknown    metric.Int64Counter
	eventsFailed     metric.Int64Counter
	eventsStale      metric.Int64Counter
	identityConflict metric.Int64Counter

	applyDuration metric.Float64Histogram
	eventLag      metric.Float64Histogram
}

// NewMetrics creates the projection worker metrics from a meter.
// A nil meter returns a no-op Metrics.
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	metrics := &Metrics{}

	if meter == nil {
		return metrics, nil
	}

	var err error

	if metrics.eventsConsumed, err = meter.Int64Counter(
		"gereh.projection.events.consumed.total",
		metric.WithDescription(
			"Number of Kafka events consumed by the projection worker",
		),
	); err != nil {
		return nil, err
	}

	if metrics.eventsApplied, err = meter.Int64Counter(
		"gereh.projection.events.applied.total",
		metric.WithDescription(
			"Number of events applied to the read model for the first time",
		),
	); err != nil {
		return nil, err
	}

	if metrics.eventsDuplicate, err = meter.Int64Counter(
		"gereh.projection.events.duplicate.total",
		metric.WithDescription(
			"Number of duplicate events detected by the inbox",
		),
	); err != nil {
		return nil, err
	}

	if metrics.eventsUnknown, err = meter.Int64Counter(
		"gereh.projection.events.unknown.total",
		metric.WithDescription(
			"Number of events with unknown semantics that were checkpointed and ignored",
		),
	); err != nil {
		return nil, err
	}

	if metrics.eventsFailed, err = meter.Int64Counter(
		"gereh.projection.events.failed.total",
		metric.WithDescription(
			"Number of events that failed to project",
		),
	); err != nil {
		return nil, err
	}

	if metrics.eventsStale, err = meter.Int64Counter(
		"gereh.projection.events.stale.total",
		metric.WithDescription(
			"Number of stale events skipped by aggregate-version checks",
		),
	); err != nil {
		return nil, err
	}

	if metrics.identityConflict, err = meter.Int64Counter(
		"gereh.projection.events.identity.conflict.total",
		metric.WithDescription(
			"Number of event-ID identity conflicts detected",
		),
	); err != nil {
		return nil, err
	}

	if metrics.applyDuration, err = meter.Float64Histogram(
		"gereh.projection.event.apply.duration.seconds",
		metric.WithDescription(
			"Time spent applying one event transaction",
		),
		metric.WithUnit("s"),
	); err != nil {
		return nil, err
	}

	if metrics.eventLag, err = meter.Float64Histogram(
		"gereh.projection.event.lag.seconds",
		metric.WithDescription(
			"Age of consumed events at projection time",
		),
		metric.WithUnit("s"),
	); err != nil {
		return nil, err
	}

	return metrics, nil
}

// Consumed records an event received from Kafka.
func (metrics *Metrics) Consumed(eventType string) {
	if metrics == nil || metrics.eventsConsumed == nil {
		return
	}

	metrics.eventsConsumed.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}

// Applied records an event applied for the first time.
func (metrics *Metrics) Applied(eventType string) {
	if metrics == nil || metrics.eventsApplied == nil {
		return
	}

	metrics.eventsApplied.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}

// Duplicate records an inbox-detected duplicate delivery.
func (metrics *Metrics) Duplicate(eventType string) {
	if metrics == nil || metrics.eventsDuplicate == nil {
		return
	}

	metrics.eventsDuplicate.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}

// Unknown records an event with unknown semantics.
func (metrics *Metrics) Unknown(eventType string) {
	if metrics == nil || metrics.eventsUnknown == nil {
		return
	}

	metrics.eventsUnknown.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}

// Failed records an event that failed to apply.
func (metrics *Metrics) Failed(eventType string) {
	if metrics == nil || metrics.eventsFailed == nil {
		return
	}

	metrics.eventsFailed.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}

// Stale records a stale event skipped by its aggregate version.
func (metrics *Metrics) Stale(eventType string) {
	if metrics == nil || metrics.eventsStale == nil {
		return
	}

	metrics.eventsStale.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}

// IdentityConflict records an event-ID reuse with different content.
func (metrics *Metrics) IdentityConflict() {
	if metrics == nil || metrics.identityConflict == nil {
		return
	}

	metrics.identityConflict.Add(
		context.Background(),
		1,
	)
}

// ObserveApplyDuration records the apply transaction duration.
func (metrics *Metrics) ObserveApplyDuration(
	seconds float64,
	eventType string,
) {
	if metrics == nil || metrics.applyDuration == nil {
		return
	}

	metrics.applyDuration.Record(
		context.Background(),
		seconds,
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}

// ObserveLag records the event age at projection time.
func (metrics *Metrics) ObserveLag(
	lag time.Duration,
	eventType string,
) {
	if metrics == nil || metrics.eventLag == nil {
		return
	}

	metrics.eventLag.Record(
		context.Background(),
		lag.Seconds(),
		metric.WithAttributes(
			attribute.String("event_type", eventType),
		),
	)
}
