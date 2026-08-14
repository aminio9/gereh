package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/ports"
)

type CatalogWorkerConfig struct {
	BatchSize    int
	PollInterval time.Duration
	Lease        time.Duration
	MaxBackoff   time.Duration
}

type CatalogWorker struct {
	config  CatalogWorkerConfig
	repo    ports.CatalogRepository
	service *Service
	logger  *slog.Logger
}

func NewCatalogWorker(
	config CatalogWorkerConfig,
	repo ports.CatalogRepository,
	service *Service,
	logger *slog.Logger,
) *CatalogWorker {
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 500 * time.Millisecond
	}
	if config.Lease <= 0 {
		config.Lease = 60 * time.Second
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = 5 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &CatalogWorker{
		config:  config,
		repo:    repo,
		service: service,
		logger:  logger,
	}
}

func (w *CatalogWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *CatalogWorker) processBatch(ctx context.Context) {
	jobs, err := w.repo.ClaimCatalogRefresh(
		ctx,
		w.config.BatchSize,
		w.config.Lease,
	)
	if err != nil {
		w.logger.Error("claim catalog refresh jobs", "error", err)
		return
	}

	for _, job := range jobs {
		if err := w.service.ExecuteCatalogRefresh(ctx, job); err != nil {
			w.logger.Error("execute catalog refresh job",
				"refresh_id", job.RefreshID,
				"connection_id", job.ConnectionID,
				"error", err,
			)

			backoff := time.Duration(1<<job.Attempts) * time.Second
			if backoff > w.config.MaxBackoff {
				backoff = w.config.MaxBackoff
			}

			_ = w.repo.ReleaseCatalogRefresh(
				ctx,
				job.RefreshID,
				time.Now().UTC().Add(backoff),
				err.Error(),
			)
		}
	}
}
