package kafka

import (
	"fmt"
	"log/slog"

	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
)

func baseClientOptions(
	config Config,
	telemetry *observability.Telemetry,
	logger *slog.Logger,
) ([]kgo.Opt, *kotel.Tracer, error) {
	if err := config.ValidateCommon(); err != nil {
		return nil, nil, err
	}

	tracerOptions := []kotel.TracerOpt{
		kotel.ClientID(config.ClientID),
		kotel.TracerProvider(
			telemetry.TracerProvider(),
		),
		kotel.TracerPropagator(
			telemetry.Propagator(),
		),
	}

	if config.GroupID != "" {
		tracerOptions = append(
			tracerOptions,
			kotel.ConsumerGroup(config.GroupID),
		)
	}

	tracer := kotel.NewTracer(tracerOptions...)

	meter := kotel.NewMeter(
		kotel.MeterProvider(
			telemetry.MeterProvider(),
		),
	)

	instrumentation := kotel.NewKotel(
		kotel.WithTracer(tracer),
		kotel.WithMeter(meter),
	)

	options := []kgo.Opt{
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(config.ClientID),
		kgo.DialTimeout(config.DialTimeout),
		kgo.RequestTimeoutOverhead(
			config.RequestTimeoutOverhead,
		),
		kgo.WithHooks(instrumentation.Hooks()...),
		kgo.WithLogger(
			newSlogKafkaLogger(
				logger,
				kgo.LogLevelInfo,
			),
		),
	}

	tlsConfig, err := buildTLSConfig(config.TLS)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"build Kafka TLS configuration: %w",
			err,
		)
	}

	if tlsConfig != nil {
		options = append(
			options,
			kgo.DialTLSConfig(tlsConfig),
		)
	}

	saslMechanism, err := buildSASLMechanism(config.SASL)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"build Kafka SASL mechanism: %w",
			err,
		)
	}

	if saslMechanism != nil {
		options = append(
			options,
			kgo.SASL(saslMechanism),
		)
	}

	return options, tracer, nil
}
