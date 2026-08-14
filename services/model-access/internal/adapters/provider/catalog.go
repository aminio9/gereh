package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

// CatalogClient performs model discovery against provider APIs.
type CatalogClient struct {
	httpClient *http.Client
	timeout    time.Duration
}

// NewCatalogClient constructs a new provider CatalogClient with the given timeout.
func NewCatalogClient(timeout time.Duration) *CatalogClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return &CatalogClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

const maxCatalogResponseBytes = 2 * 1024 * 1024 // 2MB bound

// DiscoverModels fetches model offerings from the specified provider API.
func (c *CatalogClient) DiscoverModels(
	ctx context.Context,
	providerKey string,
	apiKey []byte,
) ([]domain.DiscoveredModel, error) {
	switch providerKey {
	case "openai":
		return c.discoverOpenAI(ctx, apiKey)
	case "anthropic":
		return c.discoverAnthropic(ctx, apiKey)
	case "google":
		return c.discoverGoogle(ctx, apiKey)
	case "openrouter":
		return c.discoverOpenRouter(ctx, apiKey)
	default:
		return nil, domain.ErrProviderNotFound
	}
}

// OpenAI /v1/models
type openAIModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

func (c *CatalogClient) discoverOpenAI(
	ctx context.Context,
	apiKey []byte,
) ([]domain.DiscoveredModel, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.openai.com/v1/models",
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+string(apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai catalog request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, domain.ErrCredentialRejected
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrCredentialVerificationUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai catalog status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogResponseBytes))
	if err != nil {
		return nil, err
	}

	var parsed openAIModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse openai catalog: %w", err)
	}

	results := make([]domain.DiscoveredModel, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		agentUsable := isOpenAIAgentUsable(m.ID)
		var createdAt *time.Time
		if m.Created > 0 {
			t := time.Unix(m.Created, 0).UTC()
			createdAt = &t
		}

		results = append(results, domain.DiscoveredModel{
			ProviderKey:       "openai",
			ProviderModelID:   m.ID,
			DisplayName:       formatDisplayName(m.ID),
			Description:       fmt.Sprintf("OpenAI model %s (owned by %s)", m.ID, m.OwnedBy),
			AgentUsable:       agentUsable,
			ProviderCreatedAt: createdAt,
		})
	}

	return results, nil
}

func isOpenAIAgentUsable(modelID string) bool {
	lower := strings.ToLower(modelID)
	if strings.Contains(lower, "embedding") ||
		strings.Contains(lower, "dall-e") ||
		strings.Contains(lower, "tts") ||
		strings.Contains(lower, "whisper") ||
		strings.Contains(lower, "moderation") ||
		strings.Contains(lower, "realtime") ||
		strings.Contains(lower, "audio") {
		return false
	}
	if strings.HasPrefix(lower, "gpt-4") ||
		strings.HasPrefix(lower, "gpt-3.5") ||
		strings.HasPrefix(lower, "o1") ||
		strings.HasPrefix(lower, "o3") ||
		strings.HasPrefix(lower, "chatgpt") {
		return true
	}
	return false
}

// Anthropic /v1/models
type anthropicModelsResponse struct {
	Data []struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		DisplayName string `json:"display_name"`
		CreatedAt   string `json:"created_at"`
	} `json:"data"`
	HasMore bool `json:"has_more"`
}

func (c *CatalogClient) discoverAnthropic(
	ctx context.Context,
	apiKey []byte,
) ([]domain.DiscoveredModel, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.anthropic.com/v1/models?limit=100",
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", string(apiKey))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic catalog request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, domain.ErrCredentialRejected
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrCredentialVerificationUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic catalog status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogResponseBytes))
	if err != nil {
		return nil, err
	}

	var parsed anthropicModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse anthropic catalog: %w", err)
	}

	results := make([]domain.DiscoveredModel, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		displayName := m.DisplayName
		if displayName == "" {
			displayName = formatDisplayName(m.ID)
		}
		var createdAt *time.Time
		if m.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
				utc := t.UTC()
				createdAt = &utc
			}
		}

		results = append(results, domain.DiscoveredModel{
			ProviderKey:       "anthropic",
			ProviderModelID:   m.ID,
			DisplayName:       displayName,
			Description:       fmt.Sprintf("Anthropic model %s", m.ID),
			AgentUsable:       true, // Anthropic /v1/models only lists chat/agent models
			ProviderCreatedAt: createdAt,
		})
	}

	return results, nil
}

// Google /v1beta/models
type googleModelsResponse struct {
	Models []struct {
		Name                       string   `json:"name"`
		Version                    string   `json:"version"`
		DisplayName                string   `json:"displayName"`
		Description                string   `json:"description"`
		InputTokenLimit            int64    `json:"inputTokenLimit"`
		OutputTokenLimit           int64    `json:"outputTokenLimit"`
		SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	} `json:"models"`
}

func (c *CatalogClient) discoverGoogle(
	ctx context.Context,
	apiKey []byte,
) ([]domain.DiscoveredModel, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", string(apiKey))
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google catalog request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest {
		return nil, domain.ErrCredentialRejected
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrCredentialVerificationUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google catalog status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogResponseBytes))
	if err != nil {
		return nil, err
	}

	var parsed googleModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse google catalog: %w", err)
	}

	results := make([]domain.DiscoveredModel, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		modelID := strings.TrimPrefix(m.Name, "models/")
		agentUsable := isGoogleAgentUsable(m.SupportedGenerationMethods)

		results = append(results, domain.DiscoveredModel{
			ProviderKey:         "google",
			ProviderModelID:     modelID,
			DisplayName:         m.DisplayName,
			Description:         m.Description,
			AgentUsable:         agentUsable,
			ContextWindowTokens: m.InputTokenLimit,
			MaxOutputTokens:     m.OutputTokenLimit,
		})
	}

	return results, nil
}

func isGoogleAgentUsable(methods []string) bool {
	for _, method := range methods {
		if method == "generateContent" {
			return true
		}
	}
	return false
}

// OpenRouter /api/v1/models
type openRouterModelsResponse struct {
	Data []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		ContextLength int64  `json:"context_length"`
		Architecture  struct {
			Modality     string  `json:"modality"`
			Tokenizer    string  `json:"tokenizer"`
			InstructType *string `json:"instruct_type"`
		} `json:"architecture"`
		TopProvider struct {
			MaxCompletionTokens *int64 `json:"max_completion_tokens"`
		} `json:"top_provider"`
	} `json:"data"`
}

func (c *CatalogClient) discoverOpenRouter(
	ctx context.Context,
	apiKey []byte,
) ([]domain.DiscoveredModel, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://openrouter.ai/api/v1/models",
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+string(apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter catalog request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, domain.ErrCredentialRejected
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrCredentialVerificationUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter catalog status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogResponseBytes))
	if err != nil {
		return nil, err
	}

	var parsed openRouterModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse openrouter catalog: %w", err)
	}

	results := make([]domain.DiscoveredModel, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		var maxOutput int64
		if m.TopProvider.MaxCompletionTokens != nil {
			maxOutput = *m.TopProvider.MaxCompletionTokens
		}

		results = append(results, domain.DiscoveredModel{
			ProviderKey:         "openrouter",
			ProviderModelID:     m.ID,
			DisplayName:         m.Name,
			Description:         m.Description,
			AgentUsable:         true,
			ContextWindowTokens: m.ContextLength,
			MaxOutputTokens:     maxOutput,
		})
	}

	return results, nil
}

func formatDisplayName(id string) string {
	parts := strings.Split(id, "/")
	name := parts[len(parts)-1]
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}
