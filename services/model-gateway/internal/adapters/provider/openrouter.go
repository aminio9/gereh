// Package provider contains provider adapters for upstream LLM services.
package provider

import (
	"context"
	"time"

	modelgatewayv1 "github.com/aminio9/gereh/gen/go/gereh/model/gateway/v1"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
)

// OpenRouterAdapter handles OpenRouter API (OpenAI-compatible protocol).
type OpenRouterAdapter struct {
	openAI *OpenAIAdapter
}

// NewOpenRouterAdapter creates a new OpenRouter adapter targeting openrouter.ai/api/v1.
func NewOpenRouterAdapter(timeout time.Duration, baseURL string) *OpenRouterAdapter {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	return &OpenRouterAdapter{
		openAI: NewOpenAIAdapter(timeout, baseURL),
	}
}

// ProviderKey returns the provider identifier.
func (a *OpenRouterAdapter) ProviderKey() string {
	return "openrouter"
}

// Complete executes a non-streaming OpenRouter chat completion request.
func (a *OpenRouterAdapter) Complete(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
) (domain.InferenceResult, error) {
	return a.openAI.Complete(ctx, route, apiKey, req)
}

// Stream executes a streaming OpenRouter chat completion request.
func (a *OpenRouterAdapter) Stream(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
	onChunk func(domain.StreamChunk) error,
) (domain.TokenUsage, error) {
	return a.openAI.Stream(ctx, route, apiKey, req, onChunk)
}
