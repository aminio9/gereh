package kafka

import (
	"context"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

type slogKafkaLogger struct {
	logger *slog.Logger
	level  kgo.LogLevel
}

func newSlogKafkaLogger(
	logger *slog.Logger,
	level kgo.LogLevel,
) kgo.Logger {
	if logger == nil {
		logger = slog.Default()
	}

	return &slogKafkaLogger{
		logger: logger,
		level:  level,
	}
}

func (logger *slogKafkaLogger) Level() kgo.LogLevel {
	return logger.level
}

func (logger *slogKafkaLogger) Log(
	level kgo.LogLevel,
	message string,
	values ...any,
) {
	slogLevel := slog.LevelInfo

	switch level {
	case kgo.LogLevelDebug:
		slogLevel = slog.LevelDebug
	case kgo.LogLevelInfo:
		slogLevel = slog.LevelInfo
	case kgo.LogLevelWarn:
		slogLevel = slog.LevelWarn
	case kgo.LogLevelError:
		slogLevel = slog.LevelError
	case kgo.LogLevelNone:
		return
	}

	logger.logger.Log(
		context.Background(),
		slogLevel,
		message,
		values...,
	)
}
