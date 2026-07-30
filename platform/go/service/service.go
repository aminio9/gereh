// Package service provides shared HTTP server lifecycle infrastructure
// for Gereh services.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminio9/gereh/platform/go/observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Config defines the runtime configuration for a Gereh service.
type Config struct {
	Name           string
	Version        string
	DefaultAddress string
}

// RouteRegistrar registers service-specific HTTP routes.
type RouteRegistrar func(router chi.Router)

// Run starts a Gereh HTTP service and blocks until it stops.
//
// Run terminates the process with a non-zero exit status when the server
// cannot start or cannot shut down cleanly.
func Run(config Config, register RouteRegistrar) {
	if err := run(config, register); err != nil {
		slog.Error(
			"service stopped with an error",
			"service",
			config.Name,
			"error",
			err,
		)

		os.Exit(1)
	}
}

func run(
	config Config,
	register RouteRegistrar,
) (runErr error) {
	if config.Name == "" {
		return errors.New("service name is required")
	}

	if config.Version == "" {
		config.Version = "dev"
	}

	if config.DefaultAddress == "" {
		config.DefaultAddress = ":8080"
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	telemetryConfig, err := observability.ConfigFromEnv(
		config.Name,
		config.Version,
	)
	if err != nil {
		return fmt.Errorf(
			"configure observability: %w",
			err,
		)
	}

	telemetry, err := observability.Setup(
		ctx,
		telemetryConfig,
	)
	if err != nil {
		return fmt.Errorf(
			"initialize observability: %w",
			err,
		)
	}

	defer func() {
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			telemetryConfig.ShutdownTimeout,
		)
		defer cancel()

		runErr = errors.Join(
			runErr,
			telemetry.Shutdown(shutdownContext),
		)
	}()

	logger := slog.New(
		observability.NewTraceHandler(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelInfo,
				},
			),
		),
	)

	slog.SetDefault(logger)

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(30 * time.Second))

	router.Get(
		"/health/live",
		func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(
				writer,
				http.StatusOK,
				map[string]string{
					"status":  "ok",
					"service": config.Name,
				},
			)
		},
	)

	router.Get(
		"/health/ready",
		func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(
				writer,
				http.StatusOK,
				map[string]string{
					"status":  "ready",
					"service": config.Name,
				},
			)
		},
	)

	router.Get(
		"/version",
		func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(
				writer,
				http.StatusOK,
				map[string]string{
					"service": config.Name,
					"version": config.Version,
				},
			)
		},
	)

	if register != nil {
		register(router)
	}

	address := envOrDefault(
		"HTTP_ADDRESS",
		config.DefaultAddress,
	)

	handler := telemetry.HTTPHandler(
		config.Name,
		router,
	)

	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.InfoContext(
			ctx,
			"service listening",
			"service",
			config.Name,
			"version",
			config.Version,
			"address",
			address,
			"telemetry_enabled",
			telemetryConfig.Enabled,
		)

		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf(
				"serve HTTP: %w",
				err,
			)
		}

		return nil

	case <-ctx.Done():
		logger.InfoContext(
			ctx,
			"shutdown requested",
			"service",
			config.Name,
		)
	}

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf(
			"shut down HTTP server: %w",
			err,
		)
	}

	logger.Info(
		"service stopped",
		"service",
		config.Name,
	)

	return nil
}

func writeJSON(
	writer http.ResponseWriter,
	status int,
	value any,
) {
	writer.Header().Set(
		"Content-Type",
		"application/json",
	)

	writer.WriteHeader(status)

	if err := json.NewEncoder(writer).Encode(value); err != nil {
		slog.Error(
			"encode response",
			"error",
			err,
		)
	}
}

func envOrDefault(
	name string,
	fallback string,
) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
