package secrets

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
)

// CleanupConfig configures the secret cleanup worker.
type CleanupConfig struct {
	BatchSize int

	PollInterval time.Duration
	Lease        time.Duration
	MaxBackoff   time.Duration
}

// CleanupWorker drains the durable secret-cleanup queue.
type CleanupWorker struct {
	config CleanupConfig

	repository ports.Repository
	store      ports.SecretStore

	logger *slog.Logger
}

// NewCleanupWorker constructs a secret cleanup worker.
func NewCleanupWorker(
	config CleanupConfig,
	repository ports.Repository,
	store ports.SecretStore,
	logger *slog.Logger,
) (*CleanupWorker, error) {
	if config.BatchSize <= 0 ||
		config.PollInterval <= 0 ||
		config.Lease <= 0 ||
		config.MaxBackoff <= 0 {
		return nil,
			fmt.Errorf(
				"invalid secret cleanup configuration",
			)
	}

	if repository == nil {
		return nil, fmt.Errorf(
			"secret cleanup repository is required",
		)
	}

	if store == nil {
		return nil, fmt.Errorf(
			"secret cleanup secret store is required",
		)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &CleanupWorker{
		config:     config,
		repository: repository,
		store:      store,
		logger:     logger,
	}, nil
}

// Run blocks until the context is cancelled, draining the queue on a ticker.
func (worker *CleanupWorker) Run(
	ctx context.Context,
) {
	ticker :=
		time.NewTicker(
			worker.config.PollInterval,
		)
	defer ticker.Stop()

	for {
		worker.process(ctx)

		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
		}
	}
}

func (worker *CleanupWorker) process(
	ctx context.Context,
) {
	items, err :=
		worker.repository.
			ClaimSecretCleanup(
				ctx,
				worker.config.BatchSize,
				worker.config.Lease,
			)
	if err != nil {
		worker.logger.ErrorContext(
			ctx,
			"secret cleanup claim failed",
			"error",
			err,
		)

		return
	}

	for _, item := range items {
		err :=
			worker.cleanupOne(
				ctx,
				item,
			)

		if err == nil {
			_ =
				worker.repository.
					CompleteSecretCleanup(
						ctx,
						item.ID,
					)

			continue
		}

		exponent :=
			min(
				item.Attempts,
				8,
			)

		backoff :=
			time.Duration(
				math.Pow(
					2,
					float64(exponent),
				),
			) * time.Second

		if backoff >
			worker.config.MaxBackoff {
			backoff =
				worker.config.MaxBackoff
		}

		// Do not log the secret reference. Although it contains no API
		// key, it is unnecessary tenant/security topology.
		_ =
			worker.repository.
				ReleaseSecretCleanup(
					ctx,
					item.ID,
					time.Now().
						UTC().
						Add(backoff),
					err.Error(),
				)
	}
}

func (
	worker *CleanupWorker,
) cleanupOne(
	ctx context.Context,
	item domain.SecretCleanup,
) error {
	switch item.Action {
	case "destroy_version":
		if item.Version == nil {
			return fmt.Errorf(
				"secret cleanup version missing",
			)
		}

		return worker.store.
			DestroyVersion(
				ctx,
				item.SecretRef,
				*item.Version,
			)

	case "purge_secret":
		return worker.store.
			Purge(
				ctx,
				item.SecretRef,
			)

	default:
		return fmt.Errorf(
			"unknown secret cleanup action",
		)
	}
}
