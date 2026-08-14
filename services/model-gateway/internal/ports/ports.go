package ports

import (
	"context"

	modelgatewayv1 "github.com/aminio9/gereh/gen/go/gereh/model/gateway/v1"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
)

// ResolverClient queries Model Access to resolve the inference plan for an agent.
type ResolverClient interface {
	ResolveInferencePlan(
		ctx context.Context,
		tenantID string,
		agentID string,
	) (*modelgatewayv1.InferencePlan, error)
}

// SecretStore retrieves decrypted BYOK credentials from Vault for inference.
type SecretStore interface {
	GetBYOKSecret(
		ctx context.Context,
		tenantID string,
		connectionID string,
	) ([]byte, error)
}

// BudgetVerifier checks and records budget reservations for an inference request.
type BudgetVerifier interface {
	CheckBudget(
		ctx context.Context,
		tenantID string,
		agentID string,
		maxCostMicroUSD *int64,
	) error
}

// DeferredBudgetVerifier is a no-op implementation of BudgetVerifier until Phase 30.
type DeferredBudgetVerifier struct{}

// CheckBudget is a no-op implementation of the BudgetVerifier interface until Phase 30.
func (DeferredBudgetVerifier) CheckBudget(
	_ context.Context,
	_ string,
	_ string,
	_ *int64,
) error {
	return nil
}

// ProviderAdapter handles communication with a specific upstream provider (OpenAI, Anthropic, Gemini, OpenRouter).
type ProviderAdapter interface {
	ProviderKey() string
	Complete(
		ctx context.Context,
		route *modelgatewayv1.InferenceRoute,
		apiKey []byte,
		req domain.InferenceRequest,
	) (domain.InferenceResult, error)

	Stream(
		ctx context.Context,
		route *modelgatewayv1.InferenceRoute,
		apiKey []byte,
		req domain.InferenceRequest,
		onChunk func(domain.StreamChunk) error,
	) (domain.TokenUsage, error)
}

// JournalRepository persists admitted requests and records completions in PostgreSQL.
type JournalRepository interface {
	AdmitRequest(
		ctx context.Context,
		record domain.JournalRecord,
	) error

	CompleteRequest(
		ctx context.Context,
		record domain.JournalRecord,
		outboxEvent *domain.OutboxEvent,
	) error
}
