// Package main runs the model-gateway service.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	modelgatewayv1 "github.com/aminio9/gereh/gen/go/gereh/model/gateway/v1"
	platformkafka "github.com/aminio9/gereh/platform/go/events/kafka"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/aminio9/gereh/platform/go/observability"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/model-gateway/internal/adapters/outbox"
	modelpostgres "github.com/aminio9/gereh/services/model-gateway/internal/adapters/postgres"
	"github.com/aminio9/gereh/services/model-gateway/internal/adapters/provider"
	modelsecrets "github.com/aminio9/gereh/services/model-gateway/internal/adapters/secrets"
	"github.com/aminio9/gereh/services/model-gateway/internal/application"
	"github.com/aminio9/gereh/services/model-gateway/internal/auth"
	"github.com/aminio9/gereh/services/model-gateway/internal/config"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
	"github.com/aminio9/gereh/services/model-gateway/internal/ports"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("model-gateway service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv(version)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	telemetryConfig, err := observability.ConfigFromEnv(cfg.ServiceName, cfg.ServiceVersion)
	if err != nil {
		return err
	}

	telemetry, err := observability.Setup(ctx, telemetryConfig)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), telemetryConfig.ShutdownTimeout)
		defer cancel()
		_ = telemetry.Shutdown(shutdownCtx)
	}()

	logger := slog.New(
		observability.NewTraceHandler(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{Level: slog.LevelInfo},
			),
		),
	)
	slog.SetDefault(logger)

	database, err := platformpostgres.Open(
		ctx,
		platformpostgres.Config{
			URL:                      cfg.DatabaseURL,
			ApplicationName:          cfg.ServiceName,
			MaxConnections:           cfg.PostgresMaxConnections,
			MinConnections:           cfg.PostgresMinConnections,
			MaxConnectionLifetime:    30 * time.Minute,
			MaxConnectionIdleTime:    5 * time.Minute,
			HealthCheckPeriod:        30 * time.Second,
			StatementTimeout:         15 * time.Second,
			LockTimeout:              3 * time.Second,
			IdleInTransactionTimeout: 15 * time.Second,
			OwnedTableSchemas:        []string{"public"},
		},
	)
	if err != nil {
		return err
	}
	defer database.Close()

	producer, err := platformkafka.NewProducer(cfg.Kafka, telemetry, logger)
	if err != nil {
		return err
	}
	defer producer.Close()

	if err := producer.Ping(ctx); err != nil {
		return err
	}

	// 1. Setup Resolver Client
	resolverConfig := grpcx.DefaultClientConfig(cfg.ResolverGRPCTarget)
	resolverConfig.Insecure = cfg.ResolverGRPCInsecure
	resolverConn, err := grpcx.NewClient(resolverConfig, telemetry)
	if err != nil {
		return fmt.Errorf("create resolver client: %w", err)
	}
	defer func() { _ = resolverConn.Close() }()

	resolverClient := modelgatewayv1.NewModelGatewayResolverServiceClient(resolverConn)

	// 2. Setup SecretStore (Vault)
	vaultStore, err := modelsecrets.NewVaultStore(modelsecrets.VaultConfig{
		Address:           cfg.VaultAddress,
		Mount:             cfg.VaultMount,
		Namespace:         cfg.VaultNamespace,
		TokenFile:         cfg.VaultTokenFile,
		StaticToken:       cfg.VaultStaticToken,
		CAFile:            cfg.VaultCAFile,
		AllowInsecureHTTP: cfg.VaultAllowInsecureHTTP,
		Timeout:           cfg.VaultTimeout,
	})
	if err != nil {
		return fmt.Errorf("configure vault store: %w", err)
	}

	// 3. Setup JWT verifier
	var tokenVerifier *auth.Verifier
	if cfg.RuntimePublicKeyFile != "" {
		tokenVerifier, err = auth.NewVerifierFromFile(cfg.RuntimePublicKeyFile, cfg.RuntimeTokenIssuer)
		if err != nil {
			return fmt.Errorf("load runtime public key: %w", err)
		}
	} else {
		// Development fallback: generate transient Ed25519 key
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		tokenVerifier = auth.NewVerifier(pub, cfg.RuntimeTokenIssuer)
	}

	// 4. Setup Providers
	adapters := []ports.ProviderAdapter{
		provider.NewOpenAIAdapter(cfg.ProviderTimeout, ""),
		provider.NewAnthropicAdapter(cfg.ProviderTimeout, ""),
		provider.NewGeminiAdapter(cfg.ProviderTimeout, ""),
		provider.NewOpenRouterAdapter(cfg.ProviderTimeout, ""),
	}

	repository := modelpostgres.New(database)

	appService := application.New(
		application.ServiceConfig{
			EventTopic:               cfg.EventTopic,
			RequireBudgetReservation: cfg.RequireBudgetReservation,
		},
		&resolverAdapter{client: resolverClient},
		&secretAdapter{store: vaultStore},
		ports.DeferredBudgetVerifier{},
		repository,
		adapters,
		logger,
	)

	// Outbox Relay Worker
	relay, err := outbox.New(
		outbox.Config{
			BatchSize:    cfg.OutboxBatchSize,
			PollInterval: cfg.OutboxPollInterval,
			Lease:        cfg.OutboxLease,
			MaxBackoff:   cfg.OutboxMaxBackoff,
		},
		repository,
		producer,
		logger,
	)
	if err != nil {
		return err
	}
	go relay.Run(ctx)

	// HTTP Server
	router := chi.NewRouter()

	// Probes
	router.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("live"))
	})
	router.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := database.Pool().Ping(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// Inference endpoint: POST /v1/chat/completions
	router.Post("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		claims, err := tokenVerifier.Verify(authHeader)
		if err != nil {
			http.Error(w, `{"error":{"message":"Unauthorized","type":"invalid_request_error"}}`, http.StatusUnauthorized)
			return
		}

		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		correlationID := strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
		if correlationID == "" {
			correlationID = requestID
		}

		var reqPayload domain.InferenceRequest
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
		if err != nil {
			http.Error(w, `{"error":{"message":"Invalid request body","type":"invalid_request_error"}}`, http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(bodyBytes, &reqPayload); err != nil {
			http.Error(w, `{"error":{"message":"Invalid JSON payload","type":"invalid_request_error"}}`, http.StatusBadRequest)
			return
		}

		reqPayload.RequestID = requestID
		reqPayload.CorrelationID = correlationID

		if reqPayload.Stream {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")

			err := appService.ExecuteStream(r.Context(), claims, reqPayload, func(chunk domain.StreamChunk) error {
				chunkBytes, err := json.Marshal(chunk)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", string(chunkBytes)); err != nil {
					return err
				}
				flusher.Flush()
				return nil
			})

			if err != nil && !errors.Is(err, domain.ErrClientDisconnected) {
				chunkBytes, _ := json.Marshal(map[string]any{
					"error": map[string]string{
						"message": err.Error(),
						"type":    "server_error",
					},
				})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", string(chunkBytes))
				flusher.Flush()
			}

			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		result, err := appService.Execute(r.Context(), claims, reqPayload)
		if err != nil {
			if errors.Is(err, domain.ErrDuplicateRequestID) {
				http.Error(w, `{"error":{"message":"Duplicate request ID","type":"invalid_request_error"}}`, http.StatusConflict)
				return
			}
			if errors.Is(err, domain.ErrBudgetExceeded) {
				http.Error(w, `{"error":{"message":"Budget limit exceeded","type":"insufficient_quota"}}`, http.StatusPaymentRequired)
				return
			}
			http.Error(w, fmt.Sprintf(`{"error":{"message":%q,"type":"api_error"}}`, err.Error()), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddress,
		Handler: router,
		// No generic WriteTimeout so long SSE streams are not killed after 30s
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(ctx, "tcp", cfg.HTTPAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddress, err)
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("model-gateway service listening", "address", cfg.HTTPAddress)
		serverErrors <- server.Serve(listener)
	}()

	select {
	case serverErr := <-serverErrors:
		return serverErr
	case <-ctx.Done():
		logger.Info("model-gateway service shutdown requested")
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownContext)
}

type resolverAdapter struct {
	client modelgatewayv1.ModelGatewayResolverServiceClient
}

func (r *resolverAdapter) ResolveInferencePlan(
	ctx context.Context,
	tenantID string,
	agentID string,
) (*modelgatewayv1.InferencePlan, error) {
	resp, err := r.client.ResolveInferencePlan(ctx, &modelgatewayv1.ResolveInferencePlanRequest{
		TenantId: tenantID,
		AgentId:  agentID,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetPlan(), nil
}

type secretAdapter struct {
	store *modelsecrets.VaultStore
}

func (s *secretAdapter) GetBYOKSecret(
	ctx context.Context,
	tenantID string,
	connectionID string,
) ([]byte, error) {
	return s.store.GetBYOKSecret(ctx, tenantID, connectionID)
}
